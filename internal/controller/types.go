package controller

import (
	"encoding/json"
	"time"
)

type ClusterStatus string

const (
	StatusHealthy           ClusterStatus = "healthy"
	StatusFailureSuspected  ClusterStatus = "failure_suspected"
	StatusDegraded          ClusterStatus = "degraded"
	StatusPromoting         ClusterStatus = "promoting"
	StatusRepairing         ClusterStatus = "repairing"
	StatusRebalancing       ClusterStatus = "rebalancing"
	StatusUnavailable       ClusterStatus = "unavailable"
	StatusPotentialDataLoss ClusterStatus = "potential_data_loss"
)

type ClusterView struct {
	MetadataVersion uint64              `json:"metadataVersion"`
	Slots           []SlotView          `json:"slots"`
	Nodes           map[string]NodeView `json:"nodes"`
	ObservedAt      time.Time           `json:"observedAt"`
	Status          StatusSummary       `json:"status"`
}

type NodeView struct {
	ID           string `json:"id"`
	Reachable    bool   `json:"reachable"`
	Suspected    bool   `json:"suspected"`
	ConsecFails  int    `json:"consecutiveFails"`
	LeaderSlots  int    `json:"leaderSlots"`
	ReplicaSlots int    `json:"replicaSlots"`
	Eligible     bool   `json:"eligible"`
}

type SlotView struct {
	Number       uint32 `json:"number"`
	LeaderID     string `json:"leaderId"`
	ReplicaID    string `json:"replicaId,omitempty"`
	Term         uint64 `json:"term"`
	ReplicaReady bool   `json:"replicaReady"`
}

type StatusSummary struct {
	State                    ClusterStatus `json:"state"`
	FailedNodes              []string      `json:"failedNodes,omitempty"`
	SuspectedNodes           []string      `json:"suspectedNodes,omitempty"`
	DegradedSlots            int           `json:"degradedSlots"`
	UnavailableSlots         int           `json:"unavailableSlots"`
	LatestCommittedOperation string        `json:"latestCommittedOperation,omitempty"`
}

type SlotClass string

const (
	SlotUnaffected      SlotClass = "unaffected"
	SlotLeaderless      SlotClass = "leaderless"
	SlotReplicaLost     SlotClass = "replica_lost"
	SlotNoSurvivingCopy SlotClass = "no_surviving_copy"
)

type StepKind string

const (
	StepPromote         StepKind = "promote"
	StepAssignReplica   StepKind = "assign_replica"
	StepSync            StepKind = "sync"
	StepMarkUnavailable StepKind = "mark_unavailable"
)

type PlanKind string

const (
	PlanRecovery  PlanKind = "recovery"
	PlanRebalance PlanKind = "rebalance"
)

type PlanStep struct {
	Kind   StepKind `json:"kind"`
	Slot   uint32   `json:"slot"`
	Target string   `json:"target"`
}

type RecoveryPlan struct {
	ID                string       `json:"id"`
	Kind              PlanKind     `json:"kind"`
	Reason            string       `json:"reason"`
	Epoch             uint64       `json:"epoch"`
	DeadNodes         []string     `json:"deadNodes"`
	Steps             []PlanStep   `json:"steps"`
	Done              map[int]bool `json:"done"`
	Unrecoverable     []uint32     `json:"unrecoverable"`
	WriteBlockedSlots []uint32     `json:"writeBlockedSlots,omitempty"`
}

type CommandType string

const (
	CommandObserveView        CommandType = "observe_view"
	CommandProposePlan        CommandType = "propose_plan"
	CommandStepDone           CommandType = "step_done"
	CommandPlanComplete       CommandType = "plan_complete"
	CommandSupersedePlan      CommandType = "supersede_plan"
	CommandProposeRebalance   CommandType = "propose_rebalance"
	CommandMarkUnavailable    CommandType = "mark_unavailable"
	CommandAdmitReturningNode CommandType = "admit_returning_node"
)

type Command struct {
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func NewCommand(commandType CommandType, payload any) (Command, error) {
	raw, err := json.Marshal(payload)
	return Command{Type: commandType, Payload: raw}, err
}

type StepDonePayload struct {
	PlanID    string `json:"planId"`
	StepIndex int    `json:"stepIndex"`
}

type PlanIDPayload struct {
	PlanID string `json:"planId"`
}

type SupersedePlanPayload struct {
	OldPlanID string       `json:"oldPlanId"`
	NewPlan   RecoveryPlan `json:"newPlan"`
}

type ReturningNodePayload struct {
	NodeID   string `json:"nodeId"`
	Admitted bool   `json:"admitted"`
}

type UnavailableSlot struct {
	Slot      uint32   `json:"slot"`
	LeaderID  string   `json:"leaderId"`
	ReplicaID string   `json:"replicaId"`
	Failures  []string `json:"failures"`
}

type FSMSnapshot struct {
	LatestView     ClusterView                `json:"latestView"`
	ActivePlan     *RecoveryPlan              `json:"activePlan,omitempty"`
	ControlIndex   uint64                     `json:"controlIndex"`
	Unavailable    map[uint32]UnavailableSlot `json:"unavailable"`
	ReturningNodes map[string]bool            `json:"returningNodes"`
}
