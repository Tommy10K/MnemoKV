package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
)

type fakeReturningNode struct {
	prepareCalls int
	admitCalls   int
	clusterID    string
	version      uint64
}

func (n *fakeReturningNode) PrepareReturning(context.Context, string, uint64, uint64) (ReturningNodeResponse, error) {
	n.prepareCalls++
	return ReturningNodeResponse{ClusterID: n.clusterID, MetadataVersion: n.version, EntryCount: 0, RemovedSnapshots: 1, DataState: "recovering"}, nil
}

func (n *fakeReturningNode) AdmitReturning(context.Context, string, uint64, uint64) (ReturningNodeResponse, error) {
	n.admitCalls++
	return ReturningNodeResponse{ClusterID: n.clusterID, MetadataVersion: n.version, EntryCount: 0, DataState: "active"}, nil
}

func TestReturningNodeControllerCommitsAdmissionAfterValidation(t *testing.T) {
	proposer := &fsmProposer{leader: true, fsm: NewFSM()}
	view := ClusterView{ClusterID: "cluster", MetadataVersion: 12, Nodes: map[string]NodeView{
		"node-1": {ID: "node-1", Reachable: true, Returning: true},
	}}
	proposer.apply(t, CommandObserveView, view)
	node := &fakeReturningNode{clusterID: "cluster", version: 12}
	worker := NewReturningNodeController(config.ClusterConfig{}, map[string]ReturningNodeAPI{"node-1": node}, proposer)
	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if admitted, tracked := proposer.State().ReturningNodes["node-1"]; !tracked || admitted || node.prepareCalls != 0 {
		t.Fatalf("node was not committed ineligible first: tracked=%v admitted=%v prepare=%d", tracked, admitted, node.prepareCalls)
	}
	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := proposer.State()
	if !state.ReturningNodes["node-1"] || node.prepareCalls != 1 || node.admitCalls != 1 {
		t.Fatalf("admission did not converge: state=%v prepare=%d admit=%d", state.ReturningNodes, node.prepareCalls, node.admitCalls)
	}
}

func TestReturningNodeWaitsForActiveRecoveryAndDoesNotClearUnavailable(t *testing.T) {
	proposer := &fsmProposer{leader: true, fsm: NewFSM()}
	view := ClusterView{ClusterID: "cluster", MetadataVersion: 12, Nodes: map[string]NodeView{
		"node-1": {ID: "node-1", Reachable: true, Returning: true},
	}}
	proposer.apply(t, CommandObserveView, view)
	proposer.apply(t, CommandMarkUnavailable, []UnavailableSlot{{Slot: 3, LeaderID: "node-1", ReplicaID: "node-2"}})
	proposer.apply(t, CommandProposePlan, RecoveryPlan{ID: "active", Kind: PlanKindRecovery, Done: map[int]bool{}})
	node := &fakeReturningNode{clusterID: "cluster", version: 12}
	worker := NewReturningNodeController(config.ClusterConfig{}, map[string]ReturningNodeAPI{"node-1": node}, proposer)
	if err := worker.ReconcileOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := proposer.State()
	if node.prepareCalls != 0 || len(state.ReturningNodes) != 0 {
		t.Fatalf("returning node advanced during recovery: prepare=%d state=%v", node.prepareCalls, state.ReturningNodes)
	}
	if _, ok := state.Unavailable[3]; !ok {
		t.Fatal("returning node cleared potential-data-loss classification")
	}
}

func TestAdmittedFifthNodeIsFilledOnlyByNormalRebalanceSync(t *testing.T) {
	view := balancedFourNodeView(20)
	view.Nodes["node-1"] = eligibleNode("node-1", 0, 0)
	plan, needed := PlanRebalance(view, 1, 100)
	if !needed {
		t.Fatal("four-to-five rebalance was not planned")
	}
	assigns, syncs := 0, 0
	for _, step := range plan.Steps {
		if step.Target != "node-1" {
			continue
		}
		if step.Kind == StepAssignReplica {
			assigns++
		}
		if step.Kind == StepSync {
			syncs++
		}
	}
	if assigns == 0 || assigns != syncs {
		t.Fatalf("new ownership bypassed full-slot sync: assigns=%d syncs=%d", assigns, syncs)
	}
}

func balancedFourNodeView(slotCount int) ClusterView {
	nodes := map[string]NodeView{}
	for i := 2; i <= 5; i++ {
		id := fmt.Sprintf("node-%d", i)
		nodes[id] = eligibleNode(id, 0, 0)
	}
	view := ClusterView{ClusterID: "cluster", MetadataVersion: 20, Nodes: nodes, Slots: make([]SlotView, slotCount)}
	ids := []string{"node-2", "node-3", "node-4", "node-5"}
	for i := range view.Slots {
		leader, replica := ids[i%4], ids[(i+1)%4]
		view.Slots[i] = SlotView{Number: uint32(i), LeaderID: leader, ReplicaID: replica, Term: 1, ReplicaReady: true}
		ln, rn := view.Nodes[leader], view.Nodes[replica]
		ln.LeaderSlots++
		rn.ReplicaSlots++
		view.Nodes[leader], view.Nodes[replica] = ln, rn
	}
	view.Status = summarizeStatus(view)
	return view
}
