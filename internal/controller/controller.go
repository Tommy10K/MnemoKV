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

	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
	raft     *RaftNode
	observer *Observer
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
	raftNode, err := NewRaftNodeFromConfig(c.cfg, c.nodeID)
	if err != nil {
		cancel()
		return err
	}
	c.cancel = cancel
	c.raft = raftNode
	observer, err := NewObserverFromConfig(c.cfg, raftNode)
	if err != nil {
		_ = raftNode.Shutdown()
		cancel()
		return err
	}
	c.observer = observer
	c.done = make(chan struct{})
	go func() {
		defer close(c.done)
		observer.Run(workerCtx)
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
	raftNode := c.raft
	c.cancel = nil
	c.raft = nil
	c.observer = nil
	c.mu.Unlock()

	select {
	case <-done:
		if raftNode != nil {
			return raftNode.Shutdown()
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
