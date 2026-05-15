package failover_test

import (
	"context"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func makeManager(nodeID string) *cluster.Manager {
	cfg := config.ClusterConfig{
		Enabled:            true,
		ShardingEnabled:    true,
		ReplicationEnabled: true,
		AutoFailover:       true,
		WriteSafetyMode:    "strong",
		Peers: []config.PeerConfig{
			{ID: "node-1", Address: "127.0.0.1:6381"},
			{ID: "node-2", Address: "127.0.0.1:6382"},
		},
	}
	return cluster.NewManagerWithNode(cfg, nodeID)
}

func TestStaleWriteRejection(t *testing.T) {
	m := makeManager("node-1")
	cp := m.ControlPlane()
	if cp == nil {
		t.Fatal("control plane not initialized")
	}

	if err := cp.ApplyLeader(100, "node-2", 5); err != nil {
		t.Fatalf("apply leader: %v", err)
	}

	err := cp.ValidateWriteTerm(100, 3)
	if err == nil {
		t.Fatal("expected rejection for stale term")
	}
}

func TestFailoverPromotesNewLeader(t *testing.T) {
	m := makeManager("node-1")
	cp := m.ControlPlane()
	if cp == nil {
		t.Fatal("control plane not initialized")
	}

	if err := cp.ApplyLeader(50, "node-2", 1); err != nil {
		t.Fatalf("apply leader: %v", err)
	}

	ctx := context.Background()
	winner, err := cp.BeginElection(ctx, 50)
	if err != nil {
		t.Fatalf("election: %v", err)
	}
	if winner == "" {
		t.Fatal("no winner elected")
	}
	if cp.CurrentTerm() <= 1 {
		t.Fatalf("term not advanced after election: %d", cp.CurrentTerm())
	}
}

func TestWriteHookRejectsWhenNotLeader(t *testing.T) {
	m := makeManager("node-1")
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	m.AttachEngine(eng)

	cp := m.ControlPlane()
	if cp == nil {
		t.Fatal("control plane not initialized")
	}

	// Assign leadership of all slots to node-2 at term 1.
	for slot := uint16(0); slot < 100; slot++ {
		_ = cp.ApplyLeader(slot, "node-2", 1)
	}

	cmd := &resp.Command{Name: "SET", Args: [][]byte{[]byte("key"), []byte("val")}}
	frame := eng.Execute(cmd)
	if _, ok := frame.(resp.Error); !ok {
		t.Fatalf("expected error frame for fenced write, got %T", frame)
	}
}

func TestElectionMonitorTriggersFailover(t *testing.T) {
	cfg := config.ClusterConfig{
		Enabled:            true,
		ShardingEnabled:    true,
		ReplicationEnabled: true,
		AutoFailover:       true,
		WriteSafetyMode:    "async",
		Peers: []config.PeerConfig{
			{ID: "node-1", Address: "127.0.0.1:6381"},
			{ID: "node-2", Address: "127.0.0.1:6382"},
		},
	}
	m := cluster.NewManagerWithNode(cfg, "node-1")
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noop"})
	m.AttachEngine(eng)

	cp := m.ControlPlane()
	_ = cp.ApplyLeader(200, "node-2", 1)

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Simulate node-2 becoming unavailable.
	mt := m.MembershipTable()
	mt.MarkFailed("node-2")
	mt.MarkFailed("node-2")

	time.Sleep(3 * time.Second)
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = m.Shutdown(shutdownCtx)

	leader, term, _ := cp.LeaderForSlot(200)
	if leader == "node-2" {
		t.Fatal("unavailable node-2 still leader after failover period")
	}
	if term <= 1 {
		t.Fatalf("term not advanced: %d", term)
	}
}
