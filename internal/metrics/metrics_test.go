package metrics

import (
	"testing"
	"time"
)

func TestInMemoryCounters(t *testing.T) {
	m := NewInMemory(100)
	m.IncCounter("cmd.total")
	m.IncCounter("cmd.total")
	if m.Counter("cmd.total") != 2 {
		t.Fatalf("expected 2, got %d", m.Counter("cmd.total"))
	}
}

func TestInMemoryGauge(t *testing.T) {
	m := NewInMemory(100)
	m.Gauge("memory.used", 1024)
	if m.GaugeValue("memory.used") != 1024 {
		t.Fatal("gauge mismatch")
	}
}

func TestInMemoryLatency(t *testing.T) {
	m := NewInMemory(100)
	m.ObserveLatency("cmd.set", 5*time.Millisecond)
	lats := m.Latencies("cmd.set")
	if len(lats) != 1 || lats[0] != 5*time.Millisecond {
		t.Fatal("latency mismatch")
	}
}

func TestInMemoryEvents(t *testing.T) {
	m := NewInMemory(2)
	m.PublishEvent(Event{Name: "a"})
	m.PublishEvent(Event{Name: "b"})
	m.PublishEvent(Event{Name: "c"})
	events := m.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "b" || events[1].Name != "c" {
		t.Fatalf("unexpected events: %v", events)
	}
}
