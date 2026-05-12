// Package config defines the runtime configuration model for a MnemoKV node.
//
// The model is intentionally explicit: every operating mode is selected by a
// fixed combination of feature flags rather than implied by an opaque "mode"
// string. The supported combinations are documented in
// docs/adr/001-system-modes.md and enforced by Validate.
package config

// Config is the root configuration object loaded from YAML at startup.
type Config struct {
	Node          NodeConfig          `yaml:"node"`
	Network       NetworkConfig       `yaml:"network"`
	Engine        EngineConfig        `yaml:"engine"`
	Cluster       ClusterConfig       `yaml:"cluster"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// NodeConfig identifies the node and its on-disk footprint.
type NodeConfig struct {
	ID      string `yaml:"id"`
	Mode    string `yaml:"mode"` // informational: standalone | clustered
	DataDir string `yaml:"dataDir"`
}

// NetworkConfig governs the RESP listener.
type NetworkConfig struct {
	BindAddr       string `yaml:"bindAddr"`
	Port           int    `yaml:"port"`
	MaxConnections int    `yaml:"maxConnections"`
	ReadTimeoutMs  int    `yaml:"readTimeoutMs"`
	WriteTimeoutMs int    `yaml:"writeTimeoutMs"`
}

// EngineConfig governs the in-memory store.
type EngineConfig struct {
	StripeCount      int    `yaml:"stripeCount"`
	MemoryLimitBytes uint64 `yaml:"memoryLimitBytes"`
	EvictionPolicy   string `yaml:"evictionPolicy"`
}

// ClusterConfig describes the cluster topology and write-safety choices.
// The baseline milestone accepts these fields but only exercises them once
// the cluster phases land.
type ClusterConfig struct {
	Enabled            bool         `yaml:"enabled"`
	ShardingEnabled    bool         `yaml:"shardingEnabled"`
	ReplicationEnabled bool         `yaml:"replicationEnabled"`
	AutoFailover       bool         `yaml:"autoFailover"`
	WriteSafetyMode    string       `yaml:"writeSafetyMode"`
	Peers              []PeerConfig `yaml:"peers"`
}

// PeerConfig identifies one peer in a static cluster definition.
type PeerConfig struct {
	ID      string `yaml:"id"`
	Address string `yaml:"address"`
}

// ObservabilityConfig governs the HTTP API and logging.
type ObservabilityConfig struct {
	APIBindAddr string `yaml:"apiBindAddr"`
	APIPort     int    `yaml:"apiPort"`
	LogLevel    string `yaml:"logLevel"`
}

// IsStandalone reports whether the node is configured to run without cluster
// coordination. This is the only mode the baseline milestone executes.
func (c *Config) IsStandalone() bool { return !c.Cluster.Enabled }

// IsClustered reports whether any cluster coordination is enabled.
func (c *Config) IsClustered() bool { return c.Cluster.Enabled }
