package engine

import (
	"hash/fnv"
	"sync/atomic"
)

// Store is the engine's striped key dictionary. It is concurrency-safe and is
// safe to share by pointer across goroutines. All operations are O(1) on
// average; range/scan operations are not exposed in the baseline.
type Store struct {
	stripes  []*Stripe
	mask     uint32 // stripeCount - 1 when stripeCount is a power of two
	powerTwo bool

	// usedBytes is updated on every create/update/delete to give the rest of
	// the engine an approximate memory accounting view. Future eviction work
	// will read it. We use atomic.Uint64 so reads from observability paths can
	// snapshot it without acquiring stripe locks.
	usedBytes atomic.Uint64
}

// NewStore builds a Store with the requested number of stripes. The count is
// rounded up to the next power of two when possible because that lets the
// stripe lookup use a bit mask instead of a modulo.
func NewStore(stripeCount int) *Store {
	if stripeCount < 1 {
		stripeCount = 1
	}
	target := nextPowerOfTwo(stripeCount)
	powerTwo := target == stripeCount
	if !powerTwo {
		// The caller asked for a non-power-of-two count; honor it literally so
		// configuration is predictable. The stripe lookup will fall back to a
		// modulo in that case.
		target = stripeCount
	}
	stripes := make([]*Stripe, target)
	for i := range stripes {
		stripes[i] = newStripe()
	}
	s := &Store{
		stripes:  stripes,
		powerTwo: powerTwo,
	}
	if powerTwo {
		s.mask = uint32(target - 1)
	}
	return s
}

// stripeFor returns the stripe that owns the given key.
func (s *Store) stripeFor(key []byte) *Stripe {
	h := fnv.New32a()
	_, _ = h.Write(key)
	v := h.Sum32()
	if s.powerTwo {
		return s.stripes[v&s.mask]
	}
	return s.stripes[v%uint32(len(s.stripes))]
}

// UsedBytes returns the engine's current approximate memory usage.
func (s *Store) UsedBytes() uint64 { return s.usedBytes.Load() }

// addUsed and subUsed wrap the atomic counter so the unfamiliar two's
// complement trick stays in one place.
func (s *Store) addUsed(n uint64) { s.usedBytes.Add(n) }
func (s *Store) subUsed(n uint64) { s.usedBytes.Add(^n + 1) }
func (s *Store) adjustUsed(oldSize, newSize uint64) {
	if newSize >= oldSize {
		s.addUsed(newSize - oldSize)
	} else {
		s.subUsed(oldSize - newSize)
	}
}

// Get returns a snapshot view of the entry. Lazy expiration: if the entry has
// expired, it is deleted in place and the call returns (nil, false).
func (s *Store) Get(key []byte) (*Entry, bool) {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock() // upgrade-friendly: we may need to delete on expiry
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if !ok {
		return nil, false
	}
	if e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		return nil, false
	}
	e.touchRead(now)
	return e, true
}

// Put inserts or replaces the entry under its key. The caller is responsible
// for filling in entry.Type, entry.Value, and entry.SizeBytes; this method
// fills in timestamps and version metadata.
func (s *Store) Put(entry *Entry) {
	st := s.stripeFor([]byte(entry.Key))
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()

	if existing, ok := st.entries[entry.Key]; ok {
		// Replace: subtract the old size, then add the new one.
		s.subUsed(existing.SizeBytes)
		entry.CreatedAtNs = existing.CreatedAtNs
		entry.AccessCount = existing.AccessCount
	} else {
		entry.CreatedAtNs = now
	}
	entry.UpdatedAtNs = now
	entry.LastAccessNs = now
	entry.Version++
	st.entries[entry.Key] = entry
	s.addUsed(entry.SizeBytes)
}

// Delete removes the entry under key, if any. It returns true if a key was
// removed.
func (s *Store) Delete(key []byte) bool {
	st := s.stripeFor(key)
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if !ok {
		return false
	}
	delete(st.entries, e.Key)
	s.subUsed(e.SizeBytes)
	return true
}

// Exists reports whether the key is present and unexpired. Lazy expiration is
// applied identically to Get.
func (s *Store) Exists(key []byte) bool {
	_, ok := s.Get(key)
	return ok
}

// Flush removes every entry from the store. It returns the number of entries
// removed.
func (s *Store) Flush() int {
	n := 0
	for _, st := range s.stripes {
		st.mu.Lock()
		for _, e := range st.entries {
			s.subUsed(e.SizeBytes)
		}
		n += len(st.entries)
		st.entries = make(map[string]*Entry, 64)
		st.mu.Unlock()
	}
	return n
}

// WithEntry runs fn with the entry under key while holding the stripe write
// lock. If the key does not exist, fn is called with nil. fn may mutate the
// entry in place; the store updates the size accounting based on the
// difference between entry.SizeBytes before and after the callback.
//
// fn must not call back into the store with the same key (it would deadlock).
func (s *Store) WithEntry(key []byte, fn func(e *Entry) error) error {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if ok && e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		ok = false
		e = nil
	}

	var sizeBefore uint64
	if ok {
		sizeBefore = e.SizeBytes
	}
	if err := fn(e); err != nil {
		return err
	}
	if ok && e.SizeBytes != sizeBefore {
		s.adjustUsed(sizeBefore, e.SizeBytes)
	}
	return nil
}

// IncrementBy atomically adds delta to the int64 value stored under key. The
// key is created with value delta if it does not exist. The mutation runs
// under the stripe write lock so two concurrent INCRs on the same key
// always observe each other.
func (s *Store) IncrementBy(key []byte, delta int64) (int64, error) {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if ok && e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		ok = false
		e = nil
	}

	var current int64
	if ok {
		if e.Type != ValueTypeString {
			return 0, ErrWrongType
		}
		sv, _ := e.Value.(*StringValue)
		if sv == nil {
			return 0, ErrWrongType
		}
		parsed, parseOk := parseInt64(sv.Data)
		if !parseOk {
			return 0, ErrNotInteger
		}
		current = parsed
	}

	if (delta > 0 && current > maxInt64-delta) || (delta < 0 && current < minInt64-delta) {
		return 0, ErrIntOverflow
	}
	next := current + delta
	newBytes := formatInt64(next)
	newSize := stringEntrySize(key, newBytes)

	if ok {
		oldSize := e.SizeBytes
		e.Value = NewStringValue(newBytes)
		e.SizeBytes = newSize
		e.touchWrite(now)
		s.adjustUsed(oldSize, newSize)
		return next, nil
	}

	ne := &Entry{
		Key:          string(key),
		Type:         ValueTypeString,
		Value:        NewStringValue(newBytes),
		SizeBytes:    newSize,
		CreatedAtNs:  now,
		UpdatedAtNs:  now,
		LastAccessNs: now,
		AccessCount:  1,
		Version:      1,
	}
	st.entries[ne.Key] = ne
	s.addUsed(newSize)
	return next, nil
}

// SetExpireAt assigns an absolute expiration to the entry under key, if any.
// Returns true if the key existed and the TTL was applied.
func (s *Store) SetExpireAt(key []byte, expiresAtNs int64) bool {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()
	e, ok := st.entries[string(key)]
	if !ok {
		return false
	}
	if e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		return false
	}
	e.ExpiresAtNs = expiresAtNs
	return true
}

const (
	maxInt64 = int64(^uint64(0) >> 1)
	minInt64 = -maxInt64 - 1
)

// nextPowerOfTwo returns the smallest power of two >= n. n is assumed
// positive.
func nextPowerOfTwo(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
