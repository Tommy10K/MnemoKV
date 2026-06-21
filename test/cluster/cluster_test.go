package cluster_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/server"
	"github.com/mnemokv/mnemokv/internal/workload"
)

type runningNode struct {
	id      string
	address string
	manager *cluster.Manager
	engine  *engine.Engine
	server  *server.Server
	cancel  context.CancelFunc
	stopped bool
}

func (n *runningNode) stop() {
	if n.stopped {
		return
	}
	n.stopped = true
	n.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = n.server.Shutdown(ctx)
}

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

func startCluster(t *testing.T) ([]*runningNode, config.ClusterConfig) {
	t.Helper()
	addresses := []string{reserveAddress(t), reserveAddress(t), reserveAddress(t)}
	peers := make([]config.PeerConfig, 3)
	for i := range peers {
		peers[i] = config.PeerConfig{ID: fmt.Sprintf("node-%d", i+1), Address: addresses[i], APIAddress: fmt.Sprintf("127.0.0.1:%d", 7901+i)}
	}
	cfg := config.ClusterConfig{ID: "acceptance", Enabled: true, ShardingEnabled: true, ReplicationEnabled: true, SlotCount: 32, RoutingMode: "proxy", FailoverMode: "manual", Peers: peers}
	nodes := make([]*runningNode, 3)
	for i := range nodes {
		manager := cluster.NewManagerWithNode(cfg, peers[i].ID)
		eng := engine.New(config.EngineConfig{StripeCount: 8, EvictionPolicy: "noeviction"})
		manager.AttachEngine(eng)
		host, portText, _ := net.SplitHostPort(addresses[i])
		var port int
		_, _ = fmt.Sscanf(portText, "%d", &port)
		srv := server.New(config.NetworkConfig{BindAddr: host, Port: port, MaxConnections: 64}, manager.Coordinator(), metrics.NewNoop())
		ctx, cancel := context.WithCancel(context.Background())
		nodes[i] = &runningNode{id: peers[i].ID, address: addresses[i], manager: manager, engine: eng, server: srv, cancel: cancel}
		go func() { _ = srv.Start(ctx) }()
	}
	t.Cleanup(func() {
		for _, node := range nodes {
			node.stop()
			_ = node.manager.Shutdown(context.Background())
		}
	})
	for _, node := range nodes {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if conn, err := net.Dial("tcp", node.address); err == nil {
				_ = conn.Close()
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nodes, cfg
}

func keyFor(t *testing.T, metadata *cluster.Metadata, leader string, exactSlot *uint32, start int) string {
	t.Helper()
	for i := start; i < start+1000000; i++ {
		key := fmt.Sprintf("acceptance:%d", i)
		slot := metadata.SlotForKey([]byte(key))
		state, _ := metadata.Slot(slot)
		if state.LeaderID == leader && (exactSlot == nil || slot == *exactSlot) {
			return key
		}
	}
	t.Fatalf("no key found for leader=%s slot=%v", leader, exactSlot)
	return ""
}

func command(t *testing.T, address string, args ...string) (any, error) {
	t.Helper()
	client, err := workload.Dial(address, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.Do(args...)
}

func TestProxyRoutingStrictReplicationManualRepairAndRejoin(t *testing.T) {
	nodes, cfg := startCluster(t)
	metadata := nodes[0].manager.Metadata()
	key := keyFor(t, metadata, "node-1", nil, 0)
	slot := metadata.SlotForKey([]byte(key))

	if got, err := command(t, nodes[2].address, "SET", key, "before-failure"); err != nil || got != "OK" {
		t.Fatalf("write through gateway: got=%v err=%v", got, err)
	}
	for _, index := range []int{0, 1} {
		entry, ok := nodes[index].engine.Store().Peek([]byte(key))
		if !ok || string(entry.Value.(*engine.StringValue).Data) != "before-failure" {
			t.Fatalf("node-%d missing acknowledged data", index+1)
		}
	}
	if _, ok := nodes[2].engine.Store().Peek([]byte(key)); ok {
		t.Fatal("non-owner stored routed key")
	}
	if got, err := command(t, nodes[2].address, "GET", key); err != nil || got != "before-failure" {
		t.Fatalf("routed read: got=%v err=%v", got, err)
	}

	other := keyFor(t, metadata, "node-2", nil, 1000)
	if _, err := command(t, nodes[2].address, "DEL", key, other); err == nil || !strings.Contains(err.Error(), "CROSSSLOT") {
		t.Fatalf("cross-slot command error = %v", err)
	}

	// Losing the replica makes writes fail before the leader mutates.
	oldReplicaManager := nodes[1].manager
	nodes[1].stop()
	_ = oldReplicaManager.Shutdown(context.Background())
	rejectedKey := keyFor(t, metadata, "node-1", nil, 2000)
	if _, err := command(t, nodes[2].address, "SET", rejectedKey, "rejected"); err == nil {
		t.Fatal("write succeeded without replica")
	}
	if _, ok := nodes[0].engine.Store().Peek([]byte(rejectedKey)); ok {
		t.Fatal("leader mutated without replica acknowledgement")
	}

	// Bring the replica process back for the leader-failure and manual repair path.
	restartedManager := cluster.NewManagerWithNode(cfg, "node-2")
	restartedEngine := engine.New(config.EngineConfig{StripeCount: 8, EvictionPolicy: "noeviction"})
	restartedManager.AttachEngine(restartedEngine)
	host, portText, _ := net.SplitHostPort(nodes[1].address)
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	restartedServer := server.New(config.NetworkConfig{BindAddr: host, Port: port, MaxConnections: 64}, restartedManager.Coordinator(), metrics.NewNoop())
	restartCtx, restartCancel := context.WithCancel(context.Background())
	nodes[1] = &runningNode{id: "node-2", address: nodes[1].address, manager: restartedManager, engine: restartedEngine, server: restartedServer, cancel: restartCancel}
	go func() { _ = restartedServer.Start(restartCtx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("tcp", nodes[1].address); err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Restore the surviving replica's data as it would have loaded from disk.
	entries, _ := nodes[0].engine.SnapshotEntries()
	_, _ = restartedEngine.RestoreSnapshotEntries(entries, time.Now())

	nodes[0].stop()
	if _, err := command(t, nodes[2].address, "GET", key); err == nil || !strings.Contains(err.Error(), "CLUSTERDOWN") {
		t.Fatalf("failed leader read error = %v", err)
	}

	promoted, _, err := restartedManager.Promote(context.Background(), slot)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Slots[slot].LeaderID != "node-2" || promoted.Slots[slot].Term <= 1 {
		t.Fatalf("promotion failed: %+v", promoted.Slots[slot])
	}
	if _, _, err := restartedManager.AssignReplica(context.Background(), slot, "node-3"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := restartedManager.SyncReplica(context.Background(), slot, "node-3"); err != nil {
		t.Fatal(err)
	}

	repairedKey := keyFor(t, restartedManager.Metadata(), "node-2", &slot, 3000)
	if got, err := command(t, nodes[2].address, "SET", repairedKey, "after-repair"); err != nil || got != "OK" {
		t.Fatalf("write after repair: got=%v err=%v", got, err)
	}
	for _, node := range nodes[1:] {
		if _, ok := node.engine.Store().Peek([]byte(repairedKey)); !ok {
			t.Fatalf("%s missing repaired write", node.id)
		}
	}

	// A stale returning node fetches the newer metadata and cannot reclaim leadership.
	returning := cluster.NewManagerWithNode(cfg, "node-1")
	returning.AttachEngine(engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"}))
	if err := returning.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer returning.Shutdown(context.Background())
	returnedSlot, _ := returning.Metadata().Slot(slot)
	if returnedSlot.LeaderID != "node-2" || returnedSlot.Term != promoted.Slots[slot].Term+1 {
		t.Fatalf("returning node retained stale leadership: %+v", returnedSlot)
	}
}
