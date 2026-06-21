package workload

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/server"
)

func startTestServer(t *testing.T) string {
	t.Helper()
	port := freePort(t)
	cfg := config.NetworkConfig{
		BindAddr:       "127.0.0.1",
		Port:           port,
		MaxConnections: 64,
	}
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	srv := server.New(cfg, eng, metrics.NewNoop())

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start on %s", addr)
	return addr
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestWorkloadRun(t *testing.T) {
	addr := startTestServer(t)
	res, err := Run(context.Background(), RunOptions{
		Address:     addr,
		Profile:     StringProfile(),
		Concurrency: 4,
		Duration:    300 * time.Millisecond,
		KeySpan:     50,
		Seed:        1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalOps == 0 {
		t.Fatalf("expected ops > 0, got %+v", res)
	}
	if res.Errors > 0 {
		t.Fatalf("unexpected errors: %+v", res)
	}
}
