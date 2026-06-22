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
