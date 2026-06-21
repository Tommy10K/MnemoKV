package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStandalone(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "standalone.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.IsStandalone() {
		t.Fatalf("expected standalone")
	}
	if cfg.Network.Port == 0 {
		t.Fatalf("port not loaded")
	}
	if cfg.Persistence.DataDir != cfg.Node.DataDir || cfg.Persistence.Format != "json" {
		t.Fatalf("unexpected persistence defaults: %+v", cfg.Persistence)
	}
}

func TestLoadStandalonePresets(t *testing.T) {
	tests := []struct {
		name              string
		file              string
		memoryLimit       uint64
		persistenceFormat string
	}{
		{name: "low memory", file: "standalone-low-memory.yaml", memoryLimit: 512},
		{name: "JSON persistence", file: "standalone-persistence-json.yaml", persistenceFormat: "json"},
		{name: "binary persistence", file: "standalone-persistence-binary.yaml", persistenceFormat: "binary"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(filepath.Join("..", "..", "configs", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if !cfg.IsStandalone() || cfg.Engine.MemoryLimitBytes != tc.memoryLimit {
				t.Fatalf("unexpected preset: %+v", cfg)
			}
			if tc.persistenceFormat == "" {
				if cfg.Persistence.Enabled {
					t.Fatal("low-memory preset unexpectedly enables persistence")
				}
				return
			}
			if !cfg.Persistence.Enabled || cfg.Persistence.Format != tc.persistenceFormat || !cfg.Persistence.LoadOnStart {
				t.Fatalf("unexpected persistence preset: %+v", cfg.Persistence)
			}
		})
	}
}

func TestValidatePersistence(t *testing.T) {
	base := Config{
		Node:        NodeConfig{ID: "n", DataDir: "./data"},
		Network:     NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:      EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster:     ClusterConfig{},
		Persistence: PersistenceConfig{Enabled: true, DataDir: "./data", SnapshotIntervalSec: 1, MaxSnapshots: 1, Format: "BINARY"},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("validate persistence: %v", err)
	}
	if base.Persistence.Format != "binary" {
		t.Fatalf("format = %q, want binary", base.Persistence.Format)
	}

	base.Persistence.Format = "yaml"
	if err := base.Validate(); err == nil {
		t.Fatal("expected unsupported persistence format to fail")
	}
}

func TestLoadCluster(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "cluster-node-1.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.IsClustered() {
		t.Fatalf("expected clustered")
	}
	if len(cfg.Cluster.Peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(cfg.Cluster.Peers))
	}
}

func TestLoadRejectsRemovedClusterOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "removed.yaml")
	raw := []byte("node:\n  id: n\nnetwork:\n  port: 6380\n  maxConnections: 10\nengine:\n  stripeCount: 4\n  evictionPolicy: noeviction\ncluster:\n  autoFailover: true\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected removed autoFailover option to be rejected")
	}
}

func TestValidateRejectsClusterFlagsWithoutEnabled(t *testing.T) {
	c := &Config{
		Node:    NodeConfig{ID: "n"},
		Network: NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:  EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster: ClusterConfig{ShardingEnabled: true},
	}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestValidateRequiresReplicationInClusterMode(t *testing.T) {
	c := &Config{
		Node:    NodeConfig{ID: "node-1"},
		Network: NetworkConfig{Port: 6381, MaxConnections: 10},
		Engine:  EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster: ClusterConfig{
			ID: "cluster", Enabled: true, ShardingEnabled: true, SlotCount: 1024,
			RoutingMode: "proxy", FailoverMode: "manual",
			Peers: []PeerConfig{
				{ID: "node-1", Address: "127.0.0.1:6381", APIAddress: "127.0.0.1:7381"},
				{ID: "node-2", Address: "127.0.0.1:6382", APIAddress: "127.0.0.1:7382"},
			},
		},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected cluster mode without replication to fail")
	}
}

func TestValidateRejectsUnknownLogLevel(t *testing.T) {
	c := &Config{
		Node:          NodeConfig{ID: "n"},
		Network:       NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:        EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster:       ClusterConfig{},
		Observability: ObservabilityConfig{LogLevel: "verbose"},
	}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestValidateNormalizesLogLevel(t *testing.T) {
	c := &Config{
		Node:          NodeConfig{ID: "n"},
		Network:       NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:        EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster:       ClusterConfig{},
		Observability: ObservabilityConfig{LogLevel: "WARN"},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.Observability.LogLevel != "warn" {
		t.Fatalf("log level = %q, want warn", c.Observability.LogLevel)
	}
}
