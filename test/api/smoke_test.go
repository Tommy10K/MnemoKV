package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/api"
	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
)

func TestHTTPAPISmoke(t *testing.T) {
	baseURL, stop := startAPI(t)
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		wantStatus  int
		wantAllowed string
	}{
		{name: "read endpoint wrong method", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed, wantAllowed: http.MethodGet},
		{name: "write endpoint wrong method", method: http.MethodGet, path: "/commands", wantStatus: http.StatusMethodNotAllowed, wantAllowed: http.MethodPost},
		{name: "trailing JSON", method: http.MethodPost, path: "/commands", body: `{"args":["PING"]} {"args":["PING"]}`, wantStatus: http.StatusBadRequest},
		{name: "oversized body", method: http.MethodPost, path: "/commands", body: `{"args":["PING","` + strings.Repeat("x", (1<<20)+1) + `"]}`, wantStatus: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, baseURL+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantAllowed != "" && resp.Header.Get("Allow") != tc.wantAllowed {
				t.Fatalf("Allow = %q, want %q", resp.Header.Get("Allow"), tc.wantAllowed)
			}
		})
	}

	stop()
	if _, err := client.Get(baseURL + "/health"); err == nil {
		t.Fatal("request unexpectedly succeeded after backend shutdown")
	}
}

func startAPI(t *testing.T) (string, func()) {
	t.Helper()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	sink := metrics.NewInMemory(64)
	eng := engine.NewWithMetrics(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"}, sink)
	clusterManager := cluster.NewManager(config.ClusterConfig{})
	server := api.New(
		config.ObservabilityConfig{APIBindAddr: "127.0.0.1", APIPort: port},
		config.NodeConfig{ID: "api-smoke", Mode: "standalone"},
		config.ClusterConfig{}, eng, sink, clusterManager, nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Start(ctx) }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, err := http.Post(baseURL+"/commands", "application/json", bytes.NewBufferString(`{"args":["PING"]}`))
	if err != nil {
		cancel()
		t.Fatalf("API did not become ready: %v", err)
	}
	_ = resp.Body.Close()

	var once bool
	stop := func() {
		if once {
			return
		}
		once = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("API shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("API did not stop")
		}
	}
	t.Cleanup(stop)
	return baseURL, stop
}
