package metrics

import (
	"sync"
	"time"
)

type InMemory struct {
	mu        sync.Mutex
	counters  map[string]uint64
	gauges    map[string]float64
	latencies map[string][]time.Duration
	events    []Event
	maxEvents int
}

func NewInMemory(maxEvents int) *InMemory {
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	return &InMemory{
		counters:  make(map[string]uint64),
		gauges:    make(map[string]float64),
		latencies: make(map[string][]time.Duration),
		maxEvents: maxEvents,
	}
}

func (m *InMemory) IncCounter(name string, _ ...string) {
	m.mu.Lock()
	m.counters[name]++
	m.mu.Unlock()
}

func (m *InMemory) ObserveLatency(name string, value time.Duration, _ ...string) {
	m.mu.Lock()
	m.latencies[name] = append(m.latencies[name], value)
	m.mu.Unlock()
}

func (m *InMemory) Gauge(name string, value float64, _ ...string) {
	m.mu.Lock()
	m.gauges[name] = value
	m.mu.Unlock()
}

func (m *InMemory) PublishEvent(event Event) {
	m.mu.Lock()
	if len(m.events) >= m.maxEvents {
		m.events = m.events[1:]
	}
	m.events = append(m.events, event)
	m.mu.Unlock()
}

func (m *InMemory) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

func (m *InMemory) Counter(name string) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[name]
}

func (m *InMemory) GaugeValue(name string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gauges[name]
}

func (m *InMemory) Latencies(name string) []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]time.Duration, len(m.latencies[name]))
	copy(out, m.latencies[name])
	return out
}

func (m *InMemory) Snapshot() map[string]uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]uint64, len(m.counters))
	for k, v := range m.counters {
		out[k] = v
	}
	return out
}
