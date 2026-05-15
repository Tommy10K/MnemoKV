package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/resp"
)

type Gossip struct {
	membership *Membership
	transport  Transport
	interval   time.Duration

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewGossip(m *Membership, t Transport, interval time.Duration) *Gossip {
	if interval <= 0 {
		interval = time.Second
	}
	return &Gossip{membership: m, transport: t, interval: interval}
}

func (g *Gossip) Start(ctx context.Context) {
	if g.membership == nil || g.transport == nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.wg.Add(1)
	go g.loop(ctx)
}

func (g *Gossip) Shutdown() {
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()
}

func (g *Gossip) loop(ctx context.Context) {
	defer g.wg.Done()
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.probe(ctx)
			g.membership.Tick(time.Now())
		}
	}
}

func (g *Gossip) probe(ctx context.Context) {
	for _, id := range g.membership.Peers() {
		probeCtx, cancel := context.WithTimeout(ctx, g.interval)
		_, err := g.transport.Forward(probeCtx, id, &resp.Command{Name: "PING"})
		cancel()
		if err != nil {
			g.membership.MarkFailed(id)
			continue
		}
		g.membership.MarkAlive(id)
	}
}
