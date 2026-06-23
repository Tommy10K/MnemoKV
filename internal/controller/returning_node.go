package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/raft"
	"github.com/mnemokv/mnemokv/internal/config"
)

// ReturningNodeController drives fresh-node preparation separately from the
// ordinary recovery executor. It never runs while a recovery or rebalance plan
// is active.
type ReturningNodeController struct {
	proposer viewProposer
	clients  map[string]ReturningNodeAPI
	interval time.Duration
}

func NewReturningNodeController(cfg config.ClusterConfig, clients map[string]ReturningNodeAPI, proposer viewProposer) *ReturningNodeController {
	interval := time.Duration(cfg.Controller.ObserveIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}
	return &ReturningNodeController{proposer: proposer, clients: clients, interval: interval}
}

func NewReturningNodeControllerFromConfig(cfg config.ClusterConfig, secret string, proposer viewProposer) (*ReturningNodeController, error) {
	clients := make(map[string]ReturningNodeAPI, len(cfg.Peers))
	timeout := time.Duration(cfg.Controller.FailureTimeoutMs) * time.Millisecond
	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	for _, peer := range cfg.Peers {
		client, err := NewAuthenticatedNodeClient(peer.APIAddress, timeout, secret)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", peer.ID, err)
		}
		clients[peer.ID] = client
	}
	return NewReturningNodeController(cfg, clients, proposer), nil
}

func (c *ReturningNodeController) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.proposer != nil && c.proposer.IsLeader() {
				_ = c.ReconcileOnce(ctx)
			}
		}
	}
}

func (c *ReturningNodeController) ReconcileOnce(ctx context.Context) error {
	if c.proposer == nil || !c.proposer.IsLeader() {
		return raft.ErrNotLeader
	}
	state := c.proposer.State()
	if state.ActivePlan != nil {
		return nil
	}
	ids := make([]string, 0)
	for id, node := range state.LatestView.Nodes {
		if node.Reachable && node.Returning {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	id := ids[0]
	client := c.clients[id]
	if client == nil {
		return fmt.Errorf("returning node %s has no client", id)
	}
	// A true value may be from an earlier incarnation of this node. Commit a
	// fresh ineligible decision before touching its application state.
	admitted, tracked := state.ReturningNodes[id]
	if !tracked || admitted {
		command, err := NewCommand(CommandAdmitReturningNode, ReturningNodePayload{NodeID: id, Admitted: false})
		if err != nil {
			return err
		}
		return c.proposer.Propose(command)
	}
	prepared, err := client.PrepareReturning(ctx, state.LatestView.ClusterID, state.LatestView.MetadataVersion, state.ControlIndex)
	if err != nil {
		return err
	}
	if prepared.ClusterID != state.LatestView.ClusterID || prepared.MetadataVersion < state.LatestView.MetadataVersion || prepared.EntryCount != 0 || prepared.DataState != "recovering" {
		return fmt.Errorf("returning node %s failed preparation validation", id)
	}
	command, err := NewCommand(CommandAdmitReturningNode, ReturningNodePayload{NodeID: id, Admitted: true})
	if err != nil {
		return err
	}
	if err := c.proposer.Propose(command); err != nil {
		return err
	}
	committed := c.proposer.State()
	admittedState, err := client.AdmitReturning(ctx, committed.LatestView.ClusterID, committed.LatestView.MetadataVersion, committed.ControlIndex)
	if err != nil {
		return err
	}
	if admittedState.DataState != "active" || admittedState.EntryCount != 0 {
		return fmt.Errorf("returning node %s did not enter active empty state", id)
	}
	return nil
}
