package eviction

import "sync"

const defaultSampleSize = 16

type Store interface {
	Sample(n int) []Candidate
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

func (m *Manager) PickVictims(bytesNeeded uint64, excludeKey string) []Candidate {
	m.mu.Lock()
	policy := m.policy
	m.mu.Unlock()

	candidates := m.store.Sample(defaultSampleSize)
	if len(candidates) == 0 {
		return nil
	}

	if excludeKey != "" {
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if candidate.Key != excludeKey {
				filtered = append(filtered, candidate)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return nil
	}
	return policy.PickVictims(bytesNeeded, candidates)
}
