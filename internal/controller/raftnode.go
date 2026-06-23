package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/mnemokv/mnemokv/internal/config"
)

type RaftPeer struct {
	ID      raft.ServerID
	Address raft.ServerAddress
}

type RaftNodeOptions struct {
	NodeID        string
	BootstrapID   string
	BindAddress   string
	RaftDir       string
	Peers         []RaftPeer
	Transport     raft.Transport
	LogStore      raft.LogStore
	StableStore   raft.StableStore
	SnapshotStore raft.SnapshotStore
	Config        *raft.Config
	ApplyTimeout  time.Duration
}

type RaftNode struct {
	raft         *raft.Raft
	fsm          *FSM
	transport    raft.Transport
	applyTimeout time.Duration
	closers      []io.Closer
}

func NewRaftNodeFromConfig(cfg config.ClusterConfig, nodeID string) (*RaftNode, error) {
	peers := make([]RaftPeer, 0, len(cfg.Peers))
	bindAddress := ""
	for _, peer := range cfg.Peers {
		peers = append(peers, RaftPeer{ID: raft.ServerID(peer.ID), Address: raft.ServerAddress(peer.ControlAddress)})
		if peer.ID == nodeID {
			bindAddress = peer.ControlAddress
		}
	}
	if bindAddress == "" {
		return nil, fmt.Errorf("controller peer %q has no control address", nodeID)
	}
	return NewRaftNode(RaftNodeOptions{
		NodeID: nodeID, BootstrapID: cfg.Controller.BootstrapNodeID,
		BindAddress: bindAddress, RaftDir: cfg.Controller.RaftDir, Peers: peers,
	})
}

func NewRaftNode(options RaftNodeOptions) (*RaftNode, error) {
	if options.NodeID == "" || len(options.Peers) == 0 {
		return nil, errors.New("raft node id and peers are required")
	}
	if options.ApplyTimeout <= 0 {
		options.ApplyTimeout = 5 * time.Second
	}

	transport := options.Transport
	var closers []io.Closer
	if transport == nil {
		if err := os.MkdirAll(options.RaftDir, 0o700); err != nil {
			return nil, fmt.Errorf("create raft dir: %w", err)
		}
		tcp, err := raft.NewTCPTransport(options.BindAddress, nil, 3, 10*time.Second, os.Stderr)
		if err != nil {
			return nil, fmt.Errorf("create raft transport: %w", err)
		}
		transport = tcp
		closers = append(closers, tcp)
	}

	logStore, stableStore := options.LogStore, options.StableStore
	if logStore == nil || stableStore == nil {
		store, err := raftboltdb.NewBoltStore(filepath.Join(options.RaftDir, "raft.db"))
		if err != nil {
			closeAll(closers)
			return nil, fmt.Errorf("open raft store: %w", err)
		}
		logStore, stableStore = store, store
		closers = append(closers, store)
	}
	snapshotStore := options.SnapshotStore
	if snapshotStore == nil {
		store, err := raft.NewFileSnapshotStore(options.RaftDir, 2, os.Stderr)
		if err != nil {
			closeAll(closers)
			return nil, fmt.Errorf("open raft snapshot store: %w", err)
		}
		snapshotStore = store
	}
	hasState, err := raft.HasExistingState(logStore, stableStore, snapshotStore)
	if err != nil {
		closeAll(closers)
		return nil, fmt.Errorf("inspect raft state: %w", err)
	}

	raftConfig := options.Config
	if raftConfig == nil {
		raftConfig = raft.DefaultConfig()
	}
	raftConfig.LocalID = raft.ServerID(options.NodeID)
	fsm := NewFSM()
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		closeAll(closers)
		return nil, fmt.Errorf("create raft: %w", err)
	}

	node := &RaftNode{raft: r, fsm: fsm, transport: transport, applyTimeout: options.ApplyTimeout, closers: closers}
	if !hasState && options.NodeID == options.BootstrapID {
		servers := make([]raft.Server, len(options.Peers))
		for i, peer := range options.Peers {
			servers[i] = raft.Server{Suffrage: raft.Voter, ID: peer.ID, Address: peer.Address}
		}
		if err := r.BootstrapCluster(raft.Configuration{Servers: servers}).Error(); err != nil && !errors.Is(err, raft.ErrCantBootstrap) {
			_ = node.Shutdown()
			return nil, fmt.Errorf("bootstrap raft cluster: %w", err)
		}
	}
	return node, nil
}

func (n *RaftNode) Propose(command Command) error {
	if !n.IsLeader() {
		return raft.ErrNotLeader
	}
	raw, err := json.Marshal(command)
	if err != nil {
		return err
	}
	future := n.raft.Apply(raw, n.applyTimeout)
	if err := future.Error(); err != nil {
		return err
	}
	if applyErr, ok := future.Response().(error); ok {
		return applyErr
	}
	return nil
}

func (n *RaftNode) IsLeader() bool        { return n.raft.State() == raft.Leader }
func (n *RaftNode) LeaderCh() <-chan bool { return n.raft.LeaderCh() }
func (n *RaftNode) State() FSMSnapshot    { return n.fsm.State() }
func (n *RaftNode) Raft() *raft.Raft      { return n.raft }

func (n *RaftNode) Shutdown() error {
	var first error
	if n.raft != nil {
		first = n.raft.Shutdown().Error()
	}
	for _, closer := range n.closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func closeAll(closers []io.Closer) {
	for _, closer := range closers {
		_ = closer.Close()
	}
}
