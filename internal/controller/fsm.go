package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

type FSM struct {
	mu    sync.RWMutex
	state FSMSnapshot
}

func NewFSM() *FSM {
	return &FSM{state: FSMSnapshot{
		Unavailable:    make(map[uint32]UnavailableSlot),
		ReturningNodes: make(map[string]bool),
	}}
}

func (f *FSM) Apply(log *raft.Log) any {
	var command Command
	if err := json.Unmarshal(log.Data, &command); err != nil {
		return fmt.Errorf("decode command: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	latestOperation := f.state.LatestView.Status.LatestCommittedOperation
	if err := f.applyCommand(command); err != nil {
		return err
	}
	f.state.ControlIndex = log.Index
	if command.Type == CommandPlanComplete && f.state.LastRebalance != nil && f.state.LastRebalance.ControlIndex == 0 {
		f.state.LastRebalance.ControlIndex = log.Index
	}
	if command.Type == CommandObserveView {
		f.state.LatestView.Status.LatestCommittedOperation = latestOperation
		if f.state.ActivePlan != nil {
			updatePlanStatus(&f.state)
		}
	} else {
		f.state.LatestView.Status.LatestCommittedOperation = string(command.Type)
	}
	return nil
}

func (f *FSM) applyCommand(command Command) error {
	switch command.Type {
	case CommandObserveView:
		return json.Unmarshal(command.Payload, &f.state.LatestView)
	case CommandProposePlan, CommandProposeRebalance:
		var plan RecoveryPlan
		if err := json.Unmarshal(command.Payload, &plan); err != nil {
			return err
		}
		if plan.Done == nil {
			plan.Done = make(map[int]bool)
		}
		f.state.ActivePlan = &plan
		updatePlanStatus(&f.state)
		return nil
	case CommandStepDone:
		var done StepDonePayload
		if err := json.Unmarshal(command.Payload, &done); err != nil {
			return err
		}
		if f.state.ActivePlan == nil || f.state.ActivePlan.ID != done.PlanID {
			return fmt.Errorf("active plan %q not found", done.PlanID)
		}
		if done.StepIndex < 0 || done.StepIndex >= len(f.state.ActivePlan.Steps) {
			return fmt.Errorf("step index %d out of range", done.StepIndex)
		}
		f.state.ActivePlan.Done[done.StepIndex] = true
		updatePlanStatus(&f.state)
		return nil
	case CommandPlanComplete:
		var payload PlanIDPayload
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return err
		}
		if f.state.ActivePlan == nil || f.state.ActivePlan.ID != payload.PlanID {
			return fmt.Errorf("active plan %q not found", payload.PlanID)
		}
		f.state.LastCompletedPlanID = payload.PlanID
		if f.state.ActivePlan.Kind == PlanKindRebalance {
			f.state.LastRebalance = &CompletedPlan{ID: f.state.ActivePlan.ID, Kind: f.state.ActivePlan.Kind, Epoch: f.state.ActivePlan.Epoch}
		}
		f.state.ActivePlan = nil
		return nil
	case CommandSupersedePlan:
		var payload SupersedePlanPayload
		if err := json.Unmarshal(command.Payload, &payload); err != nil {
			return err
		}
		if f.state.ActivePlan == nil || f.state.ActivePlan.ID != payload.OldPlanID {
			return fmt.Errorf("active plan %q not found", payload.OldPlanID)
		}
		if payload.NewPlan.ID == "" {
			return fmt.Errorf("replacement plan id is empty")
		}
		if payload.NewPlan.Done == nil {
			payload.NewPlan.Done = make(map[int]bool)
		}
		f.state.ActivePlan = &payload.NewPlan
		updatePlanStatus(&f.state)
		return nil
	case CommandMarkUnavailable:
		var slots []UnavailableSlot
		if err := json.Unmarshal(command.Payload, &slots); err != nil {
			return err
		}
		for _, slot := range slots {
			f.state.Unavailable[slot.Slot] = slot
		}
		return nil
	case CommandAdmitReturningNode:
		var node ReturningNodePayload
		if err := json.Unmarshal(command.Payload, &node); err != nil {
			return err
		}
		if node.NodeID == "" {
			return fmt.Errorf("returning node id is empty")
		}
		f.state.ReturningNodes[node.NodeID] = node.Admitted
		return nil
	default:
		return fmt.Errorf("unknown command type %q", command.Type)
	}
}

func updatePlanStatus(state *FSMSnapshot) {
	if state.ActivePlan == nil {
		return
	}
	if state.ActivePlan.Kind == PlanKindRebalance {
		state.LatestView.Status.State = StatusRebalancing
		return
	}
	for index, step := range state.ActivePlan.Steps {
		if state.ActivePlan.Done[index] {
			continue
		}
		switch step.Kind {
		case StepPromote:
			state.LatestView.Status.State = StatusPromoting
		case StepAssignReplica, StepSync:
			state.LatestView.Status.State = StatusRepairing
		case StepMarkUnavailable:
			state.LatestView.Status.State = StatusPotentialDataLoss
		}
		return
	}
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	raw, err := json.Marshal(f.State())
	if err != nil {
		return nil, err
	}
	return &fsmSnapshot{data: raw}, nil
}

func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()
	var state FSMSnapshot
	if err := json.NewDecoder(reader).Decode(&state); err != nil {
		return err
	}
	if state.Unavailable == nil {
		state.Unavailable = make(map[uint32]UnavailableSlot)
	}
	if state.ReturningNodes == nil {
		state.ReturningNodes = make(map[string]bool)
	}
	f.mu.Lock()
	f.state = cloneState(state)
	f.mu.Unlock()
	return nil
}

func (f *FSM) State() FSMSnapshot {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return cloneState(f.state)
}

func cloneState(state FSMSnapshot) FSMSnapshot {
	raw, _ := json.Marshal(state)
	var clone FSMSnapshot
	_ = json.Unmarshal(raw, &clone)
	return clone
}

type fsmSnapshot struct{ data []byte }

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := sink.Write(s.data); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (*fsmSnapshot) Release() {}
