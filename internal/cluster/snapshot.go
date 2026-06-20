package cluster

import "github.com/mnemokv/mnemokv/internal/snapshot"

const currentSlotCount uint32 = 16384

// SnapshotMetadata exposes the cluster state available in the current
// prototype without implying that the later fixed-slot redesign is complete.
func (m *Manager) SnapshotMetadata() snapshot.ClusterMetadata {
	if m == nil || !m.cfg.Enabled {
		return snapshot.ClusterMetadata{}
	}
	meta := snapshot.ClusterMetadata{ClusterID: m.cfg.ID, SlotCount: currentSlotCount}
	var (
		term        uint64
		leaders     map[uint16]string
		lastApplied uint64
	)
	if m.controlPlane != nil {
		term = m.controlPlane.CurrentTerm()
		meta.MetadataVersion = term
		leaders = m.controlPlane.Leaders()
	}
	if m.replicator != nil {
		lastApplied = m.replicator.AppliedSequence()
	}
	meta.Slots = make([]snapshot.Slot, 0, currentSlotCount)
	for slot := uint32(0); slot < currentSlotCount; slot++ {
		leader := leaders[uint16(slot)]
		if leader == "" && m.ring != nil {
			leader = m.ring.Owner([]byte{byte(slot >> 8), byte(slot)})
		}
		role := "none"
		if leader == m.nodeID {
			role = "leader"
		} else if m.cfg.ReplicationEnabled {
			// The current prototype fans writes out to every peer, so each
			// non-leader peer acts as a replica for persisted-state purposes.
			role = "replica"
		}
		meta.Slots = append(meta.Slots, snapshot.Slot{
			Number: slot, Role: role, Term: term, LastAppliedSequence: lastApplied,
		})
	}
	return meta
}
