package engine

import "sync/atomic"

type MemoryTracker struct {
	usedBytes  *atomic.Uint64
	limitBytes uint64
}

func NewMemoryTracker(store *Store, limitBytes uint64) *MemoryTracker {
	return &MemoryTracker{
		usedBytes:  &store.usedBytes,
		limitBytes: limitBytes,
	}
}

func (m *MemoryTracker) Used() uint64 {
	return m.usedBytes.Load()
}

func (m *MemoryTracker) Limit() uint64 {
	return m.limitBytes
}

func (m *MemoryTracker) HasLimit() bool {
	return m.limitBytes > 0
}

func (m *MemoryTracker) ExceedsLimit() bool {
	if m.limitBytes == 0 {
		return false
	}
	return m.usedBytes.Load() > m.limitBytes
}

func (m *MemoryTracker) Available() uint64 {
	if m.limitBytes == 0 {
		return ^uint64(0)
	}
	used := m.usedBytes.Load()
	if used >= m.limitBytes {
		return 0
	}
	return m.limitBytes - used
}

func (m *MemoryTracker) UsageRatio() float64 {
	if m.limitBytes == 0 {
		return 0
	}
	return float64(m.usedBytes.Load()) / float64(m.limitBytes)
}
