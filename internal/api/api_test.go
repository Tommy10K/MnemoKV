package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/persistence"
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
		eng, sink, cluMgr, nil,
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

func TestReadHandlersRejectWrongMethods(t *testing.T) {
	s := newTestServer()
	cases := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/health", s.handleHealth},
		{"/engine/state", s.handleEngineState},
		{"/metrics/summary", s.handleMetricsSummary},
		{"/cluster/state", s.handleClusterState},
		{"/events", s.handleEvents},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		rr := httptest.NewRecorder()
		tc.handler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected 405, got %d", tc.path, rr.Code)
		}
		if allow := rr.Header().Get("Allow"); allow != http.MethodGet {
			t.Fatalf("%s: Allow=%q, want GET", tc.path, allow)
		}
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

func TestCommandsRejectTrailingJSON(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(`{"args":["PING"]} {"args":["PING"]}`))
	rr := httptest.NewRecorder()

	s.handleCommands(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body %s", rr.Code, rr.Body.String())
	}
}

func TestCommandsRejectOversizedBody(t *testing.T) {
	s := newTestServer()
	body := `{"args":["PING","` + strings.Repeat("x", int(maxJSONBodyBytes)+1) + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/commands", strings.NewReader(body))
	rr := httptest.NewRecorder()

	s.handleCommands(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body %s", rr.Code, rr.Body.String())
	}
}

func TestPostHandlersRejectWrongMethods(t *testing.T) {
	s := newTestServer()
	cases := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/commands", s.handleCommands},
		{"/engine/eviction-policy", s.handleEvictionPolicy},
		{"/admin/snapshot", s.handleSnapshot},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		tc.handler(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: expected 405, got %d", tc.path, rr.Code)
		}
		if allow := rr.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("%s: Allow=%q, want POST", tc.path, allow)
		}
	}
}

type fakeSnapshotter struct {
	result persistence.Result
	err    error
}

func (f fakeSnapshotter) Snapshot() (persistence.Result, error) { return f.result, f.err }

func TestManualSnapshot(t *testing.T) {
	s := newTestServer()
	s.snapshots = fakeSnapshotter{result: persistence.Result{
		Path: "snapshot.json", Format: "json", CreatedAt: time.Unix(1, 0).UTC(), EntryCount: 2, Checksum: "abc",
	}}
	req := httptest.NewRequest(http.MethodPost, "/admin/snapshot", nil)
	rr := httptest.NewRecorder()
	s.handleSnapshot(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var result persistence.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.EntryCount != 2 || result.Format != "json" {
		t.Fatalf("unexpected result: %+v", result)
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
