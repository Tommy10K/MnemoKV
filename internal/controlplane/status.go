package controlplane

type StatusSnapshot struct {
	State                    string       `json:"state"`
	ControlIndex             uint64       `json:"controlIndex"`
	FailedNodes              []string     `json:"failedNodes,omitempty"`
	SuspectedNodes           []string     `json:"suspectedNodes,omitempty"`
	AffectedSlotRanges       []SlotRange  `json:"affectedSlotRanges,omitempty"`
	OneCopySlots             []SlotStatus `json:"oneCopySlots,omitempty"`
	UnavailableSlots         []SlotStatus `json:"unavailableSlots,omitempty"`
	ActivePlan               *PlanStatus  `json:"activePlan,omitempty"`
	LatestCommittedOperation string       `json:"latestCommittedOperation,omitempty"`
	Warning                  string       `json:"warning,omitempty"`
	ReturningNodeDataPolicy  string       `json:"returningNodeDataPolicy,omitempty"`
}

type SlotRange struct {
	Start          uint32 `json:"start"`
	End            uint32 `json:"end"`
	Classification string `json:"classification"`
}

type SlotStatus struct {
	Slot             uint32   `json:"slot"`
	Classification   string   `json:"classification"`
	FormerLeaderID   string   `json:"formerLeaderId"`
	FormerReplicaID  string   `json:"formerReplicaId,omitempty"`
	Failures         []string `json:"failures,omitempty"`
	ReadsAvailable   bool     `json:"readsAvailable"`
	WritesAvailable  bool     `json:"writesAvailable"`
	RejectedCommands []string `json:"rejectedCommands,omitempty"`
	Message          string   `json:"message"`
}

type PlanStatus struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Reason         string `json:"reason"`
	CompletedSteps int    `json:"completedSteps"`
	TotalSteps     int    `json:"totalSteps"`
}

type ControllerStateSnapshot struct {
	NodeID        string               `json:"nodeId"`
	RaftRole      string               `json:"raftRole"`
	RaftLeaderID  string               `json:"raftLeaderId,omitempty"`
	RaftTerm      uint64               `json:"raftTerm"`
	IsLeader      bool                 `json:"isLeader"`
	ControlIndex  uint64               `json:"controlIndex"`
	CurrentView   ControllerView       `json:"currentView"`
	Recovery      StatusSnapshot       `json:"recovery"`
	LastRebalance *CompletedPlanStatus `json:"lastRebalance,omitempty"`
}

type ControllerView struct {
	ClusterID       string                 `json:"clusterId"`
	MetadataVersion uint64                 `json:"metadataVersion"`
	ObservedAt      string                 `json:"observedAt,omitempty"`
	Status          string                 `json:"status"`
	Nodes           []ControllerNodeStatus `json:"nodes"`
	Slots           []ControllerSlotStatus `json:"slots"`
}

type ControllerNodeStatus struct {
	ID           string `json:"id"`
	Reachable    bool   `json:"reachable"`
	Suspected    bool   `json:"suspected"`
	Eligible     bool   `json:"eligible"`
	Returning    bool   `json:"returning"`
	LeaderSlots  int    `json:"leaderSlots"`
	ReplicaSlots int    `json:"replicaSlots"`
}

type ControllerSlotStatus struct {
	Number       uint32 `json:"number"`
	LeaderID     string `json:"leaderId"`
	ReplicaID    string `json:"replicaId,omitempty"`
	Term         uint64 `json:"term"`
	ReplicaReady bool   `json:"replicaReady"`
}

type CompletedPlanStatus struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Epoch        uint64 `json:"epoch"`
	ControlIndex uint64 `json:"controlIndex"`
}
