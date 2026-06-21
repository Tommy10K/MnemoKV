package cluster

import (
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestSnapshotMetadataIncludesAuthoritativeSlotState(t *testing.T) {
	cfg := metadataTestConfig([]config.PeerConfig{{ID: "node-1", Address: "a", APIAddress: "A"}, {ID: "node-2", Address: "b", APIAddress: "B"}})
	manager := NewManagerWithNode(cfg, "node-1")
	meta := manager.SnapshotMetadata()
	if meta.ClusterID != cfg.ID || meta.SlotCount != cfg.SlotCount || meta.MetadataVersion != 1 || len(meta.Peers) != 2 || len(meta.Slots) != int(cfg.SlotCount) {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	for _, slot := range meta.Slots {
		if slot.LeaderID == "" || slot.ReplicaID == "" || slot.Term != 1 || !slot.ReplicaReady {
			t.Fatalf("incomplete slot metadata: %+v", slot)
		}
	}
}
