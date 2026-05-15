package engine

import (
	"context"
	"strings"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine/eviction"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type WriteHook func(ctx context.Context, cmd *resp.Command) error

type Engine struct {
	store     *Store
	executor  *Executor
	memory    *MemoryTracker
	eviction  *eviction.Manager
	metrics   metrics.Sink
	writeHook WriteHook
	hookSync  bool
}

func New(cfg config.EngineConfig) *Engine {
	return NewWithMetrics(cfg, metrics.NewNoop())
}

func NewWithMetrics(cfg config.EngineConfig, sink metrics.Sink) *Engine {
	store := NewStore(cfg.StripeCount)
	exec := newExecutor(store)
	mem := NewMemoryTracker(store, cfg.MemoryLimitBytes)
	policy := eviction.PolicyByName(cfg.EvictionPolicy)
	evMgr := eviction.NewManager(store, policy)
	return &Engine{store: store, executor: exec, memory: mem, eviction: evMgr, metrics: sink}
}

func (e *Engine) Store() *Store { return e.store }

func (e *Engine) Memory() *MemoryTracker { return e.memory }

func (e *Engine) Eviction() *eviction.Manager { return e.eviction }

func (e *Engine) Metrics() metrics.Sink { return e.metrics }

func (e *Engine) SetWriteHook(hook WriteHook, sync bool) {
	e.writeHook = hook
	e.hookSync = sync
}

func (e *Engine) Execute(cmd *resp.Command) resp.Frame {
	if e.memory.HasLimit() && e.memory.ExceedsLimit() {
		overflow := e.memory.Used() - e.memory.Limit()
		evicted := e.eviction.Run(overflow)
		if evicted > 0 {
			e.metrics.IncCounter("eviction.count")
		}
	}

	if cmd.Name == "REPLICATE" {
		return e.applyReplicated(cmd)
	}

	if e.writeHook != nil && e.hookSync && IsWriteCommand(cmd.Name) {
		if err := e.writeHook(context.Background(), cmd); err != nil {
			return resp.NewError("ERR", "replication failed: "+err.Error())
		}
	}

	start := time.Now()
	frame := e.executor.Execute(cmd)
	e.metrics.ObserveLatency("cmd."+strings.ToLower(cmd.Name), time.Since(start))
	e.metrics.IncCounter("cmd.total")

	if e.writeHook != nil && !e.hookSync && IsWriteCommand(cmd.Name) && !isErrorFrame(frame) {
		_ = e.writeHook(context.Background(), cmd)
	}
	return frame
}

func (e *Engine) applyReplicated(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 2 {
		return resp.NewError("ERR", "REPLICATE requires sequence and command")
	}
	inner := &resp.Command{
		Name: strings.ToUpper(string(cmd.Args[1])),
		Args: cmd.Args[2:],
	}
	return e.executor.Execute(inner)
}

func isErrorFrame(f resp.Frame) bool {
	_, ok := f.(resp.Error)
	return ok
}
