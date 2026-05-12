package engine

import "time"

// Entry is the value side of one stored key. Every entry carries the metadata
// the eviction policies and observability layer need, so policies do not have
// to maintain parallel maps keyed by string.
//
// Entry is mutated in place under the owning stripe's lock. Fields are simple
// values rather than atomics on purpose: contention is already controlled by
// the stripe mutex, and atomics would only add cost without buying us
// concurrency we do not have.
type Entry struct {
	Key          string
	Type         ValueType
	Value        any
	SizeBytes    uint64
	ExpiresAtNs  int64 // unix nanos; 0 means "no expiration"
	CreatedAtNs  int64
	UpdatedAtNs  int64
	LastAccessNs int64
	AccessCount  uint32
	Version      uint64
}

// IsExpired reports whether the entry has a TTL that already elapsed at
// time nowNs. Passing nowNs in lets tests use a fake clock without touching
// time.Now().
func (e *Entry) IsExpired(nowNs int64) bool {
	return e.ExpiresAtNs != 0 && nowNs >= e.ExpiresAtNs
}

// touchRead bumps the access metadata on a read. Caller holds the stripe lock.
func (e *Entry) touchRead(nowNs int64) {
	e.LastAccessNs = nowNs
	e.AccessCount++
}

// touchWrite bumps the access metadata on a write. Caller holds the stripe
// lock.
func (e *Entry) touchWrite(nowNs int64) {
	e.UpdatedAtNs = nowNs
	e.LastAccessNs = nowNs
	e.AccessCount++
	e.Version++
}

// nowNanos is a small indirection so tests can replace time.Now() if they
// need to. The default just calls time.Now().
var nowNanos = func() int64 { return time.Now().UnixNano() }
