package cluster

import (
	"context"
	"sync"
)

type Replicator struct {
	mode      string
	transport Transport
	queue     *ReplicationQueue
	followers []string
	nodeID    string

	wg     sync.WaitGroup
	cancel context.CancelFunc

	mu      sync.Mutex
	applied uint64
}

func NewReplicator(mode, nodeID string, followers []string, transport Transport, queue *ReplicationQueue) *Replicator {
	return &Replicator{
		mode:      mode,
		nodeID:    nodeID,
		followers: followers,
		transport: transport,
		queue:     queue,
	}
}

func (r *Replicator) Mode() string { return r.mode }

func (r *Replicator) Start(ctx context.Context) {
	if len(r.followers) == 0 || r.transport == nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.wg.Add(1)
	go r.drainLoop(ctx)
}

func (r *Replicator) Shutdown() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.queue != nil {
		r.queue.Close()
	}
	r.wg.Wait()
}

func (r *Replicator) Replicate(args []string, slot uint16) ReplicationRecord {
	rec := ReplicationRecord{
		SourceNodeID: r.nodeID,
		Slot:         slot,
		Sequence:     r.queue.NextSequence(),
		Args:         args,
	}
	r.queue.Enqueue(rec)
	return rec
}

func (r *Replicator) ReplicateSync(ctx context.Context, args []string, slot uint16) error {
	rec := ReplicationRecord{
		SourceNodeID: r.nodeID,
		Slot:         slot,
		Sequence:     r.queue.NextSequence(),
		Args:         args,
	}
	return r.fanout(ctx, rec)
}

func (r *Replicator) drainLoop(ctx context.Context) {
	defer r.wg.Done()
	for {
		rec, ok := r.queue.Dequeue(ctx)
		if !ok {
			return
		}
		_ = r.fanout(ctx, rec)
	}
}

func (r *Replicator) fanout(ctx context.Context, rec ReplicationRecord) error {
	var lastErr error
	for _, follower := range r.followers {
		if err := r.transport.SendReplication(ctx, follower, rec); err != nil {
			lastErr = err
			continue
		}
		r.markApplied(rec.Sequence)
	}
	return lastErr
}

func (r *Replicator) markApplied(seq uint64) {
	r.mu.Lock()
	if seq > r.applied {
		r.applied = seq
	}
	r.mu.Unlock()
}

func (r *Replicator) AppliedSequence() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.applied
}

func (r *Replicator) QueueDepth() int64 {
	if r.queue == nil {
		return 0
	}
	return r.queue.Depth()
}
