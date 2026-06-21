package config

import (
	"fmt"
	"strings"

	"github.com/mnemokv/mnemokv/internal/logging"
)

// validEvictionPolicies enumerates the policy names the engine recognizes.
// The set is intentionally small at the baseline milestone.
var validEvictionPolicies = map[string]struct{}{
	"noeviction": {},
	"fifo":       {},
	"lru":        {},
	"lfu":        {},
	"random":     {},
}

var validSnapshotFormats = map[string]struct{}{
	"json":   {},
	"binary": {},
}

// Validate enforces the cross-field invariants documented in
// docs/adr/001-system-modes.md. It returns a single error describing the first
// rule that was violated; configuration must be entirely valid before the node
// boots.
func (c *Config) Validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node.id must not be empty")
	}
	if c.Network.Port <= 0 || c.Network.Port > 65535 {
		return fmt.Errorf("network.port %d is out of range", c.Network.Port)
	}
	if c.Network.MaxConnections <= 0 {
		return fmt.Errorf("network.maxConnections must be positive")
	}
	logLevel := strings.ToLower(c.Observability.LogLevel)
	if logLevel == "" {
		logLevel = "info"
	}
	if logLevel == "warning" {
		logLevel = "warn"
	}
	if _, ok := logging.ParseLevel(logLevel); !ok {
		return fmt.Errorf("observability.logLevel %q is not supported", c.Observability.LogLevel)
	}
	c.Observability.LogLevel = logLevel

	if c.Engine.StripeCount <= 0 {
		return fmt.Errorf("engine.stripeCount must be positive")
	}
	policy := strings.ToLower(c.Engine.EvictionPolicy)
	if _, ok := validEvictionPolicies[policy]; !ok {
		return fmt.Errorf("engine.evictionPolicy %q is not supported", c.Engine.EvictionPolicy)
	}
	c.Engine.EvictionPolicy = policy

	snapshotFormat := strings.ToLower(c.Persistence.Format)
	if snapshotFormat == "" {
		snapshotFormat = "json"
	}
	if _, ok := validSnapshotFormats[snapshotFormat]; !ok {
		return fmt.Errorf("persistence.format %q is not supported", c.Persistence.Format)
	}
	c.Persistence.Format = snapshotFormat
	if c.Persistence.Enabled {
		if strings.TrimSpace(c.Persistence.DataDir) == "" {
			return fmt.Errorf("persistence.dataDir must not be empty when persistence is enabled")
		}
		if c.Persistence.SnapshotIntervalSec <= 0 {
			return fmt.Errorf("persistence.snapshotIntervalSec must be positive when persistence is enabled")
		}
		if c.Persistence.MaxSnapshots <= 0 {
			return fmt.Errorf("persistence.maxSnapshots must be positive when persistence is enabled")
		}
	}

	c.Cluster.RoutingMode = strings.ToLower(c.Cluster.RoutingMode)
	c.Cluster.FailoverMode = strings.ToLower(c.Cluster.FailoverMode)

	if !c.Cluster.Enabled {
		// Standalone mode rejects any leftover cluster flag so misconfigurations
		// fail loudly rather than being silently ignored.
		if c.Cluster.ShardingEnabled || c.Cluster.ReplicationEnabled {
			return fmt.Errorf("cluster.enabled=false but other cluster flags are set")
		}
		if len(c.Cluster.Peers) > 0 {
			return fmt.Errorf("cluster.enabled=false but cluster.peers is non-empty")
		}
		return nil
	}

	if strings.TrimSpace(c.Cluster.ID) == "" {
		return fmt.Errorf("cluster.id must not be empty when cluster is enabled")
	}
	if !c.Cluster.ShardingEnabled {
		return fmt.Errorf("cluster.enabled=true requires cluster.shardingEnabled=true")
	}
	if !c.Cluster.ReplicationEnabled {
		return fmt.Errorf("cluster.enabled=true requires cluster.replicationEnabled=true")
	}
	if c.Cluster.SlotCount == 0 || c.Cluster.SlotCount > 65536 {
		return fmt.Errorf("cluster.slotCount %d is out of range", c.Cluster.SlotCount)
	}
	if c.Cluster.RoutingMode != "proxy" {
		return fmt.Errorf("cluster.routingMode %q is not supported", c.Cluster.RoutingMode)
	}
	if c.Cluster.FailoverMode != "manual" {
		return fmt.Errorf("cluster.failoverMode %q is not supported", c.Cluster.FailoverMode)
	}
	if len(c.Cluster.Peers) < 2 || len(c.Cluster.Peers) > 5 {
		return fmt.Errorf("cluster.peers must contain between 2 and 5 nodes")
	}

	seen := make(map[string]struct{}, len(c.Cluster.Peers))
	seenAddresses := make(map[string]struct{}, len(c.Cluster.Peers))
	seenAPIAddresses := make(map[string]struct{}, len(c.Cluster.Peers))
	selfFound := false
	for _, p := range c.Cluster.Peers {
		if p.ID == "" || p.Address == "" || p.APIAddress == "" {
			return fmt.Errorf("cluster.peers entries must have id, address, and apiAddress")
		}
		if _, dup := seen[p.ID]; dup {
			return fmt.Errorf("cluster.peers contains duplicate id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		if _, dup := seenAddresses[p.Address]; dup {
			return fmt.Errorf("cluster.peers contains duplicate address %q", p.Address)
		}
		seenAddresses[p.Address] = struct{}{}
		if _, dup := seenAPIAddresses[p.APIAddress]; dup {
			return fmt.Errorf("cluster.peers contains duplicate apiAddress %q", p.APIAddress)
		}
		seenAPIAddresses[p.APIAddress] = struct{}{}
		if p.ID == c.Node.ID {
			selfFound = true
		}
	}
	if !selfFound {
		return fmt.Errorf("cluster.peers must include this node (%q)", c.Node.ID)
	}
	return nil
}
