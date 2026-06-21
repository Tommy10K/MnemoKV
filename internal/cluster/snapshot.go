package cluster

import "github.com/mnemokv/mnemokv/internal/snapshot"

func (m *Manager) SnapshotMetadata() snapshot.ClusterMetadata {
	if m == nil || m.metadata == nil {
		return snapshot.ClusterMetadata{}
	}
	state := m.metadata.Snapshot()
	meta := snapshot.ClusterMetadata{ClusterID: state.ClusterID, SlotCount: state.SlotCount, MetadataVersion: state.Version, Peers: make([]snapshot.Peer, len(state.Peers)), Slots: make([]snapshot.Slot, len(state.Slots))}
	for i, peer := range state.Peers {
		meta.Peers[i] = snapshot.Peer{ID: peer.ID, Address: peer.Address, APIAddress: peer.APIAddress}
	}
	for i, slot := range state.Slots {
		meta.Slots[i] = snapshot.Slot{
			Number: slot.Number, Role: m.metadata.LocalRole(slot), LeaderID: slot.LeaderID,
			ReplicaID: slot.ReplicaID, Term: slot.Term, LastSequence: slot.LastSequence,
			LastAppliedSequence: slot.LastAppliedSequence, ReplicaReady: slot.ReplicaReady,
		}
	}
	return meta
}
