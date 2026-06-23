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
	ControlPlane  ControlPlaneConfig  `yaml:"controlPlane"`
	Persistence   PersistenceConfig   `yaml:"persistence"`
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

// ClusterConfig describes the fixed-slot cluster topology.
type ClusterConfig struct {
	ID                 string           `yaml:"id"`
	Enabled            bool             `yaml:"enabled"`
	ShardingEnabled    bool             `yaml:"shardingEnabled"`
	ReplicationEnabled bool             `yaml:"replicationEnabled"`
	SlotCount          uint32           `yaml:"slotCount"`
	RoutingMode        string           `yaml:"routingMode"`
	FailoverMode       string           `yaml:"failoverMode"`
	Controller         ControllerConfig `yaml:"controller"`
	Peers              []PeerConfig     `yaml:"peers"`
}

// ControllerConfig configures the embedded Raft controller. It is ignored in
// manual mode and validated as a complete unit in automatic mode.
type ControllerConfig struct {
	ControlPort            int    `yaml:"controlPort"`
	RaftDir                string `yaml:"raftDir"`
	BootstrapNodeID        string `yaml:"bootstrapNodeId"`
	ObserveIntervalMs      int    `yaml:"observeIntervalMs"`
	FailureTimeoutMs       int    `yaml:"failureTimeoutMs"`
	ConsecutiveFailures    int    `yaml:"consecutiveFailures"`
	RebalanceSkewThreshold int    `yaml:"rebalanceSkewThreshold"`
	MigrationRateLimit     int    `yaml:"migrationRateLimit"`
}

// ControlPlaneConfig contains node-side credentials used to authenticate
// controller administration calls. It is intentionally separate from data
// snapshots and the ordinary cluster metadata model.
type ControlPlaneConfig struct {
	RequestSigningSecret string `yaml:"requestSigningSecret"`
}

// PersistenceConfig governs periodic and manual engine snapshots.
type PersistenceConfig struct {
	Enabled             bool   `yaml:"enabled"`
	DataDir             string `yaml:"dataDir"`
	SnapshotIntervalSec int    `yaml:"snapshotIntervalSec"`
	MaxSnapshots        int    `yaml:"maxSnapshots"`
	LoadOnStart         bool   `yaml:"loadOnStart"`
	Format              string `yaml:"format"`
}

// PeerConfig identifies one peer in a static cluster definition.
type PeerConfig struct {
	ID             string `yaml:"id"`
	Address        string `yaml:"address"`
	APIAddress     string `yaml:"apiAddress"`
	ControlAddress string `yaml:"controlAddress"`
	FailoverMode   string `yaml:"failoverMode"`
}

// ObservabilityConfig governs the HTTP API and logging.
type ObservabilityConfig struct {
	APIBindAddr string `yaml:"apiBindAddr"`
	APIPort     int    `yaml:"apiPort"`
	LogLevel    string `yaml:"logLevel"`
}

// IsStandalone reports whether the node runs without cluster coordination.
func (c *Config) IsStandalone() bool { return !c.Cluster.Enabled }

// IsClustered reports whether any cluster coordination is enabled.
func (c *Config) IsClustered() bool { return c.Cluster.Enabled }
