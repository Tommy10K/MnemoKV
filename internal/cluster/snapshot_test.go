package cluster

import (
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestSnapshotMetadataIncludesEverySlot(t *testing.T) {
	cfg := config.ClusterConfig{
		ID: "cluster-1", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true,
		Peers: []config.PeerConfig{{ID: "node-1", Address: "127.0.0.1:1"}, {ID: "node-2", Address: "127.0.0.1:2"}},
	}
	manager := NewManagerWithNode(cfg, "node-1")
	meta := manager.SnapshotMetadata()
	if meta.ClusterID != "cluster-1" || meta.SlotCount != currentSlotCount || len(meta.Slots) != int(currentSlotCount) {
		t.Fatalf("unexpected metadata: cluster=%q slots=%d entries=%d", meta.ClusterID, meta.SlotCount, len(meta.Slots))
	}
	for _, slot := range meta.Slots {
		if slot.Role != "leader" && slot.Role != "replica" {
			t.Fatalf("slot %d has role %q", slot.Number, slot.Role)
		}
	}
}
