package config

import (
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

func TestValidatePersistence(t *testing.T) {
	base := Config{
		Node:        NodeConfig{ID: "n", DataDir: "./data"},
		Network:     NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:      EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster:     ClusterConfig{WriteSafetyMode: "async"},
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

func TestValidateRejectsAutoFailoverWithoutReplication(t *testing.T) {
	c := &Config{
		Node:    NodeConfig{ID: "n"},
		Network: NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:  EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster: ClusterConfig{
			Enabled:         true,
			ShardingEnabled: true,
			AutoFailover:    true,
			WriteSafetyMode: "async",
			Peers:           []PeerConfig{{ID: "n", Address: "127.0.0.1:1"}},
		},
	}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestValidateRejectsClusterFlagsWithoutEnabled(t *testing.T) {
	c := &Config{
		Node:    NodeConfig{ID: "n"},
		Network: NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:  EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster: ClusterConfig{ShardingEnabled: true, WriteSafetyMode: "async"},
	}
	if err := c.Validate(); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestValidateRejectsUnknownLogLevel(t *testing.T) {
	c := &Config{
		Node:          NodeConfig{ID: "n"},
		Network:       NetworkConfig{Port: 6380, MaxConnections: 10},
		Engine:        EngineConfig{StripeCount: 8, EvictionPolicy: "lru"},
		Cluster:       ClusterConfig{WriteSafetyMode: "async"},
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
		Cluster:       ClusterConfig{WriteSafetyMode: "async"},
		Observability: ObservabilityConfig{LogLevel: "WARN"},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.Observability.LogLevel != "warn" {
		t.Fatalf("log level = %q, want warn", c.Observability.LogLevel)
	}
}
