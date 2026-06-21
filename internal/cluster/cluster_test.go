package cluster

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type failingTransport struct{ err error }

func (f failingTransport) Forward(context.Context, string, *resp.Command) (resp.Frame, error) {
	return nil, f.err
}
func (f failingTransport) SendReplication(context.Context, string, ReplicationRecord) error {
	return f.err
}
func (f failingTransport) Close() error { return nil }

func metadataTestConfig(peers []config.PeerConfig) config.ClusterConfig {
	return config.ClusterConfig{
		ID: "test-cluster", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true,
		SlotCount: 16, RoutingMode: "proxy", FailoverMode: "manual", Peers: peers,
	}
}

func TestEveryNodeComputesIdenticalSlotMap(t *testing.T) {
	peersA := []config.PeerConfig{
		{ID: "node-3", Address: "c", APIAddress: "C"},
		{ID: "node-1", Address: "a", APIAddress: "A"},
		{ID: "node-2", Address: "b", APIAddress: "B"},
	}
	peersB := []config.PeerConfig{peersA[1], peersA[2], peersA[0]}
	a := NewMetadata(metadataTestConfig(peersA), "node-1").Snapshot()
	b := NewMetadata(metadataTestConfig(peersB), "node-2").Snapshot()
	for i := range a.Slots {
		a.Slots[i].LastAppliedSequence = 0
		b.Slots[i].LastAppliedSequence = 0
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("slot maps differ\nA=%+v\nB=%+v", a, b)
	}
	for _, slot := range a.Slots {
		if slot.LeaderID == slot.ReplicaID || slot.ReplicaID == "" || slot.Term != 1 || !slot.ReplicaReady {
			t.Fatalf("invalid initial slot: %+v", slot)
		}
	}
}

func TestRouterUsesMetadataLeader(t *testing.T) {
	peers := []config.PeerConfig{{ID: "node-1", Address: "a", APIAddress: "A"}, {ID: "node-2", Address: "b", APIAddress: "B"}}
	metadata := NewMetadata(metadataTestConfig(peers), "node-1")
	router := NewRouter("node-1", metadata)
	for i := 0; i < 100; i++ {
		key := []byte{byte(i)}
		route := router.Resolve(key)
		state, ok := metadata.Slot(route.Slot)
		if !ok || route.OwnerNodeID != state.LeaderID || route.IsLocal != (state.LeaderID == "node-1") {
			t.Fatalf("route mismatch: route=%+v state=%+v", route, state)
		}
	}
}

func TestManualPromotionAndReplicaAssignmentAdvanceTerm(t *testing.T) {
	peers := []config.PeerConfig{{ID: "node-1", Address: "a", APIAddress: "A"}, {ID: "node-2", Address: "b", APIAddress: "B"}, {ID: "node-3", Address: "c", APIAddress: "C"}}
	metadata := NewMetadata(metadataTestConfig(peers), "node-1")
	before, _ := metadata.Slot(0)
	promoted, err := metadata.Promote(0)
	if err != nil {
		t.Fatal(err)
	}
	after := promoted.Slots[0]
	if after.LeaderID != before.ReplicaID || after.Term != before.Term+1 || after.ReplicaID != "" || after.ReplicaReady {
		t.Fatalf("unexpected promotion: before=%+v after=%+v", before, after)
	}
	assigned, err := metadata.AssignReplica(0, "node-3")
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Slots[0].ReplicaID != "node-3" || assigned.Slots[0].Term != after.Term+1 || assigned.Slots[0].ReplicaReady {
		t.Fatalf("unexpected assignment: %+v", assigned.Slots[0])
	}
}

func TestReplicatorPreservesReplicaGapForRepair(t *testing.T) {
	peers := []config.PeerConfig{{ID: "node-1", Address: "a", APIAddress: "A"}, {ID: "node-2", Address: "b", APIAddress: "B"}}
	metadata := NewMetadata(metadataTestConfig(peers), "node-1")
	replicator := NewReplicator("node-1", metadata, failingTransport{err: ErrSequenceGap})
	var key []byte
	for i := 0; i < 1000; i++ {
		candidate := []byte{byte(i >> 8), byte(i)}
		slot := metadata.SlotForKey(candidate)
		state, _ := metadata.Slot(slot)
		if state.LeaderID == "node-1" {
			key = candidate
			break
		}
	}
	if err := replicator.Replicate(context.Background(), &resp.Command{Name: "SET", Args: [][]byte{key, []byte("v")}}); !errors.Is(err, ErrSequenceGap) {
		t.Fatalf("replication error = %v", err)
	}
}
