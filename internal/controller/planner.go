package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
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
		Kind: PlanKindRecovery, Epoch: view.MetadataVersion, DeadNodes: deadNodes,
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

// PlanRebalance deterministically creates a bounded set of complete slot
// moves. It refuses to move while any slot has fewer than two ready copies.
type slotPlacement struct {
	number  uint32
	leader  string
	replica string
	ready   bool
}

func PlanRebalance(view ClusterView, skewThreshold, maxMoves int) (RebalancePlan, bool) {
	if skewThreshold < 0 {
		skewThreshold = 0
	}
	if maxMoves <= 0 {
		return RebalancePlan{}, false
	}
	eligible := eligibleNodeIDs(view)
	if len(eligible) < 2 {
		return RebalancePlan{}, false
	}
	slots := make([]slotPlacement, len(view.Slots))
	leaderCounts := make(map[string]int, len(eligible))
	replicaCounts := make(map[string]int, len(eligible))
	eligibleSet := make(map[string]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}
	for i, slot := range view.Slots {
		if !slot.ReplicaReady || slot.LeaderID == slot.ReplicaID {
			return RebalancePlan{}, false
		}
		if _, ok := eligibleSet[slot.LeaderID]; !ok {
			return RebalancePlan{}, false
		}
		if _, ok := eligibleSet[slot.ReplicaID]; !ok {
			return RebalancePlan{}, false
		}
		slots[i] = slotPlacement{number: slot.Number, leader: slot.LeaderID, replica: slot.ReplicaID, ready: true}
		leaderCounts[slot.LeaderID]++
		replicaCounts[slot.ReplicaID]++
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].number < slots[j].number })

	plan := RebalancePlan{
		Kind: PlanKindRebalance, Reason: "rebalance-eligible-placement", Epoch: view.MetadataVersion,
		DeadNodes: confirmedFailedNodes(view), Done: make(map[int]bool),
	}
	leaderMoved := make(map[uint32]bool)
	replicaMoved := make(map[uint32]bool)
	moves := 0
	for moves < maxMoves {
		source, target, skew := countExtremes(eligible, leaderCounts)
		if skew <= skewThreshold {
			break
		}
		index := findLeaderMove(slots, leaderMoved, source, target)
		if index < 0 {
			break
		}
		slot := &slots[index]
		oldLeader, oldReplica := slot.leader, slot.replica
		plan.Steps = append(plan.Steps,
			PlanStep{Kind: StepAssignReplica, Slot: slot.number, Target: target},
			PlanStep{Kind: StepSync, Slot: slot.number, Target: target},
			PlanStep{Kind: StepPromote, Slot: slot.number, Target: target},
			PlanStep{Kind: StepAssignReplica, Slot: slot.number, Target: oldLeader},
			PlanStep{Kind: StepSync, Slot: slot.number, Target: oldLeader},
		)
		plan.WriteBlockedSlots = append(plan.WriteBlockedSlots, slot.number)
		leaderCounts[oldLeader]--
		leaderCounts[target]++
		replicaCounts[oldReplica]--
		replicaCounts[oldLeader]++
		slot.leader, slot.replica = target, oldLeader
		leaderMoved[slot.number] = true
		moves++
	}
	for moves < maxMoves {
		source, target, skew := countExtremes(eligible, replicaCounts)
		if skew <= skewThreshold {
			break
		}
		index := findReplicaMove(slots, replicaMoved, source, target)
		if index < 0 {
			break
		}
		slot := &slots[index]
		plan.Steps = append(plan.Steps,
			PlanStep{Kind: StepAssignReplica, Slot: slot.number, Target: target},
			PlanStep{Kind: StepSync, Slot: slot.number, Target: target},
		)
		plan.WriteBlockedSlots = append(plan.WriteBlockedSlots, slot.number)
		replicaCounts[source]--
		replicaCounts[target]++
		slot.replica = target
		replicaMoved[slot.number] = true
		moves++
	}
	if len(plan.Steps) == 0 {
		return RebalancePlan{}, false
	}
	plan.ID = deterministicPlanID(plan)
	return plan, true
}

func countExtremes(nodes []string, counts map[string]int) (source, target string, skew int) {
	sorted := append([]string(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool {
		if counts[sorted[i]] != counts[sorted[j]] {
			return counts[sorted[i]] < counts[sorted[j]]
		}
		return sorted[i] < sorted[j]
	})
	target, source = sorted[0], sorted[len(sorted)-1]
	return source, target, counts[source] - counts[target]
}

func findLeaderMove(slots []slotPlacement, moved map[uint32]bool, source, target string) int {
	for i, slot := range slots {
		if !moved[slot.number] && slot.ready && slot.leader == source && target != source {
			return i
		}
	}
	return -1
}

func findReplicaMove(slots []slotPlacement, moved map[uint32]bool, source, target string) int {
	for i, slot := range slots {
		if !moved[slot.number] && slot.ready && slot.replica == source && slot.leader != target {
			return i
		}
	}
	return -1
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
	proposer      viewProposer
	interval      time.Duration
	cooldown      time.Duration
	skewThreshold int
	maxMoves      int
	now           func() time.Time
	eligibleKey   string
	stableSince   time.Time
}

func NewPlanner(proposer viewProposer, cfg config.ControllerConfig) *Planner {
	interval := time.Duration(cfg.ObserveIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}
	cooldown := time.Duration(cfg.FailureTimeoutMs) * time.Millisecond
	if cooldown <= 0 {
		cooldown = 10 * time.Second
	}
	threshold := cfg.RebalanceSkewThreshold
	if threshold <= 0 {
		threshold = 1
	}
	maxMoves := cfg.MigrationRateLimit
	if maxMoves <= 0 {
		maxMoves = 1
	}
	return &Planner{proposer: proposer, interval: interval, cooldown: cooldown, skewThreshold: threshold, maxMoves: maxMoves, now: time.Now}
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
		return p.evaluateRebalance(state)
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
	// Metadata normally advances after every successful executor step. Those
	// observations are progress, not invalidation: the executor checks each
	// remaining step's postcondition against converged live metadata. A newly
	// confirmed failure is the condition that invalidates its copy assumptions.
	if !hasNewConfirmedFailure(state.ActivePlan.DeadNodes, plan.DeadNodes) {
		return nil
	}
	command, err := NewCommand(CommandSupersedePlan, SupersedePlanPayload{OldPlanID: state.ActivePlan.ID, NewPlan: plan})
	if err != nil {
		return err
	}
	return p.proposer.Propose(command)
}

func (p *Planner) evaluateRebalance(state FSMSnapshot) error {
	if state.ActivePlan != nil || hasSuspectedNode(state.LatestView) {
		p.resetStability()
		return nil
	}
	key := strings.Join(eligibleNodeIDs(state.LatestView), ",")
	if key == "" {
		p.resetStability()
		return nil
	}
	now := p.now()
	if key != p.eligibleKey {
		p.eligibleKey, p.stableSince = key, now
		return nil
	}
	if now.Sub(p.stableSince) < p.cooldown {
		return nil
	}
	plan, needed := PlanRebalance(state.LatestView, p.skewThreshold, p.maxMoves)
	if !needed || state.LastCompletedPlanID == plan.ID {
		return nil
	}
	command, err := NewCommand(CommandProposeRebalance, plan)
	if err != nil {
		return err
	}
	return p.proposer.Propose(command)
}

func (p *Planner) resetStability() {
	p.eligibleKey = ""
	p.stableSince = time.Time{}
}

func hasSuspectedNode(view ClusterView) bool {
	for _, node := range view.Nodes {
		if node.Suspected {
			return true
		}
	}
	return false
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
