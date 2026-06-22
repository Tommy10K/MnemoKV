package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func TestSecondFailureDuringRepairPreservesSurvivingAndUnaffectedSlots(t *testing.T) {
	nodes, cfg := startManagerExecutorCluster(t, 5, 5)
	metadata := nodes[0].manager.Metadata()
	keys := make([]string, 5)
	for slot := uint32(0); slot < 5; slot++ {
		keys[slot] = keyForExactSlot(t, metadata, slot)
		owner, _ := metadata.Slot(slot)
		ownerIndex := int(owner.LeaderID[len("node-")] - '1')
		frame := nodes[ownerIndex].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(keys[slot]), []byte("before")}})
		if frame != resp.OK {
			t.Fatalf("seed slot %d: %#v", slot, frame)
		}
	}

	firstView := viewFromManagerMetadataFailures(nodes[2].manager.Metadata().Snapshot(), "node-1")
	firstPlan, ok := PlanFailover(firstView)
	if !ok {
		t.Fatal("first failure produced no plan")
	}
	proposer := newFSMProposer(t, firstView, firstPlan)

	nodes[0].stopData()
	nodes[0].reachable = false
	nodes[1].stopData()
	nodes[1].reachable = false

	// Slot 2 is owned by node-3/node-4 and must remain writable while slots
	// affected by the overlapping failures are being replanned.
	if frame := nodes[2].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(keys[2]), []byte("unaffected")}}); frame != resp.OK {
		t.Fatalf("unaffected slot stopped serving: %#v", frame)
	}
	if frame := nodes[2].manager.Coordinator().Execute(&resp.Command{Name: "GET", Args: [][]byte{[]byte(keys[1])}}); !isClusterDown(frame) {
		t.Fatalf("leaderless slot served before promotion: %#v", frame)
	}

	secondView := viewFromManagerMetadataFailures(nodes[2].manager.Metadata().Snapshot(), "node-1", "node-2")
	proposer.apply(t, CommandObserveView, secondView)
	planner := NewPlanner(proposer, config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 100, RebalanceSkewThreshold: 1, MigrationRateLimit: 100})
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	superseded := proposer.State().ActivePlan
	if superseded == nil || superseded.ID == firstPlan.ID || len(superseded.Unrecoverable) != 1 || superseded.Unrecoverable[0] != 0 {
		t.Fatalf("overlapping failure did not supersede from surviving copies: %+v", superseded)
	}

	clients := make(map[string]AdminNodeAPI, len(nodes))
	for _, node := range nodes {
		client, err := NewAuthenticatedNodeClient(node.http.URL, 0, "secret")
		if err != nil {
			t.Fatal(err)
		}
		clients[node.id] = client
	}
	cfg.Controller = config.ControllerConfig{ObserveIntervalMs: 5, FailureTimeoutMs: 1000, MigrationRateLimit: 1000}
	if err := NewExecutor(cfg, clients, proposer).ExecuteOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := proposer.State()
	unavailable, ok := state.Unavailable[0]
	if !ok || unavailable.LeaderID != "node-1" || unavailable.ReplicaID != "node-2" {
		t.Fatalf("last-copy slot was not committed unavailable: %+v", state.Unavailable)
	}
	slotZero, _ := nodes[2].manager.Metadata().Slot(0)
	if slotZero.LeaderID != "node-1" || slotZero.ReplicaID != "node-2" {
		t.Fatalf("unavailable slot was silently reassigned: %+v", slotZero)
	}
	if frame := nodes[2].manager.Coordinator().Execute(&resp.Command{Name: "GET", Args: [][]byte{[]byte(keys[0])}}); !isClusterDown(frame) {
		t.Fatalf("unavailable slot read did not fail clearly: %#v", frame)
	}
	if frame := nodes[2].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(keys[0]), []byte("must-not-appear")}}); !isClusterDown(frame) {
		t.Fatalf("unavailable slot write did not fail clearly: %#v", frame)
	}
	if frame := nodes[2].manager.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(keys[1]), []byte("recovered")}}); frame != resp.OK {
		t.Fatalf("recoverable slot did not resume writes: %#v", frame)
	}

	finalView := viewFromManagerMetadataFailures(nodes[2].manager.Metadata().Snapshot(), "node-1", "node-2")
	proposer.apply(t, CommandObserveView, finalView)
	status := BuildStatusSnapshot(proposer.State())
	if status.State != string(StatusPotentialDataLoss) || len(status.UnavailableSlots) != 1 || len(status.OneCopySlots) != 0 {
		t.Fatalf("unexpected final overlapping-failure status: %+v", status)
	}
	detail := status.UnavailableSlots[0]
	if detail.Slot != 0 || !strings.Contains(detail.Message, "no authoritative copy") || len(detail.Failures) != 2 || status.ReturningNodeDataPolicy == "" {
		t.Fatalf("unavailable detail is incomplete: %+v status=%+v", detail, status)
	}
}

func isClusterDown(frame resp.Frame) bool {
	err, ok := frame.(resp.Error)
	return ok && err.Prefix == "CLUSTERDOWN"
}

func TestBuildStatusSnapshotReportsOneCopySemanticsAndRanges(t *testing.T) {
	view := plannerView(
		[]SlotView{
			{Number: 3, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true},
			{Number: 4, LeaderID: "node-3", ReplicaID: "node-1", ReplicaReady: true},
			{Number: 5, LeaderID: "node-3", ReplicaID: "node-2", ReplicaReady: true},
		},
		failedNode("node-1"), eligibleNode("node-2", 0, 1), eligibleNode("node-3", 2, 0),
	)
	status := BuildStatusSnapshot(FSMSnapshot{LatestView: view, ControlIndex: 12})
	if status.ControlIndex != 12 || len(status.OneCopySlots) != 2 || len(status.AffectedSlotRanges) != 2 {
		t.Fatalf("unexpected degraded status: %+v", status)
	}
	if status.OneCopySlots[0].ReadsAvailable || status.OneCopySlots[0].WritesAvailable {
		t.Fatalf("leaderless slot availability is dishonest: %+v", status.OneCopySlots[0])
	}
	if !status.OneCopySlots[1].ReadsAvailable || status.OneCopySlots[1].WritesAvailable {
		t.Fatalf("replica-lost slot availability is dishonest: %+v", status.OneCopySlots[1])
	}
}
