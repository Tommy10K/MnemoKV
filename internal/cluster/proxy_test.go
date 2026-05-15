package cluster

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/server"
)

func startNode(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := config.NetworkConfig{BindAddr: "127.0.0.1", Port: port, MaxConnections: 32}
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
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
	t.Fatalf("node did not start at %s", addr)
	return addr
}

func TestProxyForward(t *testing.T) {
	addr := startNode(t)
	proxy := NewRESPProxy(map[string]string{"peer": addr}, time.Second)
	defer proxy.Close()

	cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("hello"), []byte("world")}}
	frame, err := proxy.Forward(context.Background(), "peer", cmd)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := frame.(resp.SimpleString); !ok || string(s) != "OK" {
		t.Fatalf("expected OK, got %T %v", frame, frame)
	}

	getCmd := &resp.Command{Name: "GET", Args: [][]byte{[]byte("hello")}}
	frame, err = proxy.Forward(context.Background(), "peer", getCmd)
	if err != nil {
		t.Fatal(err)
	}
	bs, ok := frame.(resp.BulkString)
	if !ok || string(bs.Value) != "world" {
		t.Fatalf("expected bulk world, got %T %v", frame, frame)
	}
}

func TestProxyUnknownPeer(t *testing.T) {
	proxy := NewRESPProxy(map[string]string{}, time.Second)
	defer proxy.Close()
	cmd := &resp.Command{Name: "PING"}
	if _, err := proxy.Forward(context.Background(), "nope", cmd); err == nil {
		t.Fatal("expected error for unknown peer")
	}
}
