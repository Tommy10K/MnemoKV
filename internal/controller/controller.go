package controller

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/controlplane"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/metrics"
)

// Controller is the lifecycle boundary for the embedded control plane.
// Phase 0 intentionally runs only a cancellable no-op worker.
type Controller struct {
	cfg          config.ClusterConfig
	controlPlane config.ControlPlaneConfig
	nodeID       string
	metrics      *metrics.InMemory

	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan struct{}
	raft      *RaftNode
	observer  *Observer
	planner   *Planner
	executor  *Executor
	returning *ReturningNodeController
}

func New(cfg config.ClusterConfig, controlPlane config.ControlPlaneConfig, nodeID string, sink *metrics.InMemory) *Controller {
	return &Controller{cfg: cfg, controlPlane: controlPlane, nodeID: nodeID, metrics: sink}
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
	returning, err := NewReturningNodeControllerFromConfig(c.cfg, c.controlPlane.RequestSigningSecret, raftNode)
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
	c.returning = returning
	c.done = make(chan struct{})
	go func() {
		defer close(c.done)
		var workers sync.WaitGroup
		workers.Add(5)
		go func() { defer workers.Done(); observer.Run(workerCtx) }()
		go func() { defer workers.Done(); planner.Run(workerCtx) }()
		go func() { defer workers.Done(); executor.Run(workerCtx) }()
		go func() { defer workers.Done(); returning.Run(workerCtx) }()
		go func() { defer workers.Done(); c.monitorStatus(workerCtx, raftNode) }()
		workers.Wait()
	}()
	return nil
}

func (c *Controller) StatusSnapshot() controlplane.StatusSnapshot {
	c.mu.Lock()
	raftNode := c.raft
	c.mu.Unlock()
	if raftNode == nil {
		return controlplane.StatusSnapshot{State: "starting"}
	}
	return BuildStatusSnapshot(raftNode.State())
}

func (c *Controller) monitorStatus(ctx context.Context, raftNode *RaftNode) {
	interval := time.Duration(c.cfg.Controller.ObserveIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var previous string
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !raftNode.IsLeader() {
				continue
			}
			status := BuildStatusSnapshot(raftNode.State())
			if c.metrics != nil {
				c.metrics.Gauge("controller.degraded_slots", float64(len(status.OneCopySlots)))
				c.metrics.Gauge("controller.unavailable_slots", float64(len(status.UnavailableSlots)))
				c.metrics.Gauge("controller.failed_nodes", float64(len(status.FailedNodes)))
			}
			raw, _ := json.Marshal(status)
			fingerprint := string(raw)
			if fingerprint == previous {
				continue
			}
			previous = fingerprint
			if c.metrics != nil {
				c.metrics.IncCounter("controller.status_changes")
				if len(status.UnavailableSlots) > 0 {
					c.metrics.IncCounter("controller.potential_data_loss_events")
				}
				c.metrics.PublishEvent(metrics.Event{Name: "controller.status", Timestamp: time.Now(), Labels: map[string]string{"state": status.State}})
			}
			if len(status.UnavailableSlots) > 0 {
				logging.Warnf("controller: status=%s failed=%v unavailable_slots=%d warning=%s", status.State, status.FailedNodes, len(status.UnavailableSlots), status.Warning)
			} else {
				logging.Infof("controller: status=%s failed=%v degraded_slots=%d", status.State, status.FailedNodes, len(status.OneCopySlots))
			}
		}
	}
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
	c.returning = nil
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
