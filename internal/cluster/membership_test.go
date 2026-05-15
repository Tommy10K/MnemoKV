package cluster

import (
	"testing"
	"time"
)

func TestMembershipStateTransitions(t *testing.T) {
	nodes := []Node{{ID: "self"}, {ID: "a"}, {ID: "b"}}
	m := NewMembership("self", nodes, 100*time.Millisecond, 300*time.Millisecond)

	snap := m.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 peers (excluding self), got %d", len(snap))
	}
	for _, p := range snap {
		if p.State != StateHealthy {
			t.Errorf("expected %s healthy, got %s", p.ID, p.State)
		}
	}

	m.MarkFailed("a")
	if state := stateOf(m, "a"); state != StateSuspect {
		t.Errorf("expected suspect after one failure, got %s", state)
	}
	m.MarkFailed("a")
	if state := stateOf(m, "a"); state != StateUnavailable {
		t.Errorf("expected unavailable after two failures, got %s", state)
	}

	m.MarkAlive("a")
	if state := stateOf(m, "a"); state != StateHealthy {
		t.Errorf("expected healthy after revival, got %s", state)
	}
}

func TestMembershipTickFlagsStaleness(t *testing.T) {
	nodes := []Node{{ID: "self"}, {ID: "a"}}
	m := NewMembership("self", nodes, 50*time.Millisecond, 200*time.Millisecond)

	m.Tick(time.Now().Add(100 * time.Millisecond))
	if state := stateOf(m, "a"); state != StateSuspect {
		t.Errorf("expected suspect after stale gap, got %s", state)
	}
	m.Tick(time.Now().Add(500 * time.Millisecond))
	if state := stateOf(m, "a"); state != StateUnavailable {
		t.Errorf("expected unavailable after dead gap, got %s", state)
	}
}

func stateOf(m *Membership, id string) string {
	for _, p := range m.Snapshot() {
		if p.ID == id {
			return p.State
		}
	}
	return ""
}
