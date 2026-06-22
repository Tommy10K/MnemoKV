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
