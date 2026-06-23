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
	return summarizeStatusWithThreshold(view, 1)
}

func summarizeStatusWithThreshold(view ClusterView, skewThreshold int) StatusSummary {
	summary := StatusSummary{State: StatusHealthy}
	hasLeaderless := false
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
		case SlotLeaderless:
			hasLeaderless = true
			summary.DegradedSlots++
		case SlotReplicaLost:
			summary.DegradedSlots++
		case SlotNoSurvivingCopy:
			summary.UnavailableSlots++
		}
	}
	switch {
	case summary.UnavailableSlots > 0:
		summary.State = StatusPotentialDataLoss
	case hasLeaderless:
		summary.State = StatusUnavailable
	case summary.DegradedSlots > 0:
		summary.State = StatusDegraded
	case len(summary.SuspectedNodes) > 0:
		summary.State = StatusFailureSuspected
	case placementSkew(view) > skewThreshold:
		summary.State = StatusRebalancing
	}
	return summary
}

func placementSkew(view ClusterView) int {
	leaders := make([]int, 0)
	replicas := make([]int, 0)
	for _, node := range view.Nodes {
		if node.Eligible && node.Reachable && !node.Suspected {
			leaders = append(leaders, node.LeaderSlots)
			replicas = append(replicas, node.ReplicaSlots)
		}
	}
	leaderSkew := countSkew(leaders)
	replicaSkew := countSkew(replicas)
	if replicaSkew > leaderSkew {
		return replicaSkew
	}
	return leaderSkew
}

func countSkew(counts []int) int {
	if len(counts) == 0 {
		return 0
	}
	minimum, maximum := counts[0], counts[0]
	for _, count := range counts[1:] {
		if count < minimum {
			minimum = count
		}
		if count > maximum {
			maximum = count
		}
	}
	return maximum - minimum
}
