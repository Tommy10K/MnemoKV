package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mnemokv/mnemokv/internal/controlplane"
)

type fixedControllerStatus struct{ status controlplane.StatusSnapshot }

func (f fixedControllerStatus) StatusSnapshot() controlplane.StatusSnapshot { return f.status }

type fixedControllerState struct {
	state controlplane.ControllerStateSnapshot
}

func (f fixedControllerState) StateSnapshot() controlplane.ControllerStateSnapshot { return f.state }

func TestClusterAndEventPayloadsExposeRecoveryStatus(t *testing.T) {
	server := newTestServer()
	status := controlplane.StatusSnapshot{
		State: "potential_data_loss", FailedNodes: []string{"node-1", "node-2"},
		UnavailableSlots: []controlplane.SlotStatus{{
			Slot: 7, Classification: "no_surviving_copy", FormerLeaderID: "node-1", FormerReplicaID: "node-2",
			Failures: []string{"node-1", "node-2"}, Message: "slot unavailable — no authoritative copy currently reachable; data may be lost",
		}},
		Warning: "another failure before repair completes may cause slot unavailability or data loss",
	}
	server.SetControllerStatusProvider(fixedControllerStatus{status: status})

	request := httptest.NewRequest(http.MethodGet, "/cluster/state", nil)
	response := httptest.NewRecorder()
	server.handleClusterState(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cluster status = %d", response.Code)
	}
	var clusterState ClusterStateResponse
	if err := json.Unmarshal(response.Body.Bytes(), &clusterState); err != nil {
		t.Fatal(err)
	}
	if clusterState.Recovery == nil || clusterState.Recovery.State != "potential_data_loss" || len(clusterState.Recovery.UnavailableSlots) != 1 {
		t.Fatalf("cluster recovery status missing: %+v", clusterState.Recovery)
	}

	event := server.snapshotPayload()
	recovery, ok := event["recovery"].(controlplane.StatusSnapshot)
	if !ok || recovery.State != "potential_data_loss" {
		t.Fatalf("SSE snapshot recovery status missing: %#v", event["recovery"])
	}
}

func TestMetricsSummaryIncludesControllerGauges(t *testing.T) {
	server := newTestServer()
	server.metrics.Gauge("controller.unavailable_slots", 2)
	request := httptest.NewRequest(http.MethodGet, "/metrics/summary", nil)
	response := httptest.NewRecorder()
	server.handleMetricsSummary(response, request)
	var summary MetricsSummary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Gauges["controller.unavailable_slots"] != 2 {
		t.Fatalf("controller gauge missing: %+v", summary.Gauges)
	}
}

func TestControllerStateEndpointExposesRaftViewAndProgress(t *testing.T) {
	server := newClusterAdminTestServer(t, "automatic", t.TempDir())
	state := controlplane.ControllerStateSnapshot{
		NodeID: "node-1", RaftRole: "leader", RaftLeaderID: "node-1", RaftTerm: 4, IsLeader: true, ControlIndex: 22,
		CurrentView:   controlplane.ControllerView{ClusterID: "api-fencing", MetadataVersion: 9, Status: "repairing"},
		Recovery:      controlplane.StatusSnapshot{State: "repairing", ActivePlan: &controlplane.PlanStatus{ID: "plan-1", CompletedSteps: 2, TotalSteps: 4}},
		LastRebalance: &controlplane.CompletedPlanStatus{ID: "rebalance-1", Kind: "rebalance", Epoch: 8, ControlIndex: 18},
	}
	server.SetControllerStateProvider(fixedControllerState{state: state})
	request := httptest.NewRequest(http.MethodGet, "/controller/state", nil)
	response := httptest.NewRecorder()
	server.handleControllerState(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("controller state = %d %s", response.Code, response.Body.String())
	}
	var got controlplane.ControllerStateSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.IsLeader || got.RaftTerm != 4 || got.Recovery.ActivePlan == nil || got.LastRebalance == nil {
		t.Fatalf("controller state is incomplete: %+v", got)
	}
}

func TestControllerStateEndpointIsAbsentOutsideAutomaticMode(t *testing.T) {
	server := newClusterAdminTestServer(t, "manual", "")
	server.SetControllerStateProvider(fixedControllerState{})
	response := httptest.NewRecorder()
	server.handleControllerState(response, httptest.NewRequest(http.MethodGet, "/controller/state", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("manual controller endpoint = %d", response.Code)
	}
}
