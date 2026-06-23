package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

type viewProposer interface {
	IsLeader() bool
	Propose(Command) error
	State() FSMSnapshot
}

type failureRecord struct {
	consecutive int
	firstMiss   time.Time
	failed      bool
}

type Observer struct {
	peers         []config.PeerConfig
	clients       map[string]NodeAPI
	proposer      viewProposer
	interval      time.Duration
	failureAfter  time.Duration
	failureCount  int
	skewThreshold int
	now           func() time.Time

	mu       sync.Mutex
	failures map[string]failureRecord
}

func NewObserver(cfg config.ClusterConfig, clients map[string]NodeAPI, proposer viewProposer) *Observer {
	skewThreshold := cfg.Controller.RebalanceSkewThreshold
	if skewThreshold <= 0 {
		skewThreshold = 1
	}
	return &Observer{
		peers: append([]config.PeerConfig(nil), cfg.Peers...), clients: clients, proposer: proposer,
		interval:     time.Duration(cfg.Controller.ObserveIntervalMs) * time.Millisecond,
		failureAfter: time.Duration(cfg.Controller.FailureTimeoutMs) * time.Millisecond,
		failureCount: cfg.Controller.ConsecutiveFailures, now: time.Now,
		skewThreshold: skewThreshold,
		failures:      make(map[string]failureRecord, len(cfg.Peers)),
	}
}

func NewObserverFromConfig(cfg config.ClusterConfig, proposer viewProposer) (*Observer, error) {
	clients := make(map[string]NodeAPI, len(cfg.Peers))
	timeout := time.Duration(cfg.Controller.ObserveIntervalMs) * time.Millisecond
	if timeout <= 0 || timeout > time.Second {
		timeout = time.Second
	}
	for _, peer := range cfg.Peers {
		client, err := NewNodeClient(peer.APIAddress, timeout)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", peer.ID, err)
		}
		clients[peer.ID] = client
	}
	return NewObserver(cfg, clients, proposer), nil
}

func (o *Observer) Run(ctx context.Context) {
	interval := o.interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if o.proposer != nil && o.proposer.IsLeader() {
				_, _ = o.ObserveAndCommit(ctx)
			}
		}
	}
}

func (o *Observer) ObserveAndCommit(ctx context.Context) (ClusterView, error) {
	view, err := o.PollOnce(ctx)
	if err != nil {
		return ClusterView{}, err
	}
	if o.proposer == nil || !o.proposer.IsLeader() {
		return view, nil
	}
	if materiallyEqualViews(o.proposer.State().LatestView, view) {
		return view, nil
	}
	command, err := NewCommand(CommandObserveView, view)
	if err != nil {
		return ClusterView{}, err
	}
	return view, o.proposer.Propose(command)
}

func (o *Observer) PollOnce(ctx context.Context) (ClusterView, error) {
	type result struct {
		peer  config.PeerConfig
		state ClusterStateResponse
		err   error
	}
	results := make(chan result, len(o.peers))
	for _, peer := range o.peers {
		peer := peer
		go func() {
			client := o.clients[peer.ID]
			if client == nil {
				results <- result{peer: peer, err: errors.New("node client is missing")}
				return
			}
			health, err := client.Health(ctx)
			if err != nil || health.Status != "ok" || health.NodeID != peer.ID {
				if err == nil {
					err = fmt.Errorf("invalid health identity or status")
				}
				results <- result{peer: peer, err: err}
				return
			}
			state, err := client.ClusterState(ctx)
			if err == nil && (!state.Enabled || state.NodeID != peer.ID) {
				err = fmt.Errorf("invalid cluster state identity")
			}
			results <- result{peer: peer, state: state, err: err}
		}()
	}

	now := o.now().UTC()
	states := make([]ClusterStateResponse, 0, len(o.peers))
	nodes := make(map[string]NodeView, len(o.peers))
	for range o.peers {
		result := <-results
		node := NodeView{ID: result.peer.ID}
		o.mu.Lock()
		record := o.failures[result.peer.ID]
		if result.err == nil {
			record = failureRecord{}
			node.Reachable = true
			node.Returning = result.state.DataState == "recovering"
			node.Eligible = !node.Returning
			states = append(states, result.state)
		} else {
			record.consecutive++
			if record.firstMiss.IsZero() {
				record.firstMiss = now
			}
			record.failed = (o.failureCount > 0 && record.consecutive >= o.failureCount) ||
				(o.failureAfter > 0 && now.Sub(record.firstMiss) >= o.failureAfter)
			node.Suspected = !record.failed
			node.Reachable = !record.failed
			node.Eligible = !record.failed
		}
		node.ConsecFails = record.consecutive
		o.failures[result.peer.ID] = record
		o.mu.Unlock()
		nodes[result.peer.ID] = node
	}

	canonical, err := selectQuorumState(states, len(o.peers)/2+1)
	if err != nil {
		return ClusterView{}, err
	}
	view := ClusterView{ClusterID: canonical.ClusterID, MetadataVersion: canonical.MetadataVersion, Nodes: nodes, ObservedAt: now}
	view.Slots = make([]SlotView, len(canonical.Slots))
	for i, slot := range canonical.Slots {
		view.Slots[i] = SlotView{Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID, Term: slot.Term, ReplicaReady: slot.ReplicaReady}
		leader := view.Nodes[slot.LeaderID]
		leader.LeaderSlots++
		view.Nodes[slot.LeaderID] = leader
		if slot.ReplicaID != "" {
			replica := view.Nodes[slot.ReplicaID]
			replica.ReplicaSlots++
			view.Nodes[slot.ReplicaID] = replica
		}
	}
	view.Status = summarizeStatusWithThreshold(view, o.skewThreshold)
	return view, nil
}

func selectQuorumState(states []ClusterStateResponse, quorum int) (ClusterStateResponse, error) {
	type candidate struct {
		state ClusterStateResponse
		count int
	}
	groups := make(map[string]*candidate)
	for _, state := range states {
		raw, _ := json.Marshal(struct {
			ClusterID string       `json:"clusterId"`
			SlotCount uint32       `json:"slotCount"`
			Version   uint64       `json:"version"`
			Slots     []SlotStatus `json:"slots"`
		}{state.ClusterID, state.SlotCount, state.MetadataVersion, state.Slots})
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		if groups[digest] == nil {
			groups[digest] = &candidate{state: state}
		}
		groups[digest].count++
	}
	candidates := make([]candidate, 0, len(groups))
	for _, group := range groups {
		if group.count >= quorum {
			candidates = append(candidates, *group)
		}
	}
	if len(candidates) == 0 {
		return ClusterStateResponse{}, fmt.Errorf("no quorum-consistent cluster state (%d responses, quorum %d)", len(states), quorum)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].state.MetadataVersion > candidates[j].state.MetadataVersion })
	return candidates[0].state, nil
}

func materiallyEqualViews(left, right ClusterView) bool {
	left.ObservedAt, right.ObservedAt = time.Time{}, time.Time{}
	left.Status.LatestCommittedOperation, right.Status.LatestCommittedOperation = "", ""
	return reflect.DeepEqual(left, right)
}
