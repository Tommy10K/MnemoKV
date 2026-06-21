package cluster

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/server"
)

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func replicationPair(t *testing.T, memoryLimit uint64, policy string) (*Manager, *engine.Engine, *Manager, *engine.Engine) {
	t.Helper()
	leaderAddress := reserveAddress(t)
	replicaAddress := reserveAddress(t)
	peers := []config.PeerConfig{
		{ID: "leader", Address: leaderAddress, APIAddress: "127.0.0.1:1"},
		{ID: "replica", Address: replicaAddress, APIAddress: "127.0.0.1:2"},
	}
	cfg := metadataTestConfig(peers)
	leader := NewManagerWithNode(cfg, "leader")
	replica := NewManagerWithNode(cfg, "replica")
	leaderEngine := engine.New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: memoryLimit, EvictionPolicy: policy})
	replicaEngine := engine.New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: memoryLimit, EvictionPolicy: policy})
	leader.AttachEngine(leaderEngine)
	replica.AttachEngine(replicaEngine)

	host, portText, _ := net.SplitHostPort(replicaAddress)
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	srv := server.New(config.NetworkConfig{BindAddr: host, Port: port, MaxConnections: 32}, replica.Coordinator(), metrics.NewNoop())
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		_ = srv.Shutdown(context.Background())
		_ = leader.Shutdown(context.Background())
		_ = replica.Shutdown(context.Background())
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("tcp", replicaAddress); err == nil {
			_ = conn.Close()
			return leader, leaderEngine, replica, replicaEngine
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("replica did not start")
	return nil, nil, nil, nil
}

func keyLedBy(t *testing.T, metadata *Metadata, leader string, start int) string {
	t.Helper()
	for i := start; i < start+100000; i++ {
		key := fmt.Sprintf("key:%d", i)
		slot := metadata.SlotForKey([]byte(key))
		state, _ := metadata.Slot(slot)
		if state.LeaderID == leader {
			return key
		}
	}
	t.Fatalf("no key found for leader %s", leader)
	return ""
}

func TestSynchronousReplicationAcknowledgesExactReplica(t *testing.T) {
	leader, leaderEngine, _, replicaEngine := replicationPair(t, 0, "noeviction")
	key := keyLedBy(t, leader.Metadata(), "leader", 0)
	frame := leader.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(key), []byte("value")}})
	if frame != resp.OK {
		t.Fatalf("leader response = %#v", frame)
	}
	if _, ok := leaderEngine.Store().Peek([]byte(key)); !ok {
		t.Fatal("leader missing acknowledged key")
	}
	entry, ok := replicaEngine.Store().Peek([]byte(key))
	if !ok || string(entry.Value.(*engine.StringValue).Data) != "value" {
		t.Fatal("replica missing acknowledged key")
	}
}

func TestReplicationRejectsStaleDuplicateAndGapRecords(t *testing.T) {
	_, _, replica, replicaEngine := replicationPair(t, 0, "noeviction")
	key := keyLedBy(t, replica.Metadata(), "leader", 0)
	slot := replica.Metadata().SlotForKey([]byte(key))
	state, _ := replica.Metadata().Slot(slot)
	record := ReplicationRecord{SourceNodeID: "leader", Slot: slot, Term: state.Term, Sequence: 1, Args: []string{"SET", key, "one"}}
	if err := replica.ApplyReplication(record); err != nil {
		t.Fatal(err)
	}
	if err := replica.ApplyReplication(record); err != nil {
		t.Fatalf("duplicate should be idempotent: %v", err)
	}
	record.Sequence = 3
	if err := replica.ApplyReplication(record); err != ErrSequenceGap {
		t.Fatalf("gap error = %v", err)
	}
	record.Sequence = 2
	record.Term--
	if err := replica.ApplyReplication(record); err != ErrStaleTerm {
		t.Fatalf("stale term error = %v", err)
	}
	entry, _ := replicaEngine.Store().Peek([]byte(key))
	if string(entry.Value.(*engine.StringValue).Data) != "one" {
		t.Fatal("rejected records changed data")
	}
}

func TestReplicaFailureRejectsWriteBeforeLeaderMutation(t *testing.T) {
	missingAddress := reserveAddress(t)
	peers := []config.PeerConfig{{ID: "leader", Address: reserveAddress(t), APIAddress: "127.0.0.1:1"}, {ID: "replica", Address: missingAddress, APIAddress: "127.0.0.1:2"}}
	leader := NewManagerWithNode(metadataTestConfig(peers), "leader")
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	leader.AttachEngine(eng)
	defer leader.Shutdown(context.Background())
	key := keyLedBy(t, leader.Metadata(), "leader", 0)
	frame := leader.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(key), []byte("value")}})
	if _, ok := frame.(resp.Error); !ok {
		t.Fatalf("expected error, got %#v", frame)
	}
	if _, ok := eng.Store().Peek([]byte(key)); ok {
		t.Fatal("leader mutated before replica acknowledgement")
	}
}

func TestLeaderChosenEvictionConverges(t *testing.T) {
	leader, leaderEngine, _, replicaEngine := replicationPair(t, 180, "fifo")
	keys := []string{keyLedBy(t, leader.Metadata(), "leader", 0), keyLedBy(t, leader.Metadata(), "leader", 1000), keyLedBy(t, leader.Metadata(), "leader", 2000)}
	for i, key := range keys {
		value := strings.Repeat(string(rune('a'+i)), 20)
		frame := leader.Coordinator().Execute(&resp.Command{Name: "SET", Args: [][]byte{[]byte(key), []byte(value)}})
		if frame != resp.OK {
			t.Fatalf("SET %s: %#v", key, frame)
		}
	}
	for _, key := range keys {
		_, leaderHas := leaderEngine.Store().Peek([]byte(key))
		_, replicaHas := replicaEngine.Store().Peek([]byte(key))
		if leaderHas != replicaHas {
			t.Fatalf("key-set mismatch for %s", key)
		}
	}
}
