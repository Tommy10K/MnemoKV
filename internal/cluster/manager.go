// Package cluster owns fixed-slot routing, synchronous replication,
// membership hints, manual failover, and full-slot repair.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

type Manager struct {
	cfg         config.ClusterConfig
	nodeID      string
	metadata    *Metadata
	router      *Router
	proxy       *RESPProxy
	replicator  *Replicator
	membership  *Membership
	gossip      *Gossip
	engine      *engine.Engine
	coordinator *Coordinator
	applyMu     sync.Mutex
}

func NewManager(cfg config.ClusterConfig) *Manager { return NewManagerWithNode(cfg, "") }

func NewManagerWithNode(cfg config.ClusterConfig, nodeID string) *Manager {
	m := &Manager{cfg: cfg, nodeID: nodeID}
	if !cfg.Enabled {
		return m
	}
	m.metadata = NewMetadata(cfg, nodeID)
	m.router = NewRouter(nodeID, m.metadata)
	nodes := make([]Node, 0, len(cfg.Peers))
	peerAddrs := make(map[string]string, len(cfg.Peers))
	for _, peer := range cfg.Peers {
		nodes = append(nodes, Node{ID: peer.ID, Address: peer.Address})
		if peer.ID != nodeID {
			peerAddrs[peer.ID] = peer.Address
		}
	}
	m.proxy = NewRESPProxy(peerAddrs, 2*time.Second)
	m.membership = NewMembership(nodeID, nodes, 3*time.Second, 10*time.Second)
	m.gossip = NewGossip(m.membership, m.proxy, time.Second)
	if cfg.ReplicationEnabled {
		m.replicator = NewReplicator(nodeID, m.metadata, m.proxy)
	}
	return m
}

func (m *Manager) Enabled() bool                { return m != nil && m.cfg.Enabled }
func (m *Manager) Router() *Router              { return m.router }
func (m *Manager) Metadata() *Metadata          { return m.metadata }
func (m *Manager) Replicator() *Replicator      { return m.replicator }
func (m *Manager) MembershipTable() *Membership { return m.membership }
func (m *Manager) Coordinator() *Coordinator    { return m.coordinator }

func (m *Manager) Proxy() Transport {
	if m.proxy == nil {
		return nil
	}
	return m.proxy
}

func (m *Manager) Start(ctx context.Context) error {
	if !m.Enabled() {
		return nil
	}
	m.syncMetadata(ctx)
	if m.gossip != nil {
		m.gossip.Start(ctx)
	}
	return nil
}

func (m *Manager) Shutdown(context.Context) error {
	if m.gossip != nil {
		m.gossip.Shutdown()
	}
	if m.proxy != nil {
		return m.proxy.Close()
	}
	return nil
}

func (m *Manager) ApplyReplication(rec ReplicationRecord) error {
	if m.engine == nil || m.metadata == nil {
		return errors.New("cluster engine is not attached")
	}
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	duplicate, err := m.metadata.ValidateReplication(rec)
	if err != nil {
		return err
	}
	if duplicate {
		return nil
	}
	if len(rec.Args) == 0 {
		return errors.New("replication record has no command")
	}
	cmd := &resp.Command{Name: rec.Args[0], Args: make([][]byte, len(rec.Args)-1)}
	for i := 1; i < len(rec.Args); i++ {
		cmd.Args[i-1] = []byte(rec.Args[i])
	}
	if _, isErr := m.engine.ApplyReplicated(cmd).(resp.Error); isErr {
		return errors.New("replicated command failed")
	}
	return m.metadata.CommitReplicaSequence(rec.Slot, rec.Term, rec.Sequence)
}

func (m *Manager) syncMetadata(ctx context.Context) {
	if m.proxy == nil || m.metadata == nil {
		return
	}
	for _, peer := range m.cfg.Peers {
		if peer.ID == m.nodeID {
			continue
		}
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		frame, err := m.proxy.Forward(requestCtx, peer.ID, &resp.Command{Name: "CLUSTERMETA"})
		cancel()
		if err != nil {
			continue
		}
		bulk, ok := frame.(resp.BulkString)
		if !ok || bulk.Null {
			continue
		}
		var incoming MetadataSnapshot
		if json.Unmarshal(bulk.Value, &incoming) == nil && incoming.Version > m.metadata.Snapshot().Version {
			_ = m.metadata.ApplyRemote(incoming)
		}
	}
}

func (m *Manager) ApplyMetadata(in MetadataSnapshot) error {
	if m.metadata == nil {
		return ErrClusterMismatch
	}
	return m.metadata.ApplyRemote(in)
}

func (m *Manager) BroadcastMetadata(ctx context.Context, state MetadataSnapshot) []string {
	raw, _ := json.Marshal(state)
	failed := make([]string, 0)
	for _, peer := range m.cfg.Peers {
		if peer.ID == m.nodeID {
			continue
		}
		frame, err := m.proxy.Forward(ctx, peer.ID, &resp.Command{Name: "CLUSTERAPPLY", Args: [][]byte{raw}})
		if err != nil {
			failed = append(failed, peer.ID)
			continue
		}
		if _, isErr := frame.(resp.Error); isErr {
			failed = append(failed, peer.ID)
		}
	}
	return failed
}

func (m *Manager) Promote(ctx context.Context, slot uint32) (MetadataSnapshot, []string, error) {
	state, err := m.metadata.Promote(slot)
	if err != nil {
		return MetadataSnapshot{}, nil, err
	}
	return state, m.BroadcastMetadata(ctx, state), nil
}

func (m *Manager) AssignReplica(ctx context.Context, slot uint32, nodeID string) (MetadataSnapshot, []string, error) {
	state, err := m.metadata.AssignReplica(slot, nodeID)
	if err != nil {
		return MetadataSnapshot{}, nil, err
	}
	return state, m.BroadcastMetadata(ctx, state), nil
}

func (m *Manager) MetadataJSON() ([]byte, error) {
	if m.metadata == nil {
		return nil, ErrClusterMismatch
	}
	return json.Marshal(m.metadata.Snapshot())
}

func (m *Manager) RestoreMetadata(meta snapshot.ClusterMetadata) error {
	if m.metadata == nil || meta.ClusterID == "" {
		return nil
	}
	return m.metadata.RestoreSnapshot(meta)
}

func clusterFrameError(err error) resp.Frame {
	switch {
	case errors.Is(err, ErrNotLeader), errors.Is(err, ErrReplicaUnavailable):
		return resp.NewError("CLUSTERDOWN", err.Error())
	case errors.Is(err, ErrStaleTerm), errors.Is(err, ErrSequenceGap):
		return resp.NewError("TRYAGAIN", err.Error())
	default:
		return resp.NewError("ERR", err.Error())
	}
}

func (m *Manager) String() string {
	if m.metadata == nil {
		return "cluster disabled"
	}
	state := m.metadata.Snapshot()
	return fmt.Sprintf("cluster=%s version=%d slots=%d", state.ClusterID, state.Version, state.SlotCount)
}
