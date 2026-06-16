package config

import (
	"fmt"
	"strings"
)

// validEvictionPolicies enumerates the policy names the engine recognizes.
// The set is intentionally small at the baseline milestone.
var validEvictionPolicies = map[string]struct{}{
	"noeviction": {},
	"fifo":        {},
	"lru":         {},
	"lfu":         {},
	"random":      {},
}

var validWriteSafetyModes = map[string]struct{}{
	"async":  {},
	"strong": {},
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

	if c.Engine.StripeCount <= 0 {
		return fmt.Errorf("engine.stripeCount must be positive")
	}
	policy := strings.ToLower(c.Engine.EvictionPolicy)
	if _, ok := validEvictionPolicies[policy]; !ok {
		return fmt.Errorf("engine.evictionPolicy %q is not supported", c.Engine.EvictionPolicy)
	}
	c.Engine.EvictionPolicy = policy

	mode := strings.ToLower(c.Cluster.WriteSafetyMode)
	if _, ok := validWriteSafetyModes[mode]; !ok {
		return fmt.Errorf("cluster.writeSafetyMode %q is not supported", c.Cluster.WriteSafetyMode)
	}
	c.Cluster.WriteSafetyMode = mode

	if !c.Cluster.Enabled {
		// Standalone mode rejects any leftover cluster flag so misconfigurations
		// fail loudly rather than being silently ignored.
		if c.Cluster.ShardingEnabled || c.Cluster.ReplicationEnabled || c.Cluster.AutoFailover {
			return fmt.Errorf("cluster.enabled=false but other cluster flags are set")
		}
		if len(c.Cluster.Peers) > 0 {
			return fmt.Errorf("cluster.enabled=false but cluster.peers is non-empty")
		}
		return nil
	}

	if c.Cluster.AutoFailover && !c.Cluster.ReplicationEnabled {
		return fmt.Errorf("cluster.autoFailover requires cluster.replicationEnabled")
	}
	if c.Cluster.ReplicationEnabled && !c.Cluster.ShardingEnabled {
		return fmt.Errorf("cluster.replicationEnabled requires cluster.shardingEnabled")
	}
	if mode == "strong" && !c.Cluster.ReplicationEnabled {
		return fmt.Errorf("cluster.writeSafetyMode=strong requires cluster.replicationEnabled")
	}
	if len(c.Cluster.Peers) == 0 {
		return fmt.Errorf("cluster.enabled=true but cluster.peers is empty")
	}

	seen := make(map[string]struct{}, len(c.Cluster.Peers))
	selfFound := false
	for _, p := range c.Cluster.Peers {
		if p.ID == "" || p.Address == "" {
			return fmt.Errorf("cluster.peers entries must have id and address")
		}
		if _, dup := seen[p.ID]; dup {
			return fmt.Errorf("cluster.peers contains duplicate id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		if p.ID == c.Node.ID {
			selfFound = true
		}
	}
	if !selfFound {
		return fmt.Errorf("cluster.peers must include this node (%q)", c.Node.ID)
	}
	return nil
}
