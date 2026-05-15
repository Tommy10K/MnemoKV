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

func startEngineNode(t *testing.T) (*engine.Engine, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	srv := server.New(config.NetworkConfig{
		BindAddr: "127.0.0.1", Port: port, MaxConnections: 32,
	}, eng, metrics.NewNoop())

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
			return eng, addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("node did not start at %s", addr)
	return nil, ""
}

func TestAsyncReplicationAppliesOnFollower(t *testing.T) {
	followerEng, followerAddr := startEngineNode(t)

	leaderCfg := config.ClusterConfig{
		Enabled:            true,
		ShardingEnabled:    true,
		ReplicationEnabled: true,
		WriteSafetyMode:    "async",
		Peers: []config.PeerConfig{
			{ID: "leader", Address: "127.0.0.1:0"},
			{ID: "follower", Address: followerAddr},
		},
	}
	leaderEng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	mgr := NewManagerWithNode(leaderCfg, "leader")
	mgr.AttachEngine(leaderEng)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer mgr.Shutdown(context.Background())

	cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("foo"), []byte("bar")}}
	if frame := leaderEng.Execute(cmd); frame != resp.OK {
		t.Fatalf("expected OK on leader, got %v", frame)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e, ok := followerEng.Store().Get([]byte("foo")); ok {
			sv, _ := e.Value.(*engine.StringValue)
			if sv != nil && string(sv.Data) == "bar" {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("follower never received replicated SET")
}

func TestStrongReplicationSyncRequiresFollower(t *testing.T) {
	followerEng, followerAddr := startEngineNode(t)

	leaderCfg := config.ClusterConfig{
		Enabled:            true,
		ShardingEnabled:    true,
		ReplicationEnabled: true,
		WriteSafetyMode:    "strong",
		Peers: []config.PeerConfig{
			{ID: "leader", Address: "127.0.0.1:0"},
			{ID: "follower", Address: followerAddr},
		},
	}
	leaderEng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	mgr := NewManagerWithNode(leaderCfg, "leader")
	mgr.AttachEngine(leaderEng)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer mgr.Shutdown(context.Background())

	cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("k"), []byte("v")}}
	if frame := leaderEng.Execute(cmd); frame != resp.OK {
		t.Fatalf("expected OK, got %v", frame)
	}

	if e, ok := followerEng.Store().Get([]byte("k")); !ok {
		t.Fatal("follower missing key after strong write")
	} else {
		sv, _ := e.Value.(*engine.StringValue)
		if sv == nil || string(sv.Data) != "v" {
			t.Fatalf("unexpected value: %+v", sv)
		}
	}
}
