package controller

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/mnemokv/mnemokv/internal/config"
)

func TestPlanFailoverClassifiesAndOrdersSteps(t *testing.T) {
	tests := []struct {
		name          string
		view          ClusterView
		wantKinds     []StepKind
		wantTarget    string
		unrecoverable []uint32
		writeBlocked  []uint32
	}{
		{
			name: "leaderless promotes then repairs",
			view: plannerView(
				[]SlotView{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}},
				failedNode("node-1"), eligibleNode("node-2", 4, 0), eligibleNode("node-3", 0, 0),
			),
			wantKinds: []StepKind{StepPromote, StepAssignReplica, StepSync}, wantTarget: "node-3", writeBlocked: []uint32{0},
		},
		{
			name: "replica lost repairs without promotion",
			view: plannerView(
				[]SlotView{{Number: 1, LeaderID: "node-2", ReplicaID: "node-1", Term: 1, ReplicaReady: true}},
				failedNode("node-1"), eligibleNode("node-2", 1, 0), eligibleNode("node-3", 0, 0),
			),
			wantKinds: []StepKind{StepAssignReplica, StepSync}, wantTarget: "node-3",
		},
		{
			name: "no surviving copy is never assigned",
			view: plannerView(
				[]SlotView{{Number: 2, LeaderID: "node-1", ReplicaID: "node-2", Term: 1, ReplicaReady: true}},
				failedNode("node-1"), failedNode("node-2"), eligibleNode("node-3", 0, 0),
			),
			wantKinds: []StepKind{StepMarkUnavailable}, unrecoverable: []uint32{2},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, ok := PlanFailover(tc.view)
			if !ok {
				t.Fatal("expected recovery plan")
			}
			kinds := make([]StepKind, len(plan.Steps))
			for i, step := range plan.Steps {
				kinds[i] = step.Kind
			}
			if !reflect.DeepEqual(kinds, tc.wantKinds) || !reflect.DeepEqual(plan.Unrecoverable, tc.unrecoverable) || !reflect.DeepEqual(plan.WriteBlockedSlots, tc.writeBlocked) {
				t.Fatalf("unexpected plan: %+v", plan)
			}
			if tc.wantTarget != "" && plan.Steps[len(plan.Steps)-1].Target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", plan.Steps[len(plan.Steps)-1].Target, tc.wantTarget)
			}
		})
	}
}

func TestPlanFailoverHealthyClusterHasNoPlan(t *testing.T) {
	view := plannerView(
		[]SlotView{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true}},
		eligibleNode("node-1", 1, 0), eligibleNode("node-2", 0, 1), eligibleNode("node-3", 0, 0),
	)
	if plan, ok := PlanFailover(view); ok {
		t.Fatalf("healthy cluster produced plan: %+v", plan)
	}
}

func TestPlanFailoverResumesAssignedReplicaWithSyncOnly(t *testing.T) {
	view := plannerView(
		[]SlotView{{Number: 3, LeaderID: "node-2", ReplicaID: "node-3", ReplicaReady: false}},
		failedNode("node-1"), eligibleNode("node-2", 1, 0), eligibleNode("node-3", 0, 1),
	)
	plan, ok := PlanFailover(view)
	if !ok || len(plan.Steps) != 1 || plan.Steps[0].Kind != StepSync || plan.Steps[0].Target != "node-3" {
		t.Fatalf("assigned replica should resume at sync: %+v", plan)
	}
}

func TestPlanFailoverIsDeterministicAndUsesNodeIDTieBreak(t *testing.T) {
	slots := []SlotView{{Number: 4, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true}}
	left := plannerView(slots, failedNode("node-1"), eligibleNode("node-2", 1, 0), eligibleNode("node-4", 0, 0), eligibleNode("node-3", 0, 0))
	right := plannerView(slots, eligibleNode("node-3", 0, 0), failedNode("node-1"), eligibleNode("node-4", 0, 0), eligibleNode("node-2", 1, 0))
	leftPlan, _ := PlanFailover(left)
	rightPlan, _ := PlanFailover(right)
	if !reflect.DeepEqual(leftPlan, rightPlan) {
		t.Fatalf("planner is not deterministic:\nleft=%+v\nright=%+v", leftPlan, rightPlan)
	}
	if leftPlan.Steps[1].Target != "node-3" {
		t.Fatalf("tie-break target = %q, want node-3", leftPlan.Steps[1].Target)
	}
}

func TestPlannerQuorumDuplicateAndSupersession(t *testing.T) {
	initial := plannerView(
		[]SlotView{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true}},
		failedNode("node-1"), eligibleNode("node-2", 1, 0), eligibleNode("node-3", 0, 0),
	)

	t.Run("no plan without quorum", func(t *testing.T) {
		proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: initial}, proposeErr: raft.ErrLeadershipLost}
		if err := NewPlanner(proposer, config.ControllerConfig{}).Evaluate(); !errors.Is(err, raft.ErrLeadershipLost) {
			t.Fatalf("evaluate error = %v", err)
		}
		if proposer.state.ActivePlan != nil {
			t.Fatal("uncommitted proposal became active")
		}
	})

	t.Run("duplicate proposal suppressed", func(t *testing.T) {
		plan, _ := PlanFailover(initial)
		proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: initial, ActivePlan: &plan}}
		if err := NewPlanner(proposer, config.ControllerConfig{}).Evaluate(); err != nil {
			t.Fatal(err)
		}
		if len(proposer.commands) != 0 {
			t.Fatalf("duplicate commands = %d", len(proposer.commands))
		}
	})

	t.Run("new failure atomically supersedes", func(t *testing.T) {
		active, _ := PlanFailover(initial)
		second := plannerView(
			[]SlotView{{Number: 0, LeaderID: "node-1", ReplicaID: "node-2", ReplicaReady: true}},
			failedNode("node-1"), failedNode("node-2"), eligibleNode("node-3", 0, 0),
		)
		proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: second, ActivePlan: &active}}
		if err := NewPlanner(proposer, config.ControllerConfig{}).Evaluate(); err != nil {
			t.Fatal(err)
		}
		if len(proposer.commands) != 1 || proposer.commands[0].Type != CommandSupersedePlan {
			t.Fatalf("unexpected commands: %+v", proposer.commands)
		}
		if proposer.state.ActivePlan == nil || !reflect.DeepEqual(proposer.state.ActivePlan.DeadNodes, []string{"node-1", "node-2"}) {
			t.Fatalf("plan was not superseded: %+v", proposer.state.ActivePlan)
		}
	})
}

func TestPlanRebalanceDetectsSkewAndCapsMoves(t *testing.T) {
	view := skewedRebalanceView()
	plan, ok := PlanRebalance(view, 1, 2)
	if !ok {
		t.Fatal("skewed placement produced no plan")
	}
	if plan.Kind != PlanKindRebalance || len(plan.WriteBlockedSlots) != 2 {
		t.Fatalf("rebalance cap not honored: %+v", plan)
	}
	if len(plan.Steps) < 5 || plan.Steps[0].Kind != StepAssignReplica || plan.Steps[1].Kind != StepSync || plan.Steps[2].Kind != StepPromote {
		t.Fatalf("leadership handoff sequence is incomplete: %+v", plan.Steps)
	}
	for _, step := range plan.Steps {
		if step.Target == "node-1" {
			t.Fatal("failed node was selected for rebalancing")
		}
	}
}

func TestPlanRebalanceRefusesBalancedOrUnreadyTopology(t *testing.T) {
	balanced := plannerView(
		[]SlotView{
			{Number: 0, LeaderID: "node-2", ReplicaID: "node-3", ReplicaReady: true},
			{Number: 1, LeaderID: "node-3", ReplicaID: "node-2", ReplicaReady: true},
		},
		failedNode("node-1"), eligibleNode("node-2", 1, 1), eligibleNode("node-3", 1, 1),
	)
	if plan, ok := PlanRebalance(balanced, 1, 10); ok {
		t.Fatalf("balanced topology produced plan: %+v", plan)
	}
	balanced.Slots[0].ReplicaReady = false
	balanced.Nodes["node-2"] = eligibleNode("node-2", 2, 0)
	balanced.Nodes["node-3"] = eligibleNode("node-3", 0, 2)
	if plan, ok := PlanRebalance(balanced, 1, 10); ok {
		t.Fatalf("unready topology produced plan: %+v", plan)
	}
}

func TestPlannerRebalancesAfterEligibleCooldownDespiteFailedNode(t *testing.T) {
	view := skewedRebalanceView()
	proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: view}}
	planner := NewPlanner(proposer, config.ControllerConfig{ObserveIntervalMs: 1, FailureTimeoutMs: 100, RebalanceSkewThreshold: 1, MigrationRateLimit: 10})
	now := time.Unix(5000, 0)
	planner.now = func() time.Time { return now }
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(99 * time.Millisecond)
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 0 {
		t.Fatal("rebalance started before cooldown")
	}
	now = now.Add(2 * time.Millisecond)
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 1 || proposer.commands[0].Type != CommandProposeRebalance {
		t.Fatalf("rebalance did not start with failed node excluded: %+v", proposer.commands)
	}
}

func TestPlannerRefusesRebalanceWithActivePlanOrUnstableEligibleSet(t *testing.T) {
	view := skewedRebalanceView()
	active := RecoveryPlan{ID: "recovery", Kind: PlanKindRecovery}
	proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: view, ActivePlan: &active}}
	planner := NewPlanner(proposer, config.ControllerConfig{FailureTimeoutMs: 1, RebalanceSkewThreshold: 1, MigrationRateLimit: 10})
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 0 {
		t.Fatal("rebalance started with active recovery")
	}
	proposer.state.ActivePlan = nil
	suspect := proposer.state.LatestView.Nodes["node-3"]
	suspect.Suspected = true
	proposer.state.LatestView.Nodes["node-3"] = suspect
	if err := planner.Evaluate(); err != nil {
		t.Fatal(err)
	}
	if len(proposer.commands) != 0 {
		t.Fatal("rebalance started with unstable eligible set")
	}
}

func skewedRebalanceView() ClusterView {
	slots := []SlotView{
		{Number: 0, LeaderID: "node-2", ReplicaID: "node-3", ReplicaReady: true},
		{Number: 1, LeaderID: "node-2", ReplicaID: "node-3", ReplicaReady: true},
		{Number: 2, LeaderID: "node-2", ReplicaID: "node-4", ReplicaReady: true},
		{Number: 3, LeaderID: "node-2", ReplicaID: "node-4", ReplicaReady: true},
		{Number: 4, LeaderID: "node-2", ReplicaID: "node-5", ReplicaReady: true},
		{Number: 5, LeaderID: "node-3", ReplicaID: "node-2", ReplicaReady: true},
		{Number: 6, LeaderID: "node-4", ReplicaID: "node-2", ReplicaReady: true},
		{Number: 7, LeaderID: "node-5", ReplicaID: "node-2", ReplicaReady: true},
	}
	view := plannerView(slots,
		failedNode("node-1"), eligibleNode("node-2", 0, 0), eligibleNode("node-3", 0, 0),
		eligibleNode("node-4", 0, 0), eligibleNode("node-5", 0, 0),
	)
	for _, slot := range slots {
		leader := view.Nodes[slot.LeaderID]
		leader.LeaderSlots++
		view.Nodes[slot.LeaderID] = leader
		replica := view.Nodes[slot.ReplicaID]
		replica.ReplicaSlots++
		view.Nodes[slot.ReplicaID] = replica
	}
	view.Status = summarizeStatusWithThreshold(view, 1)
	return view
}

func plannerView(slots []SlotView, nodes ...NodeView) ClusterView {
	view := ClusterView{MetadataVersion: 10, Slots: append([]SlotView(nil), slots...), Nodes: make(map[string]NodeView, len(nodes))}
	for _, node := range nodes {
		view.Nodes[node.ID] = node
	}
	view.Status = summarizeStatus(view)
	return view
}

func eligibleNode(id string, leaders, replicas int) NodeView {
	return NodeView{ID: id, Reachable: true, Eligible: true, LeaderSlots: leaders, ReplicaSlots: replicas}
}

func failedNode(id string) NodeView { return NodeView{ID: id} }

type plannerProposer struct {
	leader     bool
	state      FSMSnapshot
	proposeErr error
	commands   []Command
}

func (p *plannerProposer) IsLeader() bool     { return p.leader }
func (p *plannerProposer) State() FSMSnapshot { return cloneState(p.state) }
func (p *plannerProposer) Propose(command Command) error {
	p.commands = append(p.commands, command)
	if p.proposeErr != nil {
		return p.proposeErr
	}
	switch command.Type {
	case CommandProposePlan:
		var plan RecoveryPlan
		_ = json.Unmarshal(command.Payload, &plan)
		p.state.ActivePlan = &plan
	case CommandSupersedePlan:
		var payload SupersedePlanPayload
		_ = json.Unmarshal(command.Payload, &payload)
		p.state.ActivePlan = &payload.NewPlan
	}
	return nil
}
