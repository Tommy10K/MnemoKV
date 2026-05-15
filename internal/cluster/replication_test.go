package cluster

import (
	"context"
	"testing"
	"time"
)

func TestReplicationQueueOrdering(t *testing.T) {
	q := NewReplicationQueue()
	for i := 0; i < 5; i++ {
		q.Enqueue(ReplicationRecord{Sequence: q.NextSequence(), Args: []string{"SET", "k"}})
	}
	if q.Depth() != 5 {
		t.Fatalf("expected depth 5, got %d", q.Depth())
	}

	for i := uint64(1); i <= 5; i++ {
		rec, ok := q.Dequeue(context.Background())
		if !ok {
			t.Fatal("expected record")
		}
		if rec.Sequence != i {
			t.Fatalf("expected seq %d, got %d", i, rec.Sequence)
		}
	}
}

func TestReplicationQueueBlockingDequeue(t *testing.T) {
	q := NewReplicationQueue()
	go func() {
		time.Sleep(20 * time.Millisecond)
		q.Enqueue(ReplicationRecord{Sequence: q.NextSequence()})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rec, ok := q.Dequeue(ctx)
	if !ok || rec.Sequence != 1 {
		t.Fatalf("expected seq 1, got %+v ok=%v", rec, ok)
	}
}

func TestReplicationQueueClose(t *testing.T) {
	q := NewReplicationQueue()
	q.Close()
	if _, ok := q.Dequeue(context.Background()); ok {
		t.Fatal("expected closed dequeue to return false")
	}
}
