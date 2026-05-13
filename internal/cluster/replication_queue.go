package cluster

import (
	"context"
	"sync"
	"sync/atomic"
)

type ReplicationQueue struct {
	mu     sync.Mutex
	items  []ReplicationRecord
	signal chan struct{}
	seq    atomic.Uint64
	depth  atomic.Int64
	closed bool
}

func NewReplicationQueue() *ReplicationQueue {
	return &ReplicationQueue{signal: make(chan struct{}, 1)}
}

func (q *ReplicationQueue) NextSequence() uint64 {
	return q.seq.Add(1)
}

func (q *ReplicationQueue) Enqueue(rec ReplicationRecord) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.items = append(q.items, rec)
	q.depth.Store(int64(len(q.items)))
	q.mu.Unlock()

	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *ReplicationQueue) Dequeue(ctx context.Context) (ReplicationRecord, bool) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			rec := q.items[0]
			q.items = q.items[1:]
			q.depth.Store(int64(len(q.items)))
			q.mu.Unlock()
			return rec, true
		}
		if q.closed {
			q.mu.Unlock()
			return ReplicationRecord{}, false
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return ReplicationRecord{}, false
		case _, ok := <-q.signal:
			if !ok {
				return ReplicationRecord{}, false
			}
		}
	}
}

func (q *ReplicationQueue) Depth() int64 {
	return q.depth.Load()
}

func (q *ReplicationQueue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.mu.Unlock()
	close(q.signal)
}
