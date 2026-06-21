package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

type ShardSnapshot struct {
	SourceNodeID string           `json:"sourceNodeId"`
	Slot         uint32           `json:"slot"`
	Term         uint64           `json:"term"`
	Sequence     uint64           `json:"sequence"`
	Entries      []snapshot.Entry `json:"entries"`
}

func (m *Manager) SyncReplica(ctx context.Context, slot uint32, target string) (MetadataSnapshot, []string, error) {
	if m.engine == nil || m.metadata == nil {
		return MetadataSnapshot{}, nil, errors.New("cluster engine is not attached")
	}
	state, ok := m.metadata.Slot(slot)
	if !ok {
		return MetadataSnapshot{}, nil, ErrSlotOutOfRange
	}
	if state.LeaderID != m.nodeID {
		return MetadataSnapshot{}, nil, ErrNotLeader
	}
	if target == "" {
		target = state.ReplicaID
	}
	if target == "" || target != state.ReplicaID {
		return MetadataSnapshot{}, nil, ErrNotReplica
	}

	allEntries, err := m.engine.SnapshotEntries()
	if err != nil {
		return MetadataSnapshot{}, nil, err
	}
	entries := make([]snapshot.Entry, 0)
	for _, entry := range allEntries {
		if m.metadata.SlotForKey([]byte(entry.Key)) == slot {
			entries = append(entries, entry)
		}
	}
	transfer := ShardSnapshot{SourceNodeID: m.nodeID, Slot: slot, Term: state.Term, Sequence: state.LastSequence, Entries: entries}
	raw, err := json.Marshal(transfer)
	if err != nil {
		return MetadataSnapshot{}, nil, err
	}
	frame, err := m.proxy.Forward(ctx, target, &resp.Command{Name: "CLUSTERSNAPSHOT", Args: [][]byte{raw}})
	if err != nil {
		return MetadataSnapshot{}, nil, ErrReplicaUnavailable
	}
	if _, isErr := frame.(resp.Error); isErr {
		return MetadataSnapshot{}, nil, ErrReplicaUnavailable
	}
	updated, err := m.metadata.MarkReplicaReady(slot, state.Term, state.LastSequence)
	if err != nil {
		return MetadataSnapshot{}, nil, err
	}
	return updated, m.BroadcastMetadata(ctx, updated), nil
}

func (m *Manager) ApplyShardSnapshot(transfer ShardSnapshot) error {
	if m.engine == nil || m.metadata == nil {
		return errors.New("cluster engine is not attached")
	}
	state, ok := m.metadata.Slot(transfer.Slot)
	if !ok {
		return ErrSlotOutOfRange
	}
	if state.ReplicaID != m.nodeID || transfer.SourceNodeID != state.LeaderID || transfer.Term != state.Term {
		return ErrStaleTerm
	}
	for _, entry := range transfer.Entries {
		if m.metadata.SlotForKey([]byte(entry.Key)) != transfer.Slot {
			return ErrClusterMismatch
		}
	}

	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	current, err := m.engine.SnapshotEntries()
	if err != nil {
		return err
	}
	combined := make([]snapshot.Entry, 0, len(current)+len(transfer.Entries))
	for _, entry := range current {
		if m.metadata.SlotForKey([]byte(entry.Key)) != transfer.Slot {
			combined = append(combined, entry)
		}
	}
	combined = append(combined, transfer.Entries...)
	if _, err := m.engine.RestoreSnapshotEntries(combined, time.Now()); err != nil {
		return err
	}
	return m.metadata.SetReplicaApplied(transfer.Slot, transfer.Term, transfer.Sequence)
}
