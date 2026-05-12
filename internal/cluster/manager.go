// Package cluster will own routing, replication, membership, and the control
// plane in later phases. The baseline milestone only needs a placeholder
// Manager so the binary's wiring stays stable across phases.
package cluster

import (
	"context"

	"github.com/mnemokv/mnemokv/internal/config"
)

// Manager is the cluster lifecycle handle. In the baseline milestone every
// method is a no-op because clustering is not active.
type Manager struct {
	cfg config.ClusterConfig
}

// NewManager returns a Manager whose Start/Shutdown methods are inert until
// the cluster phases land.
func NewManager(cfg config.ClusterConfig) *Manager {
	return &Manager{cfg: cfg}
}

// Start is a no-op in the baseline.
func (m *Manager) Start(ctx context.Context) error { return nil }

// Shutdown is a no-op in the baseline.
func (m *Manager) Shutdown(ctx context.Context) error { return nil }
