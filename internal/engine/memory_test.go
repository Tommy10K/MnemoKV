package engine

import (
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestMemoryTrackerBasic(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 10000, EvictionPolicy: "noop"})
	mem := e.Memory()

	if mem.Used() != 0 {
		t.Fatalf("expected 0 used, got %d", mem.Used())
	}
	if !mem.HasLimit() {
		t.Fatal("expected limit to be set")
	}
	if mem.ExceedsLimit() {
		t.Fatal("should not exceed limit on empty store")
	}

	exec(t, e, "SET", "k", "somevalue")
	if mem.Used() == 0 {
		t.Fatal("expected non-zero usage after SET")
	}

	exec(t, e, "DEL", "k")
	if mem.Used() != 0 {
		t.Fatalf("expected 0 after DEL, got %d", mem.Used())
	}
}

func TestMemoryTrackerNoLimit(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 0, EvictionPolicy: "noop"})
	mem := e.Memory()
	if mem.HasLimit() {
		t.Fatal("expected no limit")
	}
	if mem.ExceedsLimit() {
		t.Fatal("should never exceed when limit is 0")
	}
}

func TestMemoryTrackerExceeds(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 1, EvictionPolicy: "noop"})
	mem := e.Memory()
	exec(t, e, "SET", "k", "this-is-definitely-more-than-1-byte")
	if !mem.ExceedsLimit() {
		t.Fatal("expected to exceed limit")
	}
}
