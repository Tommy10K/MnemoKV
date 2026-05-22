package cluster

import (
	"context"
	"sync"
	"time"
)

type ControlPlane struct {
	mu           sync.RWMutex
	term         Term
	leaderBySlot map[uint16]string
	localNodeID  string
	membership   *Membership
	transport    Transport
	ring         *Ring
	electing     map[uint16]bool
	onChange     func(slot uint16, leader string, term uint64)
}

type ControlPlaneConfig struct {
	LocalNodeID string
	Membership  *Membership
	Transport   Transport
	Ring        *Ring
}

func NewControlPlane(cfg ControlPlaneConfig) *ControlPlane {
	return &ControlPlane{
		localNodeID:  cfg.LocalNodeID,
		membership:   cfg.Membership,
		transport:    cfg.Transport,
		ring:         cfg.Ring,
		leaderBySlot: make(map[uint16]string),
		electing:     make(map[uint16]bool),
	}
}

func (cp *ControlPlane) SetOnChange(fn func(slot uint16, leader string, term uint64)) {
	cp.mu.Lock()
	cp.onChange = fn
	cp.mu.Unlock()
}

func (cp *ControlPlane) CurrentTerm() uint64 {
	return cp.term.Current()
}

func (cp *ControlPlane) LeaderForSlot(slot uint16) (string, uint64, bool) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	leader, ok := cp.leaderBySlot[slot]
	return leader, cp.term.Current(), ok
}

func (cp *ControlPlane) IsLeader(slot uint16) bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.leaderBySlot[slot] == cp.localNodeID
}

func (cp *ControlPlane) ApplyLeader(slot uint16, leaderID string, term uint64) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	current := cp.term.Current()
	if term < current {
		return ErrStaleTerm
	}
	if term > current {
		cp.term.Set(term)
	}
	cp.leaderBySlot[slot] = leaderID
	cp.electing[slot] = false
	if cp.onChange != nil {
		cp.onChange(slot, leaderID, term)
	}
	return nil
}

func (cp *ControlPlane) BeginElection(ctx context.Context, slot uint16) (string, error) {
	cp.mu.Lock()
	if cp.electing[slot] {
		cp.mu.Unlock()
		return "", ErrElectionInProgress
	}
	cp.electing[slot] = true
	cp.mu.Unlock()

	defer func() {
		cp.mu.Lock()
		cp.electing[slot] = false
		cp.mu.Unlock()
	}()

	winner := cp.electLeader(slot)
	if winner == "" {
		return "", ErrNoCandidate
	}

	newTerm := cp.term.Advance()
	if err := cp.ApplyLeader(slot, winner, newTerm); err != nil {
		return "", err
	}
	cp.broadcastLeaderChange(ctx, slot, winner, newTerm)
	return winner, nil
}

func (cp *ControlPlane) electLeader(_ uint16) string {
	if cp.membership == nil {
		return cp.localNodeID
	}
	peers := cp.membership.Snapshot()
	for _, p := range peers {
		if p.State == StateHealthy {
			return p.ID
		}
	}
	return cp.localNodeID
}

func (cp *ControlPlane) broadcastLeaderChange(ctx context.Context, slot uint16, leader string, term uint64) {
	if cp.membership == nil || cp.transport == nil {
		return
	}
	peers := cp.membership.Peers()
	for _, peerID := range peers {
		if peerID == cp.localNodeID {
			continue
		}
		rec := ReplicationRecord{
			SourceNodeID: cp.localNodeID,
			Slot:         slot,
			Term:         term,
			Args:         []string{"LEADER_CHANGE", leader},
			Timestamp:    time.Now(),
		}
		_ = cp.transport.SendReplication(ctx, peerID, rec)
	}
}

func (cp *ControlPlane) ValidateWriteTerm(slot uint16, term uint64) error {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	current := cp.term.Current()
	if term < current {
		return ErrStaleTerm
	}
	leader, ok := cp.leaderBySlot[slot]
	if ok && leader != cp.localNodeID {
		return ErrNotLeader
	}
	return nil
}

func (cp *ControlPlane) SeedSlots(nodeID string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	for slot := uint16(0); slot < 16384; slot++ {
		if _, exists := cp.leaderBySlot[slot]; !exists {
			if cp.ring != nil {
				owner := cp.ring.Owner([]byte{byte(slot >> 8), byte(slot)})
				if owner != "" {
					cp.leaderBySlot[slot] = owner
					continue
				}
			}
			cp.leaderBySlot[slot] = nodeID
		}
	}
}

func (cp *ControlPlane) Leaders() map[uint16]string {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	out := make(map[uint16]string, len(cp.leaderBySlot))
	for k, v := range cp.leaderBySlot {
		out[k] = v
	}
	return out
}
