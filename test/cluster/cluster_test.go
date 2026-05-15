package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
)

func newClusterConfig(nodeID string) config.ClusterConfig {
	return config.ClusterConfig{
		Enabled:            true,
		ShardingEnabled:    true,
		ReplicationEnabled: true,
		AutoFailover:       true,
		WriteSafetyMode:    "async",
		Peers: []config.PeerConfig{
			{ID: "node-1", Address: "127.0.0.1:6381"},
			{ID: "node-2", Address: "127.0.0.1:6382"},
			{ID: "node-3", Address: "127.0.0.1:6383"},
		},
	}
}

func TestManagerStartShutdown(t *testing.T) {
	cfg := newClusterConfig("node-1")
	m := cluster.NewManagerWithNode(cfg, "node-1")
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	m.AttachEngine(eng)

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := m.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestRouterResolvesLocal(t *testing.T) {
	cfg := newClusterConfig("node-1")
	m := cluster.NewManagerWithNode(cfg, "node-1")
	router := m.Router()
	if router == nil {
		t.Fatal("router is nil")
	}
	route := router.Resolve([]byte("test-key"))
	if route.OwnerNodeID == "" {
		t.Fatal("owner is empty")
	}
}

func TestControlPlaneSeededOnStart(t *testing.T) {
	cfg := newClusterConfig("node-1")
	m := cluster.NewManagerWithNode(cfg, "node-1")
	cp := m.ControlPlane()
	if cp == nil {
		t.Fatal("control plane is nil with autoFailover enabled")
	}
	_, term, ok := cp.LeaderForSlot(0)
	if !ok {
		t.Fatal("no leader for slot 0 after seeding")
	}
	if term != 0 {
		t.Fatalf("expected initial term 0, got %d", term)
	}
}

func TestReplicatorMode(t *testing.T) {
	cfg := newClusterConfig("node-1")
	m := cluster.NewManagerWithNode(cfg, "node-1")
	rep := m.Replicator()
	if rep == nil {
		t.Fatal("replicator is nil")
	}
	if rep.Mode() != "async" {
		t.Fatalf("expected mode async, got %s", rep.Mode())
	}
}
