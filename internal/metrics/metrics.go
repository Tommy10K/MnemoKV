// Package metrics will expose counters, gauges, latency histograms, and
// structured events to the rest of the backend in later phases. The baseline
// milestone only needs the Sink interface and a no-op implementation so other
// packages can take it as a dependency.
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

// Noop discards every observation. It is the default sink for the baseline
// milestone.
type Noop struct{}

// NewNoop returns a Noop sink.
func NewNoop() Noop { return Noop{} }

func (Noop) IncCounter(string, ...string) {}

func (Noop) AddCounter(string, uint64, ...string) {}

func (Noop) ObserveLatency(string, time.Duration, ...string) {}

func (Noop) Gauge(string, float64, ...string) {}
