// Package metrics exposes counters, gauges, latency observations, and bounded
// events through a small sink abstraction.
package metrics

import "time"

// Sink is the contract every metrics implementation must satisfy. Higher
// layers receive a Sink rather than a concrete type so the eventual Prometheus
// or in-memory backend can be swapped in transparently.
type Sink interface {
	IncCounter(name string, labels ...string)
	AddCounter(name string, delta uint64, labels ...string)
	ObserveLatency(name string, value time.Duration, labels ...string)
	Gauge(name string, value float64, labels ...string)
}

// Noop discards every observation and is useful in focused tests.
type Noop struct{}

// NewNoop returns a Noop sink.
func NewNoop() Noop { return Noop{} }

func (Noop) IncCounter(string, ...string) {}

func (Noop) AddCounter(string, uint64, ...string) {}

func (Noop) ObserveLatency(string, time.Duration, ...string) {}

func (Noop) Gauge(string, float64, ...string) {}
