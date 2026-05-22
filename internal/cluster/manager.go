// Package cluster owns routing, replication, membership, and the control
// plane.
package cluster

import (
	"context"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

type Manager struct {
	cfg          config.ClusterConfig
	nodeID       string
	ring         *Ring
	router       *Router
	proxy        *RESPProxy
	queue        *ReplicationQueue
	replicator   *Replicator
	membership   *Membership
	gossip       *Gossip
	controlPlane *ControlPlane
	election     *Election
}

func NewManager(cfg config.ClusterConfig) *Manager {
	return NewManagerWithNode(cfg, "")
}

func NewManagerWithNode(cfg config.ClusterConfig, nodeID string) *Manager {
	m := &Manager{cfg: cfg, nodeID: nodeID}
	if !cfg.Enabled {
		return m
	}

	nodes := make([]Node, 0, len(cfg.Peers))
	peerAddrs := make(map[string]string, len(cfg.Peers))
	followers := make([]string, 0, len(cfg.Peers))
	for _, p := range cfg.Peers {
		nodes = append(nodes, Node{ID: p.ID, Address: p.Address})
		if p.ID != nodeID {
			peerAddrs[p.ID] = p.Address
			followers = append(followers, p.ID)
		}
	}
	m.ring = NewRing(nodes, defaultVirtualNodes)
	m.router = NewRouter(nodeID, m.ring)
	m.proxy = NewRESPProxy(peerAddrs, 2*time.Second)
	m.membership = NewMembership(nodeID, nodes, 3*time.Second, 10*time.Second)
	m.gossip = NewGossip(m.membership, m.proxy, time.Second)

	if cfg.AutoFailover {
		m.controlPlane = NewControlPlane(ControlPlaneConfig{
			LocalNodeID: nodeID,
			Membership:  m.membership,
			Transport:   m.proxy,
			Ring:        m.ring,
		})
		m.controlPlane.SeedSlots(nodeID)
		m.election = NewElection(m.controlPlane, m.membership, nodeID, 2*time.Second)
	}

	if cfg.ReplicationEnabled {
		m.queue = NewReplicationQueue()
		m.replicator = NewReplicator(cfg.WriteSafetyMode, nodeID, followers, m.proxy, m.queue)
	}
	return m
}

func (m *Manager) Router() *Router { return m.router }

func (m *Manager) Ring() *Ring { return m.ring }

func (m *Manager) Replicator() *Replicator { return m.replicator }

func (m *Manager) MembershipTable() *Membership { return m.membership }

func (m *Manager) ControlPlane() *ControlPlane { return m.controlPlane }

func (m *Manager) Proxy() Transport {
	if m.proxy == nil {
		return nil
	}
	return m.proxy
}

func (m *Manager) Start(ctx context.Context) error {
	if m.replicator != nil {
		m.replicator.Start(ctx)
	}
	if m.gossip != nil {
		m.gossip.Start(ctx)
	}
	if m.election != nil {
		m.election.Start(ctx)
	}
	return nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	if m.election != nil {
		m.election.Shutdown()
	}
	if m.gossip != nil {
		m.gossip.Shutdown()
	}
	if m.replicator != nil {
		m.replicator.Shutdown()
	}
	if m.proxy != nil {
		return m.proxy.Close()
	}
	return nil
}
