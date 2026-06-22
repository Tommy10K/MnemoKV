package controller

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/hashicorp/raft"
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
		if err := NewPlanner(proposer, 0).Evaluate(); !errors.Is(err, raft.ErrLeadershipLost) {
			t.Fatalf("evaluate error = %v", err)
		}
		if proposer.state.ActivePlan != nil {
			t.Fatal("uncommitted proposal became active")
		}
	})

	t.Run("duplicate proposal suppressed", func(t *testing.T) {
		plan, _ := PlanFailover(initial)
		proposer := &plannerProposer{leader: true, state: FSMSnapshot{LatestView: initial, ActivePlan: &plan}}
		if err := NewPlanner(proposer, 0).Evaluate(); err != nil {
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
		if err := NewPlanner(proposer, 0).Evaluate(); err != nil {
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
