package api

type HealthResponse struct {
	Status string `json:"status"`
	NodeID string `json:"nodeId"`
	Mode   string `json:"mode"`
}

type EngineStateResponse struct {
	UsedBytes      uint64  `json:"usedBytes"`
	MemoryLimit    uint64  `json:"memoryLimit"`
	UsageRatio     float64 `json:"usageRatio"`
	EvictionPolicy string  `json:"evictionPolicy"`
}

type MetricsSummary struct {
	Counters map[string]uint64 `json:"counters"`
}

type ClusterStateResponse struct {
	Enabled      bool         `json:"enabled"`
	NodeID       string       `json:"nodeId"`
	WriteMode    string       `json:"writeMode"`
	AutoFailover bool         `json:"autoFailover"`
	Term         uint64       `json:"term,omitempty"`
	Peers        []string     `json:"peers"`
	Membership   []PeerStatus `json:"membership,omitempty"`
}

type PeerStatus struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	State   string `json:"state"`
}
