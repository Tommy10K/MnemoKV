package failover_test

import (
	"errors"
	"testing"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
)

func TestStaleMetadataAndTermsAreRejected(t *testing.T) {
	cfg := config.ClusterConfig{
		ID: "failover", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true,
		SlotCount: 8, RoutingMode: "proxy", FailoverMode: "manual",
		Peers: []config.PeerConfig{{ID: "node-1", Address: "a", APIAddress: "A"}, {ID: "node-2", Address: "b", APIAddress: "B"}},
	}
	metadata := cluster.NewMetadata(cfg, "node-1")
	initial := metadata.Snapshot()
	if _, err := metadata.Promote(0); err != nil {
		t.Fatal(err)
	}
	if err := metadata.ApplyRemote(initial); !errors.Is(err, cluster.ErrStaleMetadata) {
		t.Fatalf("stale metadata error = %v", err)
	}
	state, _ := metadata.Slot(0)
	record := cluster.ReplicationRecord{SourceNodeID: initial.Slots[0].LeaderID, Slot: 0, Term: initial.Slots[0].Term, Sequence: 1}
	if _, err := metadata.ValidateReplication(record); !errors.Is(err, cluster.ErrStaleTerm) && !errors.Is(err, cluster.ErrNotReplica) {
		t.Fatalf("stale record error = %v; current=%+v", err, state)
	}
}
