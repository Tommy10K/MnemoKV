package eviction

import "sync"

const defaultSampleSize = 16

type Store interface {
	Sample(n int) []Candidate
	Delete(key []byte) bool
}

type Manager struct {
	mu     sync.Mutex
	policy Policy
	store  Store
}

func NewManager(store Store, policy Policy) *Manager {
	return &Manager{store: store, policy: policy}
}

func (m *Manager) Policy() Policy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.policy
}

func (m *Manager) SetPolicy(p Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = p
}

func (m *Manager) Run(bytesNeeded uint64) int {
	m.mu.Lock()
	policy := m.policy
	m.mu.Unlock()

	candidates := m.store.Sample(defaultSampleSize)
	if len(candidates) == 0 {
		return 0
	}
	victims := policy.PickVictims(bytesNeeded, candidates)
	evicted := 0
	for _, v := range victims {
		if m.store.Delete([]byte(v.Key)) {
			evicted++
		}
	}
	return evicted
}
