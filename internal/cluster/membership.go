package cluster

import (
	"sync"
	"time"
)

const (
	StateHealthy     = "healthy"
	StateSuspect     = "suspect"
	StateUnavailable = "unavailable"
	StateRecovering  = "recovering"
)

type peerEntry struct {
	id       string
	address  string
	state    string
	lastSeen time.Time
}

type Membership struct {
	mu      sync.RWMutex
	self    string
	peers   map[string]*peerEntry
	suspect time.Duration
	dead    time.Duration
}

func NewMembership(self string, peers []Node, suspect, dead time.Duration) *Membership {
	if suspect <= 0 {
		suspect = 3 * time.Second
	}
	if dead <= 0 {
		dead = 10 * time.Second
	}
	m := &Membership{
		self:    self,
		peers:   make(map[string]*peerEntry, len(peers)),
		suspect: suspect,
		dead:    dead,
	}
	for _, p := range peers {
		if p.ID == self {
			continue
		}
		m.peers[p.ID] = &peerEntry{
			id:       p.ID,
			address:  p.Address,
			state:    StateHealthy,
			lastSeen: time.Now(),
		}
	}
	return m
}

func (m *Membership) MarkAlive(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.peers[id]; ok {
		p.state = StateHealthy
		p.lastSeen = time.Now()
	}
}

func (m *Membership) MarkFailed(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.peers[id]; ok {
		switch p.state {
		case StateHealthy:
			p.state = StateSuspect
		case StateSuspect:
			p.state = StateUnavailable
		}
	}
}

func (m *Membership) Tick(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.peers {
		gap := now.Sub(p.lastSeen)
		switch {
		case gap >= m.dead && p.state != StateUnavailable:
			p.state = StateUnavailable
		case gap >= m.suspect && p.state == StateHealthy:
			p.state = StateSuspect
		}
	}
}

func (m *Membership) Snapshot() []MemberInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]MemberInfo, 0, len(m.peers))
	for _, p := range m.peers {
		out = append(out, MemberInfo{ID: p.id, Address: p.address, State: p.state})
	}
	return out
}

func (m *Membership) Peers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.peers))
	for id := range m.peers {
		out = append(out, id)
	}
	return out
}
