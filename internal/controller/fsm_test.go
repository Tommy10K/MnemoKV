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
