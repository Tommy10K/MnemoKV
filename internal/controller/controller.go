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
	cfg          config.ClusterConfig
	controlPlane config.ControlPlaneConfig
	nodeID       string

	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
	raft     *RaftNode
	observer *Observer
	planner  *Planner
	executor *Executor
}

func New(cfg config.ClusterConfig, controlPlane config.ControlPlaneConfig, nodeID string) *Controller {
	return &Controller{cfg: cfg, controlPlane: controlPlane, nodeID: nodeID}
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
	observer, err := NewObserverFromConfig(c.cfg, raftNode)
	if err != nil {
		_ = raftNode.Shutdown()
		cancel()
		return err
	}
	planner := NewPlanner(raftNode, c.cfg.Controller)
	executor, err := NewExecutorFromConfig(c.cfg, c.controlPlane.RequestSigningSecret, raftNode)
	if err != nil {
		_ = raftNode.Shutdown()
		cancel()
		return err
	}
	c.cancel = cancel
	c.raft = raftNode
	c.observer = observer
	c.planner = planner
	c.executor = executor
	c.done = make(chan struct{})
	go func() {
		defer close(c.done)
		var workers sync.WaitGroup
		workers.Add(3)
		go func() { defer workers.Done(); observer.Run(workerCtx) }()
		go func() { defer workers.Done(); planner.Run(workerCtx) }()
		go func() { defer workers.Done(); executor.Run(workerCtx) }()
		workers.Wait()
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
	c.planner = nil
	c.executor = nil
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
