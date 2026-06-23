package controller

import (
	"sort"

	"github.com/mnemokv/mnemokv/internal/controlplane"
)

const (
	degradedFailureWarning = "another failure before repair completes may cause slot unavailability or data loss"
	returningDataPolicy    = "v1 does not inspect, trust, or merge returning-node application data for recovery"
)

var rejectedKeyCommands = []string{
	"DEL", "EXISTS", "EXPIRE", "GET", "INCR", "LLEN", "LPOP", "LPUSH",
	"RPOP", "RPUSH", "SET", "TTL", "ZADD", "ZCARD", "ZRANGE", "ZSCORE",
}

var rejectedWriteCommands = []string{"DEL", "EXPIRE", "INCR", "LPOP", "LPUSH", "RPOP", "RPUSH", "SET", "ZADD"}

func BuildStatusSnapshot(state FSMSnapshot) controlplane.StatusSnapshot {
	view := state.LatestView
	statusState := string(view.Status.State)
	if statusState == "" {
		statusState = "starting"
	}
	status := controlplane.StatusSnapshot{
		State: statusState, ControlIndex: state.ControlIndex,
		FailedNodes:              append([]string(nil), view.Status.FailedNodes...),
		SuspectedNodes:           append([]string(nil), view.Status.SuspectedNodes...),
		LatestCommittedOperation: view.Status.LatestCommittedOperation,
	}
	if state.ActivePlan != nil {
		completed := 0
		for index := range state.ActivePlan.Steps {
			if state.ActivePlan.Done[index] {
				completed++
			}
		}
		status.ActivePlan = &controlplane.PlanStatus{
			ID: state.ActivePlan.ID, Kind: string(state.ActivePlan.Kind), Reason: state.ActivePlan.Reason,
			CompletedSteps: completed, TotalSteps: len(state.ActivePlan.Steps),
		}
	}

	classes := ClassifySlots(view)
	slots := append([]SlotView(nil), view.Slots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].Number < slots[j].Number })
	for _, slot := range slots {
		classification := classes[slot.Number]
		if classification == SlotUnaffected {
			continue
		}
		detail := controlplane.SlotStatus{
			Slot: slot.Number, Classification: string(classification),
			FormerLeaderID: slot.LeaderID, FormerReplicaID: slot.ReplicaID,
			Failures: failedOwners(view, slot), WritesAvailable: false,
		}
		if len(detail.Failures) == 0 && state.ActivePlan != nil {
			detail.Failures = append([]string(nil), state.ActivePlan.DeadNodes...)
		}
		switch classification {
		case SlotLeaderless:
			detail.Message = "slot unavailable until its surviving replica is promoted"
			detail.RejectedCommands = append([]string(nil), rejectedKeyCommands...)
		case SlotReplicaLost:
			detail.ReadsAvailable = true
			detail.Message = "slot has one reachable authoritative copy; reads continue but writes are rejected until replica repair completes"
			detail.RejectedCommands = append([]string(nil), rejectedWriteCommands...)
		case SlotNoSurvivingCopy:
			detail.Message = "slot unavailable — no authoritative copy currently reachable; data may be lost"
			detail.RejectedCommands = append([]string(nil), rejectedKeyCommands...)
			if committed, ok := state.Unavailable[slot.Number]; ok {
				detail.FormerLeaderID, detail.FormerReplicaID = committed.LeaderID, committed.ReplicaID
				detail.Failures = append([]string(nil), committed.Failures...)
			}
			status.UnavailableSlots = append(status.UnavailableSlots, detail)
		}
		if classification != SlotNoSurvivingCopy {
			status.OneCopySlots = append(status.OneCopySlots, detail)
		}
		status.AffectedSlotRanges = appendRange(status.AffectedSlotRanges, slot.Number, string(classification))
	}
	if len(status.OneCopySlots) > 0 || len(status.UnavailableSlots) > 0 {
		status.Warning = degradedFailureWarning
	}
	if len(status.UnavailableSlots) > 0 {
		status.ReturningNodeDataPolicy = returningDataPolicy
	}
	return status
}

func BuildControllerStateSnapshot(state FSMSnapshot, nodeID, raftRole, leaderID string, term uint64, isLeader bool) controlplane.ControllerStateSnapshot {
	view := state.LatestView
	result := controlplane.ControllerStateSnapshot{
		NodeID: nodeID, RaftRole: raftRole, RaftLeaderID: leaderID, RaftTerm: term, IsLeader: isLeader,
		ControlIndex: state.ControlIndex, Recovery: BuildStatusSnapshot(state),
		CurrentView: controlplane.ControllerView{
			ClusterID: view.ClusterID, MetadataVersion: view.MetadataVersion,
			ObservedAt: view.ObservedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"), Status: string(view.Status.State),
			Nodes: make([]controlplane.ControllerNodeStatus, 0, len(view.Nodes)),
			Slots: make([]controlplane.ControllerSlotStatus, 0, len(view.Slots)),
		},
	}
	if view.ObservedAt.IsZero() {
		result.CurrentView.ObservedAt = ""
	}
	ids := make([]string, 0, len(view.Nodes))
	for id := range view.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		node := view.Nodes[id]
		result.CurrentView.Nodes = append(result.CurrentView.Nodes, controlplane.ControllerNodeStatus{
			ID: id, Reachable: node.Reachable, Suspected: node.Suspected, Eligible: node.Eligible,
			Returning: node.Returning, LeaderSlots: node.LeaderSlots, ReplicaSlots: node.ReplicaSlots,
		})
	}
	slots := append([]SlotView(nil), view.Slots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].Number < slots[j].Number })
	for _, slot := range slots {
		result.CurrentView.Slots = append(result.CurrentView.Slots, controlplane.ControllerSlotStatus{
			Number: slot.Number, LeaderID: slot.LeaderID, ReplicaID: slot.ReplicaID, Term: slot.Term, ReplicaReady: slot.ReplicaReady,
		})
	}
	if state.LastRebalance != nil {
		result.LastRebalance = &controlplane.CompletedPlanStatus{
			ID: state.LastRebalance.ID, Kind: string(state.LastRebalance.Kind), Epoch: state.LastRebalance.Epoch,
			ControlIndex: state.LastRebalance.ControlIndex,
		}
	}
	return result
}

func failedOwners(view ClusterView, slot SlotView) []string {
	owners := []string{slot.LeaderID, slot.ReplicaID}
	failed := make([]string, 0, 2)
	for _, id := range owners {
		if id == "" {
			continue
		}
		node, ok := view.Nodes[id]
		if !ok || (!node.Suspected && (!node.Reachable || !node.Eligible)) {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

func appendRange(ranges []controlplane.SlotRange, slot uint32, classification string) []controlplane.SlotRange {
	if len(ranges) > 0 {
		last := &ranges[len(ranges)-1]
		if last.Classification == classification && last.End+1 == slot {
			last.End = slot
			return ranges
		}
	}
	return append(ranges, controlplane.SlotRange{Start: slot, End: slot, Classification: classification})
}
