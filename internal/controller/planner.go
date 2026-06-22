package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// PlanFailover is pure: the same committed view always produces the same plan.
func PlanFailover(view ClusterView) (RecoveryPlan, bool) {
	classes := ClassifySlots(view)
	deadNodes := confirmedFailedNodes(view)
	loads := make(map[string]int, len(view.Nodes))
	for id, node := range view.Nodes {
		loads[id] = node.LeaderSlots + node.ReplicaSlots
	}

	slots := append([]SlotView(nil), view.Slots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].Number < slots[j].Number })
	plan := RecoveryPlan{
		Kind: PlanRecovery, Epoch: view.MetadataVersion, DeadNodes: deadNodes,
		Done: make(map[int]bool),
	}
	for _, slot := range slots {
		switch classes[slot.Number] {
		case SlotLeaderless:
			newLeader := slot.ReplicaID
			plan.Steps = append(plan.Steps, PlanStep{Kind: StepPromote, Slot: slot.Number, Target: newLeader})
			plan.WriteBlockedSlots = append(plan.WriteBlockedSlots, slot.Number)
			target, ok := leastLoadedEligible(view, loads, newLeader)
			if ok {
				plan.Steps = append(plan.Steps,
					PlanStep{Kind: StepAssignReplica, Slot: slot.Number, Target: target},
					PlanStep{Kind: StepSync, Slot: slot.Number, Target: target},
				)
				loads[target]++
			}
		case SlotReplicaLost:
			if slot.ReplicaID != "" && ownerAvailable(view.Nodes, slot.ReplicaID) {
				plan.Steps = append(plan.Steps, PlanStep{Kind: StepSync, Slot: slot.Number, Target: slot.ReplicaID})
				continue
			}
			target, ok := leastLoadedEligible(view, loads, slot.LeaderID)
			if ok {
				plan.Steps = append(plan.Steps,
					PlanStep{Kind: StepAssignReplica, Slot: slot.Number, Target: target},
					PlanStep{Kind: StepSync, Slot: slot.Number, Target: target},
				)
				loads[target]++
			}
		case SlotNoSurvivingCopy:
			plan.Unrecoverable = append(plan.Unrecoverable, slot.Number)
			plan.Steps = append(plan.Steps, PlanStep{Kind: StepMarkUnavailable, Slot: slot.Number})
		}
	}
	if len(plan.Steps) == 0 && len(plan.Unrecoverable) == 0 {
		return RecoveryPlan{}, false
	}
	plan.Reason = recoveryReason(deadNodes)
	plan.ID = deterministicPlanID(plan)
	return plan, true
}

func leastLoadedEligible(view ClusterView, loads map[string]int, excluded string) (string, bool) {
	candidates := make([]string, 0, len(view.Nodes))
	for id, node := range view.Nodes {
		if id != excluded && node.Eligible && node.Reachable && !node.Suspected {
			candidates = append(candidates, id)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if loads[candidates[i]] != loads[candidates[j]] {
			return loads[candidates[i]] < loads[candidates[j]]
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) == 0 {
		return "", false
	}
	return candidates[0], true
}

func confirmedFailedNodes(view ClusterView) []string {
	failed := make([]string, 0)
	for id, node := range view.Nodes {
		if !node.Suspected && (!node.Reachable || !node.Eligible) {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

func recoveryReason(deadNodes []string) string {
	if len(deadNodes) == 0 {
		return "repair-degraded-replication"
	}
	return "node-down:" + strings.Join(deadNodes, ",")
}

func deterministicPlanID(plan RecoveryPlan) string {
	copy := plan
	copy.ID = ""
	copy.Done = nil
	raw, _ := json.Marshal(copy)
	digest := sha256.Sum256(raw)
	return string(plan.Kind) + "-" + hex.EncodeToString(digest[:8])
}

type Planner struct {
	proposer viewProposer
	interval time.Duration
}

func NewPlanner(proposer viewProposer, interval time.Duration) *Planner {
	if interval <= 0 {
		interval = time.Second
	}
	return &Planner{proposer: proposer, interval: interval}
}

func (p *Planner) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.Evaluate()
		}
	}
}

func (p *Planner) Evaluate() error {
	if p.proposer == nil || !p.proposer.IsLeader() {
		return nil
	}
	state := p.proposer.State()
	plan, needed := PlanFailover(state.LatestView)
	if !needed {
		return nil
	}
	if state.ActivePlan == nil {
		if state.LastCompletedPlanID == plan.ID {
			return nil
		}
		command, err := NewCommand(CommandProposePlan, plan)
		if err != nil {
			return err
		}
		return p.proposer.Propose(command)
	}
	if state.ActivePlan.ID == plan.ID {
		return nil
	}
	viewAdvanced := state.LatestView.MetadataVersion > state.ActivePlan.Epoch
	if !viewAdvanced && !hasNewConfirmedFailure(state.ActivePlan.DeadNodes, plan.DeadNodes) {
		return nil
	}
	command, err := NewCommand(CommandSupersedePlan, SupersedePlanPayload{OldPlanID: state.ActivePlan.ID, NewPlan: plan})
	if err != nil {
		return err
	}
	return p.proposer.Propose(command)
}

func hasNewConfirmedFailure(active, current []string) bool {
	existing := make(map[string]struct{}, len(active))
	for _, id := range active {
		existing[id] = struct{}{}
	}
	for _, id := range current {
		if _, ok := existing[id]; !ok {
			return true
		}
	}
	return false
}
