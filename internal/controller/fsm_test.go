package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func TestFiveMemberRaftFSMAndQuorum(t *testing.T) {
	nodes, transports := newInmemRaftCluster(t, 5)
	leader := waitForLeader(t, nodes)

	for _, node := range nodes {
		if node != leader {
			command, _ := NewCommand(CommandObserveView, ClusterView{MetadataVersion: 1})
			if err := node.Propose(command); !errors.Is(err, raft.ErrNotLeader) {
				t.Fatalf("follower propose error = %v, want not leader", err)
			}
			break
		}
	}

	command, err := NewCommand(CommandObserveView, ClusterView{MetadataVersion: 7, ObservedAt: time.Unix(100, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := leader.Propose(command); err != nil {
		t.Fatalf("propose: %v", err)
	}
	waitForState(t, nodes, func(state FSMSnapshot) bool { return state.LatestView.MetadataVersion == 7 })

	followers := make([]int, 0, 4)
	for i, node := range nodes {
		if node != leader {
			followers = append(followers, i)
		}
	}
	shutdownMember(t, nodes, transports, followers[0])
	shutdownMember(t, nodes, transports, followers[1])
	command, _ = NewCommand(CommandObserveView, ClusterView{MetadataVersion: 8})
	if err := leader.Propose(command); err != nil {
		t.Fatalf("five voters should commit with two down: %v", err)
	}

	shutdownMember(t, nodes, transports, followers[2])
	command, _ = NewCommand(CommandObserveView, ClusterView{MetadataVersion: 9})
	if err := leader.Propose(command); err == nil {
		t.Fatal("five voters unexpectedly committed with three down")
	}
}

func TestFiveMemberRaftPartitionShapes(t *testing.T) {
	nodes, transports := newInmemRaftCluster(t, 5)
	leader := waitForLeader(t, nodes)
	leaderIndex := 0
	for i, node := range nodes {
		if node == leader {
			leaderIndex = i
			break
		}
	}
	majority := []int{leaderIndex}
	minority := make([]int, 0, 2)
	for i := range nodes {
		if i == leaderIndex {
			continue
		}
		if len(majority) < 3 {
			majority = append(majority, i)
		} else {
			minority = append(minority, i)
		}
	}
	partitionTransports(transports, majority, minority)
	command, _ := NewCommand(CommandObserveView, ClusterView{MetadataVersion: 30})
	if err := leader.Propose(command); err != nil {
		t.Fatalf("3-2 majority did not proceed: %v", err)
	}

	reconnectTransports(transports)
	waitForState(t, nodes, func(state FSMSnapshot) bool { return state.LatestView.MetadataVersion == 30 })
	leader = waitForLeader(t, nodes)
	leaderIndex = 0
	for i, node := range nodes {
		if node == leader {
			leaderIndex = i
			break
		}
	}
	remaining := make([]int, 0, 4)
	for i := range nodes {
		if i != leaderIndex {
			remaining = append(remaining, i)
		}
	}
	groupA := []int{leaderIndex, remaining[0]}
	groupB := []int{remaining[1], remaining[2]}
	groupC := []int{remaining[3]}
	partitionTransports(transports, groupA, append(append([]int{}, groupB...), groupC...))
	partitionTransports(transports, groupB, groupC)
	command, _ = NewCommand(CommandObserveView, ClusterView{MetadataVersion: 31})
	if err := leader.Propose(command); err == nil {
		t.Fatal("2-2-1 partition unexpectedly committed")
	}
	for _, node := range nodes {
		if node.State().LatestView.MetadataVersion == 31 {
			t.Fatal("ownership advanced without a partition majority")
		}
	}
}

func partitionTransports(transports []*raft.InmemTransport, left, right []int) {
	for _, i := range left {
		for _, j := range right {
			transports[i].Disconnect(transports[j].LocalAddr())
			transports[j].Disconnect(transports[i].LocalAddr())
		}
	}
}

func reconnectTransports(transports []*raft.InmemTransport) {
	for i := range transports {
		for j := range transports {
			if i != j {
				transports[i].Connect(transports[j].LocalAddr(), transports[j])
			}
		}
	}
}

func TestFSMSnapshotRestoreRoundTrip(t *testing.T) {
	fsm := NewFSM()
	command, _ := NewCommand(CommandMarkUnavailable, []UnavailableSlot{{Slot: 12, LeaderID: "n1", ReplicaID: "n2", Failures: []string{"n1", "n2"}}})
	raw, _ := json.Marshal(command)
	if result := fsm.Apply(&raft.Log{Index: 42, Data: raw}); result != nil {
		t.Fatalf("apply: %v", result)
	}
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	sink := &memorySnapshotSink{id: "test"}
	if err := snapshot.Persist(sink); err != nil {
		t.Fatal(err)
	}
	restored := NewFSM()
	if err := restored.Restore(io.NopCloser(bytes.NewReader(sink.Bytes()))); err != nil {
		t.Fatal(err)
	}
	state := restored.State()
	if state.ControlIndex != 42 || state.Unavailable[12].LeaderID != "n1" {
		t.Fatalf("unexpected restored state: %+v", state)
	}
}

func TestFSMRecordsLastCompletedRebalanceAtCommittedIndex(t *testing.T) {
	fsm := NewFSM()
	applyFSMCommand(t, fsm, 1, CommandObserveView, ClusterView{MetadataVersion: 5})
	plan := RecoveryPlan{ID: "rebalance-5", Kind: PlanKindRebalance, Epoch: 5, Done: map[int]bool{}}
	applyFSMCommand(t, fsm, 2, CommandProposeRebalance, plan)
	applyFSMCommand(t, fsm, 3, CommandPlanComplete, PlanIDPayload{PlanID: plan.ID})
	got := fsm.State().LastRebalance
	if got == nil || got.ID != plan.ID || got.Epoch != 5 || got.ControlIndex != 3 {
		t.Fatalf("last rebalance = %+v", got)
	}
}

func TestFSMObservationPreservesActivePlanStatus(t *testing.T) {
	fsm := NewFSM()
	view := ClusterView{Status: StatusSummary{State: StatusUnavailable}}
	applyFSMCommand(t, fsm, 1, CommandObserveView, view)
	plan := RecoveryPlan{ID: "recovery", Kind: PlanKindRecovery, Steps: []PlanStep{{Kind: StepPromote, Slot: 1}}, Done: map[int]bool{}}
	applyFSMCommand(t, fsm, 2, CommandProposePlan, plan)
	view.MetadataVersion = 2
	applyFSMCommand(t, fsm, 3, CommandObserveView, view)
	if got := fsm.State().LatestView.Status.State; got != StatusPromoting {
		t.Fatalf("active plan state after observation = %s", got)
	}
}

func applyFSMCommand(t *testing.T, fsm *FSM, index uint64, commandType CommandType, payload any) {
	t.Helper()
	command, err := NewCommand(commandType, payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(command)
	if result := fsm.Apply(&raft.Log{Index: index, Data: raw}); result != nil {
		t.Fatal(result)
	}
}

func newInmemRaftCluster(t *testing.T, size int) ([]*RaftNode, []*raft.InmemTransport) {
	t.Helper()
	addresses := make([]raft.ServerAddress, size)
	transports := make([]*raft.InmemTransport, size)
	peers := make([]RaftPeer, size)
	for i := 0; i < size; i++ {
		id := fmt.Sprintf("node-%d", i+1)
		addresses[i], transports[i] = raft.NewInmemTransport(raft.ServerAddress(id))
		peers[i] = RaftPeer{ID: raft.ServerID(id), Address: addresses[i]}
	}
	for i := range transports {
		for j := range transports {
			if i != j {
				transports[i].Connect(addresses[j], transports[j])
			}
		}
	}

	nodes := make([]*RaftNode, size)
	for i := 0; i < size; i++ {
		cfg := raft.DefaultConfig()
		cfg.LocalID = peers[i].ID
		cfg.HeartbeatTimeout = 80 * time.Millisecond
		cfg.ElectionTimeout = 80 * time.Millisecond
		cfg.LeaderLeaseTimeout = 50 * time.Millisecond
		cfg.CommitTimeout = 5 * time.Millisecond
		node, err := NewRaftNode(RaftNodeOptions{
			NodeID: string(peers[i].ID), BootstrapID: "node-1", Peers: peers,
			Transport: transports[i], LogStore: raft.NewInmemStore(), StableStore: raft.NewInmemStore(),
			SnapshotStore: raft.NewInmemSnapshotStore(), Config: cfg, ApplyTimeout: 300 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("new raft node %d: %v", i, err)
		}
		nodes[i] = node
	}
	t.Cleanup(func() {
		for _, node := range nodes {
			if node != nil {
				_ = node.Shutdown()
			}
		}
	})
	return nodes, transports
}

func waitForLeader(t *testing.T, nodes []*RaftNode) *RaftNode {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, node := range nodes {
			if node != nil && node.IsLeader() {
				return node
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("raft leader was not elected")
	return nil
}

func waitForState(t *testing.T, nodes []*RaftNode, predicate func(FSMSnapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, node := range nodes {
			if node != nil && !predicate(node.State()) {
				all = false
			}
		}
		if all {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("raft state did not converge")
}

func shutdownMember(t *testing.T, nodes []*RaftNode, transports []*raft.InmemTransport, index int) {
	t.Helper()
	address := transports[index].LocalAddr()
	if err := nodes[index].Shutdown(); err != nil {
		t.Fatalf("shutdown node %d: %v", index, err)
	}
	nodes[index] = nil
	for i, transport := range transports {
		if i != index {
			transport.Disconnect(address)
		}
	}
	transports[index].DisconnectAll()
}

type memorySnapshotSink struct {
	bytes.Buffer
	id string
}

func (s *memorySnapshotSink) ID() string    { return s.id }
func (s *memorySnapshotSink) Cancel() error { return nil }
func (s *memorySnapshotSink) Close() error  { return nil }
