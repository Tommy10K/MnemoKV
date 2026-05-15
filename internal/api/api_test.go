package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
)

func newTestServer() *Server {
	sink := metrics.NewInMemory(64)
	eng := engine.NewWithMetrics(config.EngineConfig{
		StripeCount:    4,
		EvictionPolicy: "lru",
	}, sink)
	cluMgr := cluster.NewManager(config.ClusterConfig{})
	return New(
		config.ObservabilityConfig{},
		config.NodeConfig{ID: "test", Mode: "standalone"},
		config.ClusterConfig{},
		eng, sink, cluMgr,
	)
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" || resp.NodeID != "test" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestEngineState(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/engine/state", nil)
	rr := httptest.NewRecorder()
	s.handleEngineState(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp EngineStateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.EvictionPolicy != "lru" {
		t.Fatalf("unexpected policy: %s", resp.EvictionPolicy)
	}
}

func TestMetricsSummary(t *testing.T) {
	s := newTestServer()
	s.metrics.IncCounter("cmd.total")
	req := httptest.NewRequest(http.MethodGet, "/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.handleMetricsSummary(rr, req)

	var resp MetricsSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Counters["cmd.total"] != 1 {
		t.Fatalf("expected counter 1, got %d", resp.Counters["cmd.total"])
	}
}
