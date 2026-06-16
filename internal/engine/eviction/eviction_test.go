package eviction

import "testing"

func TestFIFO(t *testing.T) {
	candidates := []Candidate{
		{Key: "new", SizeBytes: 100, CreatedAtNs: 200},
		{Key: "old", SizeBytes: 100, CreatedAtNs: 100},
	}
	victims := FIFO{}.PickVictims(100, candidates)
	if len(victims) != 1 || victims[0].Key != "old" {
		t.Fatalf("expected old to be evicted, got %v", victims)
	}
}

func TestLRU(t *testing.T) {
	candidates := []Candidate{
		{Key: "recent", SizeBytes: 50, LastAccessNs: 300},
		{Key: "stale", SizeBytes: 50, LastAccessNs: 100},
	}
	victims := LRU{}.PickVictims(50, candidates)
	if len(victims) != 1 || victims[0].Key != "stale" {
		t.Fatalf("expected stale, got %v", victims)
	}
}

func TestLFU(t *testing.T) {
	candidates := []Candidate{
		{Key: "hot", SizeBytes: 50, AccessCount: 100},
		{Key: "cold", SizeBytes: 50, AccessCount: 1},
	}
	victims := LFU{}.PickVictims(50, candidates)
	if len(victims) != 1 || victims[0].Key != "cold" {
		t.Fatalf("expected cold, got %v", victims)
	}
}

func TestRandom(t *testing.T) {
	candidates := []Candidate{
		{Key: "a", SizeBytes: 50},
		{Key: "b", SizeBytes: 50},
	}
	victims := Random{}.PickVictims(50, candidates)
	if len(victims) != 1 {
		t.Fatalf("expected 1 victim, got %d", len(victims))
	}
}

func TestNoEviction(t *testing.T) {
	victims := NoEviction{}.PickVictims(100, []Candidate{{Key: "x", SizeBytes: 100}})
	if len(victims) != 0 {
		t.Fatal("noeviction should return no victims")
	}
}

func TestPolicyByName(t *testing.T) {
	cases := []struct {
		name     string
		expected string
	}{
		{"fifo", "fifo"},
		{"lru", "lru"},
		{"lfu", "lfu"},
		{"random", "random"},
		{"noeviction", "noeviction"},
		{"unknown", "noeviction"},
		{"", "noeviction"},
	}
	for _, tc := range cases {
		p := PolicyByName(tc.name)
		if p.Name() != tc.expected {
			t.Errorf("PolicyByName(%q) = %q, want %q", tc.name, p.Name(), tc.expected)
		}
	}
}
