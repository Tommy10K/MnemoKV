package engine

import "sync"

// Stripe is one shard of the global key dictionary. The dictionary as a whole
// is sharded into a fixed number of stripes and each stripe is guarded by its
// own RWMutex. This is the simplest design that lets the engine scale across
// many goroutines without devolving into a global lock.
type Stripe struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// newStripe builds an empty stripe. The initial map capacity is intentionally
// small; the Go runtime will grow it as needed.
func newStripe() *Stripe {
	return &Stripe{entries: make(map[string]*Entry, 64)}
}
