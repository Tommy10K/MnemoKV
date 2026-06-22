package controller

import (
	"sort"

	"github.com/mnemokv/mnemokv/internal/config"
)

type TopologySets struct {
	Configured  []string
	Voters      []string
	Eligible    []string
	Unavailable []string
}

func DeriveTopology(peers []config.PeerConfig, view ClusterView) TopologySets {
	sets := TopologySets{}
	for _, peer := range peers {
		sets.Configured = append(sets.Configured, peer.ID)
		if peer.FailoverMode == "automatic" {
			sets.Voters = append(sets.Voters, peer.ID)
		}
		node, ok := view.Nodes[peer.ID]
		switch {
		case ok && node.Eligible:
			sets.Eligible = append(sets.Eligible, peer.ID)
		case !ok || !node.Suspected:
			sets.Unavailable = append(sets.Unavailable, peer.ID)
		}
	}
	sort.Strings(sets.Configured)
	sort.Strings(sets.Voters)
	sort.Strings(sets.Eligible)
	sort.Strings(sets.Unavailable)
	return sets
}

func ClassifySlots(view ClusterView) map[uint32]SlotClass {
	classes := make(map[uint32]SlotClass, len(view.Slots))
	for _, slot := range view.Slots {
		leaderAvailable := ownerAvailable(view.Nodes, slot.LeaderID)
		replicaAvailable := slot.ReplicaReady && ownerAvailable(view.Nodes, slot.ReplicaID)
		switch {
		case leaderAvailable && replicaAvailable:
			classes[slot.Number] = SlotUnaffected
		case !leaderAvailable && replicaAvailable:
			classes[slot.Number] = SlotLeaderless
		case leaderAvailable:
			classes[slot.Number] = SlotReplicaLost
		default:
			classes[slot.Number] = SlotNoSurvivingCopy
		}
	}
	return classes
}

func ownerAvailable(nodes map[string]NodeView, nodeID string) bool {
	if nodeID == "" {
		return false
	}
	node, ok := nodes[nodeID]
	return ok && node.Reachable && node.Eligible
}

func summarizeStatus(view ClusterView) StatusSummary {
	summary := StatusSummary{State: StatusHealthy}
	for id, node := range view.Nodes {
		switch {
		case node.Suspected:
			summary.SuspectedNodes = append(summary.SuspectedNodes, id)
		case !node.Reachable || !node.Eligible:
			summary.FailedNodes = append(summary.FailedNodes, id)
		}
	}
	sort.Strings(summary.SuspectedNodes)
	sort.Strings(summary.FailedNodes)
	for _, class := range ClassifySlots(view) {
		switch class {
		case SlotLeaderless, SlotReplicaLost:
			summary.DegradedSlots++
		case SlotNoSurvivingCopy:
			summary.UnavailableSlots++
		}
	}
	switch {
	case summary.UnavailableSlots > 0:
		summary.State = StatusPotentialDataLoss
	case len(summary.FailedNodes) > 0 || summary.DegradedSlots > 0:
		summary.State = StatusDegraded
	case len(summary.SuspectedNodes) > 0:
		summary.State = StatusFailureSuspected
	}
	return summary
}
