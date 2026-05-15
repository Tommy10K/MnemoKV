package cluster

import (
	"context"
	"sync"
	"time"
)

type Election struct {
	cp         *ControlPlane
	membership *Membership
	interval   time.Duration
	localNode  string

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewElection(cp *ControlPlane, membership *Membership, localNode string, checkInterval time.Duration) *Election {
	if checkInterval <= 0 {
		checkInterval = 2 * time.Second
	}
	return &Election{
		cp:         cp,
		membership: membership,
		localNode:  localNode,
		interval:   checkInterval,
	}
}

func (e *Election) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.wg.Add(1)
	go e.monitor(ctx)
}

func (e *Election) Shutdown() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

func (e *Election) monitor(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.check(ctx)
		}
	}
}

func (e *Election) check(ctx context.Context) {
	if e.membership == nil || e.cp == nil {
		return
	}
	unavailable := make(map[string]bool)
	for _, p := range e.membership.Snapshot() {
		if p.State == StateUnavailable {
			unavailable[p.ID] = true
		}
	}
	if len(unavailable) == 0 {
		return
	}
	leaders := e.cp.Leaders()
	for slot, leader := range leaders {
		if unavailable[leader] {
			_, _ = e.cp.BeginElection(ctx, slot)
		}
	}
}

func (e *Election) TriggerElection(ctx context.Context, slot uint16) (string, error) {
	return e.cp.BeginElection(ctx, slot)
}
