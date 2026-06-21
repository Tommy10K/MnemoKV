package cluster

import (
	"fmt"
	"hash/fnv"
	"sort"
	"sync"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

const (
	RoleLeader  = "leader"
	RoleReplica = "replica"
	RoleNone    = "none"
)

// SlotState is the authoritative ownership and replication state for one slot.
type SlotState struct {
	Number              uint32 `json:"number"`
	LeaderID            string `json:"leaderId"`
	ReplicaID           string `json:"replicaId,omitempty"`
	Term                uint64 `json:"term"`
	LastSequence        uint64 `json:"lastSequence"`
	LastAppliedSequence uint64 `json:"lastAppliedSequence"`
	ReplicaReady        bool   `json:"replicaReady"`
}

type MetadataSnapshot struct {
	ClusterID string         `json:"clusterId"`
	Version   uint64         `json:"version"`
	SlotCount uint32         `json:"slotCount"`
	Peers     []MetadataPeer `json:"peers"`
	Slots     []SlotState    `json:"slots"`
}

type MetadataPeer struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	APIAddress string `json:"apiAddress"`
}

// Metadata owns the single fixed-slot map used for routing, fencing,
// replication, failover, observability, and persistence.
type Metadata struct {
	mu        sync.RWMutex
	clusterID string
	version   uint64
	slotCount uint32
	localID   string
	peers     []MetadataPeer
	peerIDs   map[string]struct{}
	slots     []SlotState
}

func NewMetadata(cfg config.ClusterConfig, localID string) *Metadata {
	m := &Metadata{clusterID: cfg.ID, version: 1, slotCount: cfg.SlotCount, localID: localID, peerIDs: make(map[string]struct{}, len(cfg.Peers))}
	if !cfg.Enabled || cfg.SlotCount == 0 {
		return m
	}
	peerIDs := make([]string, 0, len(cfg.Peers))
	for _, peer := range cfg.Peers {
		peerIDs = append(peerIDs, peer.ID)
		m.peers = append(m.peers, MetadataPeer{ID: peer.ID, Address: peer.Address, APIAddress: peer.APIAddress})
		m.peerIDs[peer.ID] = struct{}{}
	}
	if len(peerIDs) == 0 {
		return m
	}
	sort.Strings(peerIDs)
	sort.Slice(m.peers, func(i, j int) bool { return m.peers[i].ID < m.peers[j].ID })
	m.slots = make([]SlotState, cfg.SlotCount)
	base := cfg.SlotCount / uint32(len(peerIDs))
	remainder := cfg.SlotCount % uint32(len(peerIDs))
	var slot uint32
	for ownerIndex, leader := range peerIDs {
		count := base
		if uint32(ownerIndex) < remainder {
			count++
		}
		for i := uint32(0); i < count; i++ {
			state := SlotState{Number: slot, LeaderID: leader, Term: 1}
			if cfg.ReplicationEnabled {
				state.ReplicaID = peerIDs[(ownerIndex+1)%len(peerIDs)]
				state.ReplicaReady = true
			}
			m.slots[slot] = state
			slot++
		}
	}
	return m
}

func (m *Metadata) SlotForKey(key []byte) uint32 {
	m.mu.RLock()
	count := m.slotCount
	m.mu.RUnlock()
	if count == 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write(key)
	return h.Sum32() % count
}

func (m *Metadata) Slot(number uint32) (SlotState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if number >= uint32(len(m.slots)) {
		return SlotState{}, false
	}
	return m.slots[number], true
}

func (m *Metadata) Snapshot() MetadataSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetadataSnapshot{ClusterID: m.clusterID, Version: m.version, SlotCount: m.slotCount, Peers: append([]MetadataPeer(nil), m.peers...), Slots: append([]SlotState(nil), m.slots...)}
}

func (m *Metadata) LocalRole(slot SlotState) string {
	switch m.localID {
	case slot.LeaderID:
		return RoleLeader
	case slot.ReplicaID:
		return RoleReplica
	default:
		return RoleNone
	}
}

func (m *Metadata) PrepareReplication(number uint32) (SlotState, uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if number >= uint32(len(m.slots)) {
		return SlotState{}, 0, ErrSlotOutOfRange
	}
	state := m.slots[number]
	if state.LeaderID != m.localID {
		return SlotState{}, 0, ErrNotLeader
	}
	if state.ReplicaID == "" || !state.ReplicaReady {
		return SlotState{}, 0, ErrReplicaUnavailable
	}
	return state, state.LastSequence + 1, nil
}

func (m *Metadata) CommitLeaderSequence(number uint32, term, sequence uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return ErrSlotOutOfRange
	}
	state := &m.slots[number]
	if state.LeaderID != m.localID || state.Term != term {
		return ErrStaleTerm
	}
	if sequence != state.LastSequence+1 {
		return ErrSequenceGap
	}
	state.LastSequence = sequence
	state.LastAppliedSequence = sequence
	return nil
}

func (m *Metadata) ValidateReplication(rec ReplicationRecord) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rec.Slot >= uint32(len(m.slots)) {
		return false, ErrSlotOutOfRange
	}
	state := m.slots[rec.Slot]
	if state.ReplicaID != m.localID {
		return false, ErrNotReplica
	}
	if rec.Term != state.Term || rec.SourceNodeID != state.LeaderID {
		return false, ErrStaleTerm
	}
	if rec.Sequence <= state.LastAppliedSequence {
		return true, nil
	}
	if rec.Sequence != state.LastAppliedSequence+1 {
		return false, ErrSequenceGap
	}
	return false, nil
}

func (m *Metadata) CommitReplicaSequence(number uint32, term, sequence uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return ErrSlotOutOfRange
	}
	state := &m.slots[number]
	if state.ReplicaID != m.localID || state.Term != term {
		return ErrStaleTerm
	}
	if sequence != state.LastAppliedSequence+1 {
		return ErrSequenceGap
	}
	state.LastAppliedSequence = sequence
	if sequence > state.LastSequence {
		state.LastSequence = sequence
	}
	return nil
}

func (m *Metadata) Promote(number uint32) (MetadataSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return MetadataSnapshot{}, ErrSlotOutOfRange
	}
	state := &m.slots[number]
	if state.ReplicaID == "" {
		return MetadataSnapshot{}, ErrNoReplicaAssigned
	}
	state.LeaderID = state.ReplicaID
	state.ReplicaID = ""
	state.ReplicaReady = false
	state.Term++
	state.LastSequence = 0
	state.LastAppliedSequence = 0
	m.version++
	return m.snapshotLocked(), nil
}

func (m *Metadata) AssignReplica(number uint32, nodeID string) (MetadataSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return MetadataSnapshot{}, ErrSlotOutOfRange
	}
	if _, ok := m.peerIDs[nodeID]; !ok {
		return MetadataSnapshot{}, fmt.Errorf("%w: %s", ErrUnknownNode, nodeID)
	}
	state := &m.slots[number]
	if nodeID == state.LeaderID {
		return MetadataSnapshot{}, ErrReplicaIsLeader
	}
	state.ReplicaID = nodeID
	state.ReplicaReady = false
	state.Term++
	state.LastSequence = 0
	state.LastAppliedSequence = 0
	m.version++
	return m.snapshotLocked(), nil
}

func (m *Metadata) MarkReplicaReady(number uint32, term, sequence uint64) (MetadataSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return MetadataSnapshot{}, ErrSlotOutOfRange
	}
	state := &m.slots[number]
	if state.LeaderID != m.localID || state.Term != term || state.ReplicaID == "" {
		return MetadataSnapshot{}, ErrStaleTerm
	}
	state.ReplicaReady = true
	state.LastSequence = sequence
	state.LastAppliedSequence = sequence
	m.version++
	return m.snapshotLocked(), nil
}

func (m *Metadata) ApplyRemote(in MetadataSnapshot) error {
	return m.apply(in, false)
}

func (m *Metadata) Restore(in MetadataSnapshot) error {
	return m.apply(in, true)
}

func (m *Metadata) apply(in MetadataSnapshot, allowEqual bool) error {
	if err := validateMetadataSnapshot(in); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if in.ClusterID != m.clusterID || in.SlotCount != m.slotCount {
		return ErrClusterMismatch
	}
	if !equalMetadataPeers(in.Peers, m.peers) {
		return ErrClusterMismatch
	}
	if in.Version < m.version || (!allowEqual && in.Version == m.version) {
		return ErrStaleMetadata
	}
	current := m.slots
	next := append([]SlotState(nil), in.Slots...)
	for i := range next {
		if !allowEqual && i < len(current) && current[i].Term == next[i].Term {
			next[i].LastAppliedSequence = current[i].LastAppliedSequence
		}
	}
	m.version = in.Version
	m.slots = next
	return nil
}

func (m *Metadata) RestoreSnapshot(in snapshot.ClusterMetadata) error {
	if in.ClusterID == "" {
		return nil
	}
	// Version-1 snapshots written before the fixed-slot metadata model contain
	// only local roles. Their dataset is still restorable, but ownership must be
	// rebuilt from config and refreshed from peers because leader/replica IDs
	// cannot be reconstructed safely.
	for _, slot := range in.Slots {
		if slot.LeaderID == "" {
			return nil
		}
	}
	slots := make([]SlotState, len(in.Slots))
	for i, slot := range in.Slots {
		slots[i] = SlotState{
			Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID,
			Term: slot.Term, LastSequence: slot.LastSequence,
			LastAppliedSequence: slot.LastAppliedSequence, ReplicaReady: slot.ReplicaReady,
		}
	}
	peers := make([]MetadataPeer, len(in.Peers))
	for i, peer := range in.Peers {
		peers[i] = MetadataPeer{ID: peer.ID, Address: peer.Address, APIAddress: peer.APIAddress}
	}
	if len(peers) == 0 {
		peers = append(peers, m.peers...)
	}
	return m.Restore(MetadataSnapshot{ClusterID: in.ClusterID, Version: in.MetadataVersion, SlotCount: in.SlotCount, Peers: peers, Slots: slots})
}

func (m *Metadata) SetReplicaApplied(number uint32, term, sequence uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if number >= uint32(len(m.slots)) {
		return ErrSlotOutOfRange
	}
	state := &m.slots[number]
	if state.Term != term || state.ReplicaID != m.localID {
		return ErrStaleTerm
	}
	state.LastAppliedSequence = sequence
	state.LastSequence = sequence
	return nil
}

func (m *Metadata) snapshotLocked() MetadataSnapshot {
	return MetadataSnapshot{ClusterID: m.clusterID, Version: m.version, SlotCount: m.slotCount, Peers: append([]MetadataPeer(nil), m.peers...), Slots: append([]SlotState(nil), m.slots...)}
}

func validateMetadataSnapshot(in MetadataSnapshot) error {
	if in.ClusterID == "" || in.SlotCount == 0 || len(in.Peers) < 2 || len(in.Peers) > 5 || len(in.Slots) != int(in.SlotCount) {
		return ErrClusterMismatch
	}
	for i, peer := range in.Peers {
		if peer.ID == "" || peer.Address == "" || peer.APIAddress == "" || (i > 0 && in.Peers[i-1].ID >= peer.ID) {
			return ErrClusterMismatch
		}
	}
	for i, slot := range in.Slots {
		if slot.Number != uint32(i) || slot.LeaderID == "" || slot.Term == 0 {
			return ErrClusterMismatch
		}
	}
	return nil
}

func equalMetadataPeers(a, b []MetadataPeer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
