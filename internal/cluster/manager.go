// Package cluster owns routing, replication, membership, and the control
// plane.
package cluster

import (
	"context"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

type Manager struct {
	cfg    config.ClusterConfig
	nodeID string
	ring   *Ring
	router *Router
	proxy  *RESPProxy
}

func NewManager(cfg config.ClusterConfig) *Manager {
	return NewManagerWithNode(cfg, "")
}

func NewManagerWithNode(cfg config.ClusterConfig, nodeID string) *Manager {
	m := &Manager{cfg: cfg, nodeID: nodeID}
	if cfg.Enabled {
		nodes := make([]Node, 0, len(cfg.Peers))
		peerAddrs := make(map[string]string, len(cfg.Peers))
		for _, p := range cfg.Peers {
			nodes = append(nodes, Node{ID: p.ID, Address: p.Address})
			if p.ID != nodeID {
				peerAddrs[p.ID] = p.Address
			}
		}
		m.ring = NewRing(nodes, defaultVirtualNodes)
		m.router = NewRouter(nodeID, m.ring)
		m.proxy = NewRESPProxy(peerAddrs, 2*time.Second)
	}
	return m
}

func (m *Manager) Router() *Router { return m.router }

func (m *Manager) Ring() *Ring { return m.ring }

func (m *Manager) Proxy() Transport {
	if m.proxy == nil {
		return nil
	}
	return m.proxy
}

func (m *Manager) Start(ctx context.Context) error { return nil }

func (m *Manager) Shutdown(ctx context.Context) error {
	if m.proxy != nil {
		return m.proxy.Close()
	}
	return nil
}
