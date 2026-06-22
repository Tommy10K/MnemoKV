package api

import "github.com/mnemokv/mnemokv/internal/controlplane"

type HealthResponse struct {
	Status    string `json:"status"`
	NodeID    string `json:"nodeId"`
	Mode      string `json:"mode"`
	DataState string `json:"dataState,omitempty"`
}

type EngineStateResponse struct {
	UsedBytes      uint64  `json:"usedBytes"`
	MemoryLimit    uint64  `json:"memoryLimit"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsageRatio     float64 `json:"usageRatio"`
	EvictionPolicy string  `json:"evictionPolicy"`
	RejectedWrites uint64  `json:"rejectedWrites"`
}

type MetricsSummary struct {
	Counters map[string]uint64  `json:"counters"`
	Gauges   map[string]float64 `json:"gauges,omitempty"`
}

type ClusterStateResponse struct {
	Enabled         bool                         `json:"enabled"`
	NodeID          string                       `json:"nodeId"`
	ClusterID       string                       `json:"clusterId,omitempty"`
	SlotCount       uint32                       `json:"slotCount,omitempty"`
	MetadataVersion uint64                       `json:"metadataVersion,omitempty"`
	RoutingMode     string                       `json:"routingMode,omitempty"`
	FailoverMode    string                       `json:"failoverMode,omitempty"`
	Peers           []string                     `json:"peers"`
	Membership      []PeerStatus                 `json:"membership,omitempty"`
	Slots           []SlotStatus                 `json:"slots,omitempty"`
	Recovery        *controlplane.StatusSnapshot `json:"recovery,omitempty"`
	DataState       string                       `json:"dataState,omitempty"`
}

type ReturningNodeResponse struct {
	ClusterID        string `json:"clusterId"`
	MetadataVersion  uint64 `json:"metadataVersion"`
	EntryCount       int    `json:"entryCount"`
	RemovedSnapshots int    `json:"removedSnapshots"`
	DataState        string `json:"dataState"`
}

type SlotStatus struct {
	Number              uint32 `json:"number"`
	LeaderID            string `json:"leaderId"`
	ReplicaID           string `json:"replicaId,omitempty"`
	LocalRole           string `json:"localRole"`
	Term                uint64 `json:"term"`
	LastSequence        uint64 `json:"lastSequence"`
	LastAppliedSequence uint64 `json:"lastAppliedSequence"`
	ReplicaReady        bool   `json:"replicaReady"`
}

type ClusterAdminResponse struct {
	MetadataVersion uint64     `json:"metadataVersion"`
	Slot            SlotStatus `json:"slot"`
	FailedPeers     []string   `json:"failedPeers,omitempty"`
}

type PeerStatus struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	State   string `json:"state"`
}
