package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
)

func TestServerEnforcesMaxConnections(t *testing.T) {
	port := freePort(t)
	cfg := config.NetworkConfig{
		BindAddr:       "127.0.0.1",
		Port:           port,
		MaxConnections: 1,
		ReadTimeoutMs:  30000,
		WriteTimeoutMs: 30000,
	}
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	srv := New(cfg, eng, metrics.NewNoop())

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	waitForServer(t, addr)

	first, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	waitForActiveConns(t, srv, 1)

	second, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	line, err := bufio.NewReader(second).ReadString('\n')
	if err != nil {
		t.Fatalf("expected max-connection error, got read error: %v", err)
	}
	if !strings.Contains(line, "max connections reached") {
		t.Fatalf("unexpected response: %q", line)
	}
}

func TestServerUsesCanonicalCommandMetric(t *testing.T) {
	port := freePort(t)
	cfg := config.NetworkConfig{
		BindAddr:       "127.0.0.1",
		Port:           port,
		MaxConnections: 4,
		ReadTimeoutMs:  30000,
		WriteTimeoutMs: 30000,
	}
	sink := metrics.NewInMemory(64)
	eng := engine.NewWithMetrics(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"}, sink)
	srv := New(cfg, eng, sink)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	waitForServer(t, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "PONG") {
		t.Fatalf("unexpected response: %q", line)
	}
	if got := sink.Counter("cmd.total"); got != 1 {
		t.Fatalf("cmd.total=%d, want 1", got)
	}
	if got := sink.Counter("commands_total"); got != 0 {
		t.Fatalf("commands_total=%d, want 0", got)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start on %s", addr)
}

func waitForActiveConns(t *testing.T, srv *Server, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.activeConns() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("active connections = %d, want %d", srv.activeConns(), want)
}
