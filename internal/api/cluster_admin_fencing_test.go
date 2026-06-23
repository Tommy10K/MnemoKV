package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/controlplane"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
)

func TestAutomaticClusterAdminAuthenticationAndFencing(t *testing.T) {
	server := newClusterAdminTestServer(t, "automatic", t.TempDir())
	body := []byte(`{"slot":0}`)

	response := callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 0, "", "")
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "managed automatically") {
		t.Fatalf("unsigned response = %d %s", response.Code, response.Body.String())
	}
	response = callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 10, "wrong-secret", "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("forged response = %d %s", response.Code, response.Body.String())
	}

	response = callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 10, "secret", "")
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated response = %d %s", response.Code, response.Body.String())
	}
	version := server.cluMgr.Metadata().Snapshot().Version
	response = callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 10, "secret", "")
	if response.Code != http.StatusOK || server.cluMgr.Metadata().Snapshot().Version != version {
		t.Fatalf("exact replay was not idempotent: status=%d version=%d", response.Code, server.cluMgr.Metadata().Snapshot().Version)
	}

	conflictingBody := []byte(`{"slot":1}`)
	response = callClusterAdmin(server.handleClusterPromote, "/cluster/promote", conflictingBody, 10, "secret", "")
	if response.Code != http.StatusConflict {
		t.Fatalf("same-index conflict = %d %s", response.Code, response.Body.String())
	}
	response = callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 9, "secret", "")
	if response.Code != http.StatusConflict {
		t.Fatalf("stale index = %d %s", response.Code, response.Body.String())
	}

	assignBody := []byte(`{"slot":0,"nodeId":"node-3"}`)
	response = callClusterAdmin(server.handleClusterReplica, "/cluster/replica", assignBody, 11, "secret", "")
	if response.Code != http.StatusOK {
		t.Fatalf("higher index = %d %s", response.Code, response.Body.String())
	}
}

func TestAutomaticModeRejectsUnsignedMutationsButKeepsStatusReadable(t *testing.T) {
	server := newClusterAdminTestServer(t, "automatic", t.TempDir())
	tests := []struct {
		path    string
		body    string
		handler http.HandlerFunc
	}{
		{path: "/cluster/promote", body: `{"slot":0}`, handler: server.handleClusterPromote},
		{path: "/cluster/replica", body: `{"slot":0,"nodeId":"node-3"}`, handler: server.handleClusterReplica},
		{path: "/cluster/sync", body: `{"slot":0,"nodeId":"node-2"}`, handler: server.handleClusterSync},
	}
	for _, test := range tests {
		response := callClusterAdmin(test.handler, test.path, []byte(test.body), 0, "", "")
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "managed automatically") {
			t.Fatalf("%s unsigned response = %d %s", test.path, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/cluster/state", nil)
	response := httptest.NewRecorder()
	server.handleClusterState(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("read-only cluster status requires controller auth: %d", response.Code)
	}
}

func TestAutomaticControlIndexPersistsAcrossAPIRestart(t *testing.T) {
	directory := t.TempDir()
	first := newClusterAdminTestServer(t, "automatic", directory)
	body := []byte(`{"slot":0}`)
	if response := callClusterAdmin(first.handleClusterPromote, "/cluster/promote", body, 20, "secret", ""); response.Code != http.StatusOK {
		t.Fatalf("initial request = %d %s", response.Code, response.Body.String())
	}

	second := newClusterAdminTestServer(t, "automatic", directory)
	if response := callClusterAdmin(second.handleClusterPromote, "/cluster/promote", body, 19, "secret", ""); response.Code != http.StatusConflict {
		t.Fatalf("restart accepted stale index: %d %s", response.Code, response.Body.String())
	}
	if response := callClusterAdmin(second.handleClusterPromote, "/cluster/promote", body, 20, "secret", ""); response.Code != http.StatusOK {
		t.Fatalf("restart rejected exact replay: %d %s", response.Code, response.Body.String())
	}
}

func TestManualClusterAdminRemainsUnsignedAndIgnoresControllerHeaders(t *testing.T) {
	server := newClusterAdminTestServer(t, "manual", "")
	body := []byte(`{"slot":0}`)
	response := callClusterAdmin(server.handleClusterPromote, "/cluster/promote", body, 1, "forged", "not-a-real-signature")
	if response.Code != http.StatusOK {
		t.Fatalf("manual promotion changed behavior: %d %s", response.Code, response.Body.String())
	}
}

func TestNoRuntimeFailoverModeEndpoint(t *testing.T) {
	server := newTestServer()
	mux := http.NewServeMux()
	server.registerRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/cluster/failover-mode", bytes.NewReader([]byte(`{"mode":"automatic"}`)))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected runtime mode endpoint status: %d", response.Code)
	}
}

func newClusterAdminTestServer(t *testing.T, mode, raftDir string) *Server {
	t.Helper()
	peers := []config.PeerConfig{
		{ID: "node-1", Address: "127.0.0.1:1", APIAddress: "127.0.0.1:11"},
		{ID: "node-2", Address: "127.0.0.1:2", APIAddress: "127.0.0.1:12"},
		{ID: "node-3", Address: "127.0.0.1:3", APIAddress: "127.0.0.1:13"},
	}
	clusterConfig := config.ClusterConfig{
		ID: "api-fencing", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true,
		SlotCount: 3, RoutingMode: "proxy", FailoverMode: mode, Peers: peers,
		Controller: config.ControllerConfig{RaftDir: raftDir},
	}
	manager := cluster.NewManagerWithNode(clusterConfig, "node-1")
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	manager.AttachEngine(eng)
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	return New(config.ObservabilityConfig{}, config.NodeConfig{ID: "node-1", Mode: "clustered"}, clusterConfig,
		config.ControlPlaneConfig{RequestSigningSecret: "secret"}, eng, metrics.NewInMemory(16), manager, nil)
}

func callClusterAdmin(handler http.HandlerFunc, path string, body []byte, index uint64, secret, explicitSignature string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if index > 0 {
		indexText := strconv.FormatUint(index, 10)
		request.Header.Set(controlplane.ControlIndexHeader, indexText)
		signature := explicitSignature
		if signature == "" {
			signature = controlplane.Sign([]byte(secret), http.MethodPost, path, body, indexText)
		}
		request.Header.Set(controlplane.ControlSignatureHeader, signature)
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}

func decodeAdminResponse(t *testing.T, response *httptest.ResponseRecorder) ClusterAdminResponse {
	t.Helper()
	var result ClusterAdminResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
