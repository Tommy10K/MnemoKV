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
