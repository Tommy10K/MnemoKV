package api

import (
	"bytes"
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

func TestCommands(t *testing.T) {
	s := newTestServer()

	send := func(args ...string) commandResult {
		body, _ := json.Marshal(commandRequest{Args: args})
		req := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		s.handleCommands(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
		}
		var out commandResult
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	if got := send("SET", "k", "v"); got.Type != "string" || got.Value != "OK" {
		t.Fatalf("SET: %+v", got)
	}
	if got := send("GET", "k"); got.Type != "bulk" || got.Value != "v" {
		t.Fatalf("GET k: %+v", got)
	}
	if got := send("GET", "missing"); got.Type != "nil" {
		t.Fatalf("GET missing: %+v", got)
	}
	if got := send("NOPE"); got.Type != "error" {
		t.Fatalf("NOPE: %+v", got)
	}
}

func TestEvictionPolicy(t *testing.T) {
	s := newTestServer()

	switchTo := func(name string) evictionPolicyResponse {
		body, _ := json.Marshal(evictionPolicyRequest{Policy: name})
		req := httptest.NewRequest(http.MethodPost, "/engine/eviction-policy", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		s.handleEvictionPolicy(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
		}
		var out evictionPolicyResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	if got := switchTo("fifo"); got.Policy != "fifo" {
		t.Fatalf("switch to fifo: %+v", got)
	}
	if got := s.engine.Eviction().Policy().Name(); got != "fifo" {
		t.Fatalf("engine policy after switch: %s", got)
	}
	if got := switchTo("LRU"); got.Policy != "lru" {
		t.Fatalf("case insensitive switch: %+v", got)
	}

	// unknown policy must be rejected
	body, _ := json.Marshal(evictionPolicyRequest{Policy: "bogus"})
	req := httptest.NewRequest(http.MethodPost, "/engine/eviction-policy", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleEvictionPolicy(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown policy, got %d", rr.Code)
	}
}
