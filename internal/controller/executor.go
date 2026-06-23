package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/mnemokv/mnemokv/internal/config"
)

var ErrPlanSuperseded = errors.New("recovery plan was superseded")

type Executor struct {
	proposer       viewProposer
	clients        map[string]AdminNodeAPI
	interval       time.Duration
	failureTimeout time.Duration
	syncInterval   time.Duration

	mu       sync.Mutex
	lastSync time.Time
}

func NewExecutor(cfg config.ClusterConfig, clients map[string]AdminNodeAPI, proposer viewProposer) *Executor {
	rate := cfg.Controller.MigrationRateLimit
	if rate <= 0 {
		rate = 1
	}
	interval := time.Duration(cfg.Controller.ObserveIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}
	failureTimeout := time.Duration(cfg.Controller.FailureTimeoutMs) * time.Millisecond
	if failureTimeout <= 0 {
		failureTimeout = 10 * time.Second
	}
	return &Executor{
		proposer: proposer, clients: clients, interval: interval, failureTimeout: failureTimeout,
		syncInterval: time.Second / time.Duration(rate),
	}
}

func NewExecutorFromConfig(cfg config.ClusterConfig, secret string, proposer viewProposer) (*Executor, error) {
	clients := make(map[string]AdminNodeAPI, len(cfg.Peers))
	timeout := time.Duration(cfg.Controller.ObserveIntervalMs) * time.Millisecond
	if timeout <= 0 || timeout > 2*time.Second {
		timeout = 2 * time.Second
	}
	for _, peer := range cfg.Peers {
		client, err := NewAuthenticatedNodeClient(peer.APIAddress, timeout, secret)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", peer.ID, err)
		}
		clients[peer.ID] = client
	}
	return NewExecutor(cfg, clients, proposer), nil
}

func (e *Executor) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e.proposer != nil && e.proposer.IsLeader() {
				_ = e.ExecuteOnce(ctx)
			}
		}
	}
}

func (e *Executor) ExecuteOnce(ctx context.Context) error {
	if e.proposer == nil || !e.proposer.IsLeader() {
		return raft.ErrNotLeader
	}
	initial := e.proposer.State()
	if initial.ActivePlan == nil {
		return nil
	}
	planID := initial.ActivePlan.ID
	for stepIndex := range initial.ActivePlan.Steps {
		state, plan, err := e.activePlan(planID)
		if err != nil {
			return err
		}
		if plan.Done[stepIndex] {
			continue
		}
		step := plan.Steps[stepIndex]
		if step.Kind == StepMarkUnavailable {
			if _, recorded := state.Unavailable[step.Slot]; !recorded {
				unavailable := e.unavailableRecord(*plan, step.Slot, state.LatestView)
				command, commandErr := NewCommand(CommandMarkUnavailable, []UnavailableSlot{unavailable})
				if commandErr != nil {
					return commandErr
				}
				if err := e.proposer.Propose(command); err != nil {
					return err
				}
			}
		} else {
			if err := e.executeTopologyStep(ctx, planID, *plan, step); err != nil {
				return err
			}
		}
		if _, _, err := e.activePlan(planID); err != nil {
			return err
		}
		done, err := NewCommand(CommandStepDone, StepDonePayload{PlanID: planID, StepIndex: stepIndex})
		if err != nil {
			return err
		}
		if err := e.proposer.Propose(done); err != nil {
			return err
		}
	}
	if _, _, err := e.activePlan(planID); err != nil {
		return err
	}
	complete, err := NewCommand(CommandPlanComplete, PlanIDPayload{PlanID: planID})
	if err != nil {
		return err
	}
	return e.proposer.Propose(complete)
}

func (e *Executor) executeTopologyStep(ctx context.Context, planID string, plan RecoveryPlan, step PlanStep) error {
	if step.Kind == StepSync {
		if err := e.waitForSyncRate(ctx); err != nil {
			return err
		}
	}
	converged, err := e.waitForConvergence(ctx, planID, plan.Epoch)
	if err != nil {
		return err
	}
	if stepPostcondition(converged, step) {
		return nil
	}
	state, _, err := e.activePlan(planID)
	if err != nil {
		return err
	}
	controlIndex := state.ControlIndex
	var callErr error
	switch step.Kind {
	case StepPromote:
		client := e.clients[step.Target]
		if client == nil {
			return fmt.Errorf("promote target %q has no client", step.Target)
		}
		_, callErr = client.Promote(ctx, step.Slot, controlIndex)
	case StepAssignReplica, StepSync:
		slot, ok := findSlot(converged, step.Slot)
		if !ok || slot.LeaderID == "" {
			return fmt.Errorf("slot %d has no current leader", step.Slot)
		}
		client := e.clients[slot.LeaderID]
		if client == nil {
			return fmt.Errorf("leader %q has no client", slot.LeaderID)
		}
		if step.Kind == StepAssignReplica {
			_, callErr = client.AssignReplica(ctx, step.Slot, step.Target, controlIndex)
		} else {
			_, callErr = client.SyncReplica(ctx, step.Slot, step.Target, controlIndex)
		}
	default:
		return fmt.Errorf("unsupported plan step %q", step.Kind)
	}

	verified, verifyErr := e.waitForConvergence(ctx, planID, plan.Epoch)
	if verifyErr == nil && stepPostcondition(verified, step) {
		return nil
	}
	if callErr != nil {
		return fmt.Errorf("execute %s for slot %d: %w", step.Kind, step.Slot, callErr)
	}
	if verifyErr != nil {
		return verifyErr
	}
	return fmt.Errorf("step %s for slot %d did not reach its postcondition", step.Kind, step.Slot)
}

func (e *Executor) activePlan(planID string) (FSMSnapshot, *RecoveryPlan, error) {
	if !e.proposer.IsLeader() {
		return FSMSnapshot{}, nil, raft.ErrNotLeader
	}
	state := e.proposer.State()
	if state.ActivePlan == nil || state.ActivePlan.ID != planID {
		return FSMSnapshot{}, nil, ErrPlanSuperseded
	}
	return state, state.ActivePlan, nil
}

func (e *Executor) waitForConvergence(ctx context.Context, planID string, epoch uint64) (ClusterStateResponse, error) {
	deadline := time.Now().Add(e.failureTimeout)
	for {
		state, _, err := e.activePlan(planID)
		if err != nil {
			return ClusterStateResponse{}, err
		}
		eligible := eligibleNodeIDs(state.LatestView)
		responses := make([]ClusterStateResponse, 0, len(eligible))
		allReached := len(eligible) > 0
		for _, nodeID := range eligible {
			client := e.clients[nodeID]
			if client == nil {
				allReached = false
				break
			}
			response, fetchErr := client.ClusterState(ctx)
			if fetchErr != nil || response.MetadataVersion < epoch {
				allReached = false
				break
			}
			responses = append(responses, response)
		}
		if allReached && clusterStatesConverged(responses) {
			return responses[0], nil
		}
		if time.Now().After(deadline) {
			return ClusterStateResponse{}, fmt.Errorf("metadata did not converge for plan %s before failure timeout", planID)
		}
		select {
		case <-ctx.Done():
			return ClusterStateResponse{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func clusterStatesConverged(states []ClusterStateResponse) bool {
	if len(states) == 0 {
		return false
	}
	first := states[0]
	for _, state := range states[1:] {
		if state.MetadataVersion != first.MetadataVersion || state.ClusterID != first.ClusterID ||
			state.SlotCount != first.SlotCount || !reflect.DeepEqual(state.Slots, first.Slots) {
			return false
		}
	}
	return true
}

func eligibleNodeIDs(view ClusterView) []string {
	ids := make([]string, 0, len(view.Nodes))
	for id, node := range view.Nodes {
		if node.Eligible && node.Reachable && !node.Suspected {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func findSlot(state ClusterStateResponse, number uint32) (SlotStatus, bool) {
	if int(number) < len(state.Slots) && state.Slots[number].Number == number {
		return state.Slots[number], true
	}
	for _, slot := range state.Slots {
		if slot.Number == number {
			return slot, true
		}
	}
	return SlotStatus{}, false
}

func stepPostcondition(state ClusterStateResponse, step PlanStep) bool {
	slot, ok := findSlot(state, step.Slot)
	if !ok {
		return false
	}
	switch step.Kind {
	case StepPromote:
		return slot.LeaderID == step.Target && slot.ReplicaID == "" && !slot.ReplicaReady
	case StepAssignReplica:
		return slot.ReplicaID == step.Target && !slot.ReplicaReady && slot.LeaderID != step.Target
	case StepSync:
		return slot.ReplicaID == step.Target && slot.ReplicaReady && slot.LeaderID != step.Target
	default:
		return false
	}
}

func (e *Executor) waitForSyncRate(ctx context.Context) error {
	e.mu.Lock()
	wait := e.syncInterval - time.Since(e.lastSync)
	e.mu.Unlock()
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	e.mu.Lock()
	e.lastSync = time.Now()
	e.mu.Unlock()
	return nil
}

func (e *Executor) unavailableRecord(plan RecoveryPlan, slotNumber uint32, view ClusterView) UnavailableSlot {
	record := UnavailableSlot{Slot: slotNumber, Failures: append([]string(nil), plan.DeadNodes...)}
	for _, slot := range view.Slots {
		if slot.Number == slotNumber {
			record.LeaderID, record.ReplicaID = slot.LeaderID, slot.ReplicaID
			break
		}
	}
	return record
}
