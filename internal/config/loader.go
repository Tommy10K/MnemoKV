package config

import (
	"fmt"
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
	if err := yaml.Unmarshal(raw, cfg); err != nil {
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
			WriteSafetyMode: "async",
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
	if c.Cluster.WriteSafetyMode == "" {
		c.Cluster.WriteSafetyMode = "async"
	}
	if c.Observability.LogLevel == "" {
		c.Observability.LogLevel = "info"
	}
}
