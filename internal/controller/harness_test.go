package controller

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

// TestFiveNodeAutomaticRecoveryHarness is the process-free end-to-end fixture:
// five real data Managers behind the production HTTP API and RESP transport,
// plus a five-voter in-memory Raft controller group.
func TestFiveNodeAutomaticRecoveryHarness(t *testing.T) {
	slotCount := uint32(1024)
	if harnessRaceEnabled {
		// Race instrumentation makes every committed per-slot progress record
		// substantially more expensive. Keep identical topology paths while the
		// normal suite retains the plan-required 1,024-slot scale assertion.
		slotCount = 64
	}
	nodes, clusterCfg := startManagerExecutorCluster(t, 5, slotCount)
	raftNodes, _ := newInmemRaftCluster(t, 5)
	leader := waitForLeader(t, raftNodes)

	initial := nodes[1].manager.Metadata().Snapshot()
	failedView := viewFromManagerMetadataFailures(initial, "node-1")
	failedView.ClusterID = initial.ClusterID
	commitHarnessCommand(t, leader, CommandObserveView, failedView)
	plan, needed := PlanFailover(failedView)
	if !needed {
		t.Fatal("single-node failure produced no recovery plan")
	}
	commitHarnessCommand(t, leader, CommandProposePlan, plan)

	// Lose the controller leader after the plan is committed but before its
	// first promotion. A new majority leader must resume the same plan.
	for i, node := range raftNodes {
		if node == leader {
			if err := node.Shutdown(); err != nil {
				t.Fatal(err)
			}
			raftNodes[i] = nil
			break
		}
	}
	leader = waitForLeader(t, raftNodes)
	nodes[0].stopData()
	nodes[0].reachable = false

	clients := make(map[string]AdminNodeAPI, len(nodes))
	for _, node := range nodes {
		client, err := NewAuthenticatedNodeClient(node.http.URL, 2*time.Second, "secret")
		if err != nil {
			t.Fatal(err)
		}
		clients[node.id] = client
	}
	executorCfg := clusterCfg
	executorCfg.Controller = config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 2000, MigrationRateLimit: 100000}
	if err := NewExecutor(executorCfg, clients, leader).ExecuteOnce(context.Background()); err != nil {
		t.Fatalf("resumed recovery: %v", err)
	}
	if leader.State().ActivePlan != nil || leader.State().LastCompletedPlanID != plan.ID {
		t.Fatalf("recovery plan did not complete exactly once: %+v", leader.State().ActivePlan)
	}

	// Rebalance repeatedly through the normal assign+full-sync path until the
	// deterministic skew gate reports convergence.
	for round := 0; round < 8; round++ {
		metadata := nodes[1].manager.Metadata().Snapshot()
		view := viewFromManagerMetadataFailures(metadata, "node-1")
		view.ClusterID = metadata.ClusterID
		commitHarnessCommand(t, leader, CommandObserveView, view)
		rebalance, needed := PlanRebalance(view, 1, 1024)
		if !needed {
			break
		}
		commitHarnessCommand(t, leader, CommandProposeRebalance, rebalance)
		if err := NewExecutor(executorCfg, clients, leader).ExecuteOnce(context.Background()); err != nil {
			t.Fatalf("rebalance round %d: %v", round, err)
		}
	}

	final := nodes[1].manager.Metadata().Snapshot()
	leaders := map[string]int{}
	replicas := map[string]int{}
	for _, slot := range final.Slots {
		if slot.LeaderID == "node-1" || slot.ReplicaID == "node-1" {
			t.Fatalf("failed node retained slot %d ownership: %+v", slot.Number, slot)
		}
		if slot.LeaderID == slot.ReplicaID || !slot.ReplicaReady {
			t.Fatalf("slot %d did not converge to two healthy copies: %+v", slot.Number, slot)
		}
		leaders[slot.LeaderID]++
		replicas[slot.ReplicaID]++
	}
	target := int(slotCount / 4)
	for i := 2; i <= 5; i++ {
		id := nodeID(i)
		if math.Abs(float64(leaders[id]-target)) > 1 || math.Abs(float64(replicas[id]-target)) > 1 {
			t.Fatalf("%s placement is not balanced: leaders=%d replicas=%d", id, leaders[id], replicas[id])
		}
	}

	// Re-executing after completion is a strict no-op.
	version := final.Version
	if err := NewExecutor(executorCfg, clients, leader).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := nodes[1].manager.Metadata().Snapshot().Version; got != version {
		t.Fatalf("duplicate execution changed metadata: before=%d after=%d", version, got)
	}
}

func TestFiveNodeHarnessReplicaFailurePreservesSurvivingLeaders(t *testing.T) {
	nodes, cfg := startManagerExecutorCluster(t, 5, 25)
	before := nodes[0].manager.Metadata().Snapshot()
	preserved := map[uint32]string{}
	for _, slot := range before.Slots {
		if slot.ReplicaID == "node-2" && slot.LeaderID != "node-2" {
			preserved[slot.Number] = slot.LeaderID
		}
	}
	nodes[1].stopData()
	nodes[1].reachable = false
	view := viewFromManagerMetadataFailures(before, "node-2")
	view.ClusterID = before.ClusterID
	plan, needed := PlanFailover(view)
	if !needed {
		t.Fatal("replica-holder failure produced no plan")
	}
	proposer := newFSMProposer(t, view, plan)
	clients := harnessAdminClients(t, nodes)
	cfg.Controller = config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 1000, MigrationRateLimit: 100000}
	if err := NewExecutor(cfg, clients, proposer).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := nodes[0].manager.Metadata().Snapshot()
	for slot, leaderID := range preserved {
		if after.Slots[slot].LeaderID != leaderID {
			t.Fatalf("replica-only failure moved slot %d leader: before=%s after=%s", slot, leaderID, after.Slots[slot].LeaderID)
		}
	}
}

func TestFiveNodeHarnessSequentialFailuresAfterFullRepair(t *testing.T) {
	nodes, cfg := startManagerExecutorCluster(t, 5, 30)
	cfg.Controller = config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 1000, MigrationRateLimit: 100000}
	clients := harnessAdminClients(t, nodes)

	nodes[0].stopData()
	nodes[0].reachable = false
	firstView := viewFromManagerMetadataFailures(nodes[1].manager.Metadata().Snapshot(), "node-1")
	firstView.ClusterID = cfg.ID
	firstPlan, needed := PlanFailover(firstView)
	if !needed {
		t.Fatal("first failure produced no plan")
	}
	proposer := newFSMProposer(t, firstView, firstPlan)
	if err := NewExecutor(cfg, clients, proposer).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	nodes[1].stopData()
	nodes[1].reachable = false
	secondView := viewFromManagerMetadataFailures(nodes[2].manager.Metadata().Snapshot(), "node-1", "node-2")
	secondView.ClusterID = cfg.ID
	secondPlan, needed := PlanFailover(secondView)
	if !needed || len(secondPlan.Unrecoverable) != 0 {
		t.Fatalf("second post-repair failure was not safely recoverable: %+v", secondPlan)
	}
	proposer.apply(t, CommandObserveView, secondView)
	proposer.apply(t, CommandProposePlan, secondPlan)
	if err := NewExecutor(cfg, clients, proposer).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	final := nodes[2].manager.Metadata().Snapshot()
	for _, slot := range final.Slots {
		if slot.LeaderID == "node-1" || slot.LeaderID == "node-2" || slot.ReplicaID == "node-1" || slot.ReplicaID == "node-2" || !slot.ReplicaReady {
			t.Fatalf("slot %d did not repair after sequential failures: %+v", slot.Number, slot)
		}
	}
}

func harnessAdminClients(t *testing.T, nodes []*managerExecutorNode) map[string]AdminNodeAPI {
	t.Helper()
	clients := make(map[string]AdminNodeAPI, len(nodes))
	for _, node := range nodes {
		client, err := NewAuthenticatedNodeClient(node.http.URL, 2*time.Second, "secret")
		if err != nil {
			t.Fatal(err)
		}
		clients[node.id] = client
	}
	return clients
}

func commitHarnessCommand(t *testing.T, leader *RaftNode, commandType CommandType, payload any) {
	t.Helper()
	command, err := NewCommand(commandType, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := leader.Propose(command); err != nil {
		t.Fatal(err)
	}
}

func nodeID(number int) string {
	return fmt.Sprintf("node-%d", number)
}
