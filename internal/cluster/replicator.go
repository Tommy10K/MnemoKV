package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mnemokv/mnemokv/internal/resp"
)

// Replicator serializes leader writes and synchronously applies each record to
// exactly the replica assigned by the authoritative slot metadata.
type Replicator struct {
	nodeID    string
	metadata  *Metadata
	transport Transport
	mu        sync.Mutex
}

func NewReplicator(nodeID string, metadata *Metadata, transport Transport) *Replicator {
	return &Replicator{nodeID: nodeID, metadata: metadata, transport: transport}
}

func (r *Replicator) Mode() string { return "synchronous" }

func (r *Replicator) Replicate(ctx context.Context, cmd *resp.Command) error {
	key := resp.ExtractPrimaryKey(cmd)
	if len(key) == 0 {
		return nil
	}
	slot := r.metadata.SlotForKey(key)
	r.mu.Lock()
	defer r.mu.Unlock()
	state, sequence, err := r.metadata.PrepareReplication(slot)
	if err != nil {
		return err
	}
	rec := ReplicationRecord{
		SourceNodeID: r.nodeID, Slot: slot, Term: state.Term, Sequence: sequence,
		Args: commandToStrings(cmd),
	}
	if err := r.transport.SendReplication(ctx, state.ReplicaID, rec); err != nil {
		if errors.Is(err, ErrSequenceGap) || errors.Is(err, ErrStaleTerm) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrReplicaUnavailable, err)
	}
	return r.metadata.CommitLeaderSequence(slot, state.Term, sequence)
}

func (r *Replicator) AppliedSequence() uint64 {
	snapshot := r.metadata.Snapshot()
	var max uint64
	for _, slot := range snapshot.Slots {
		if slot.LastAppliedSequence > max {
			max = slot.LastAppliedSequence
		}
	}
	return max
}

func (r *Replicator) QueueDepth() int64 { return 0 }
