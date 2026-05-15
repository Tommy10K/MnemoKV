package cluster

import (
	"context"
	"testing"
	"time"
)

func TestControlPlaneApplyLeader(t *testing.T) {
	cp := NewControlPlane(ControlPlaneConfig{LocalNodeID: "node-1"})

	if err := cp.ApplyLeader(0, "node-1", 1); err != nil {
		t.Fatalf("apply: %v", err)
	}
	leader, term, ok := cp.LeaderForSlot(0)
	if !ok || leader != "node-1" || term != 1 {
		t.Fatalf("got leader=%s term=%d ok=%v", leader, term, ok)
	}
}

func TestControlPlaneRejectsStaleTerm(t *testing.T) {
	cp := NewControlPlane(ControlPlaneConfig{LocalNodeID: "node-1"})
	_ = cp.ApplyLeader(0, "node-1", 5)

	err := cp.ApplyLeader(0, "node-2", 3)
	if err != ErrStaleTerm {
		t.Fatalf("expected ErrStaleTerm, got %v", err)
	}
}

func TestControlPlaneValidateWriteTerm(t *testing.T) {
	cp := NewControlPlane(ControlPlaneConfig{LocalNodeID: "node-1"})
	_ = cp.ApplyLeader(0, "node-1", 2)

	if err := cp.ValidateWriteTerm(0, 2); err != nil {
		t.Fatalf("valid write rejected: %v", err)
	}
	if err := cp.ValidateWriteTerm(0, 1); err != ErrStaleTerm {
		t.Fatalf("stale write accepted: %v", err)
	}
}

func TestControlPlaneNotLeader(t *testing.T) {
	cp := NewControlPlane(ControlPlaneConfig{LocalNodeID: "node-1"})
	_ = cp.ApplyLeader(0, "node-2", 1)

	if err := cp.ValidateWriteTerm(0, 1); err != ErrNotLeader {
		t.Fatalf("expected ErrNotLeader, got %v", err)
	}
}

func TestElectionTriggersOnUnavailable(t *testing.T) {
	nodes := []Node{{ID: "node-1", Address: "a"}, {ID: "node-2", Address: "b"}}
	m := NewMembership("node-1", nodes, time.Second, 2*time.Second)
	cp := NewControlPlane(ControlPlaneConfig{
		LocalNodeID: "node-1",
		Membership:  m,
	})
	_ = cp.ApplyLeader(10, "node-2", 1)
	m.MarkFailed("node-2")
	m.MarkFailed("node-2")

	e := NewElection(cp, m, "node-1", 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	e.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	e.Shutdown()

	leader, term, ok := cp.LeaderForSlot(10)
	if !ok {
		t.Fatal("no leader after election")
	}
	if term <= 1 {
		t.Fatalf("term not advanced: %d", term)
	}
	if leader == "node-2" {
		t.Fatal("unavailable node still leader")
	}
}

func TestBeginElectionConcurrent(t *testing.T) {
	cp := NewControlPlane(ControlPlaneConfig{LocalNodeID: "node-1"})
	_ = cp.ApplyLeader(0, "node-1", 1)

	ctx := context.Background()
	_, err := cp.BeginElection(ctx, 0)
	if err != nil {
		t.Fatalf("first election: %v", err)
	}
}
