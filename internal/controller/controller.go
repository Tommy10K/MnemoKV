package controller

import (
	"context"
	"errors"
	"sync"

	"github.com/mnemokv/mnemokv/internal/config"
)

// Controller is the lifecycle boundary for the embedded control plane.
// Phase 0 intentionally runs only a cancellable no-op worker.
type Controller struct {
	cfg    config.ClusterConfig
	nodeID string

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(cfg config.ClusterConfig, nodeID string) *Controller {
	return &Controller{cfg: cfg, nodeID: nodeID}
}

func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return errors.New("controller already started")
	}
	workerCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})
	go func() {
		defer close(c.done)
		<-workerCtx.Done()
	}()
	return nil
}

func (c *Controller) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.cancel == nil {
		c.mu.Unlock()
		return nil
	}
	c.cancel()
	done := c.done
	c.cancel = nil
	c.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
