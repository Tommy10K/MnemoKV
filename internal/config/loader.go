package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads, parses, and validates a YAML configuration file. The returned
// Config is normalized: defaults are filled in for fields the operator omitted.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	cfg := defaults()
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("config: parse %q: multiple YAML documents are not supported", path)
		}
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	cfg.applyFallbacks()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validate %q: %w", path, err)
	}
	return cfg, nil
}

// defaults returns a config pre-populated with the values used when the YAML
// file omits a field. Validation still runs on top of the merged result.
func defaults() *Config {
	return &Config{
		Node: NodeConfig{
			ID:      "node-1",
			Mode:    "standalone",
			DataDir: "./data",
		},
		Network: NetworkConfig{
			BindAddr:       "127.0.0.1",
			Port:           6380,
			MaxConnections: 1024,
			ReadTimeoutMs:  30000,
			WriteTimeoutMs: 30000,
		},
		Engine: EngineConfig{
			StripeCount:      32,
			MemoryLimitBytes: 0,
			EvictionPolicy:   "lru",
		},
		Cluster: ClusterConfig{
			SlotCount:    1024,
			RoutingMode:  "proxy",
			FailoverMode: "manual",
			Controller: ControllerConfig{
				ObserveIntervalMs:      1000,
				FailureTimeoutMs:       10000,
				ConsecutiveFailures:    3,
				RebalanceSkewThreshold: 1,
				MigrationRateLimit:     10,
			},
		},
		Persistence: PersistenceConfig{
			SnapshotIntervalSec: 60,
			MaxSnapshots:        5,
			LoadOnStart:         true,
			Format:              "json",
		},
		Observability: ObservabilityConfig{
			APIBindAddr: "127.0.0.1",
			APIPort:     7380,
			LogLevel:    "info",
		},
	}
}

func (c *Config) applyFallbacks() {
	if c.Engine.StripeCount <= 0 {
		c.Engine.StripeCount = 32
	}
	if c.Engine.EvictionPolicy == "" {
		c.Engine.EvictionPolicy = "noeviction"
	}
	if c.Cluster.SlotCount == 0 {
		c.Cluster.SlotCount = 1024
	}
	if c.Cluster.RoutingMode == "" {
		c.Cluster.RoutingMode = "proxy"
	}
	if c.Cluster.FailoverMode == "" {
		c.Cluster.FailoverMode = "manual"
	}
	if c.Cluster.Controller.ObserveIntervalMs == 0 {
		c.Cluster.Controller.ObserveIntervalMs = 1000
	}
	if c.Cluster.Controller.FailureTimeoutMs == 0 {
		c.Cluster.Controller.FailureTimeoutMs = 10000
	}
	if c.Cluster.Controller.ConsecutiveFailures == 0 {
		c.Cluster.Controller.ConsecutiveFailures = 3
	}
	if c.Cluster.Controller.RebalanceSkewThreshold == 0 {
		c.Cluster.Controller.RebalanceSkewThreshold = 1
	}
	if c.Cluster.Controller.MigrationRateLimit == 0 {
		c.Cluster.Controller.MigrationRateLimit = 10
	}
	if c.Persistence.DataDir == "" {
		c.Persistence.DataDir = c.Node.DataDir
	}
	if c.Persistence.SnapshotIntervalSec == 0 {
		c.Persistence.SnapshotIntervalSec = 60
	}
	if c.Persistence.MaxSnapshots == 0 {
		c.Persistence.MaxSnapshots = 5
	}
	if c.Persistence.Format == "" {
		c.Persistence.Format = "json"
	}
	if c.Observability.LogLevel == "" {
		c.Observability.LogLevel = "info"
	}
}
