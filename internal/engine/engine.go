package engine

import (
	"strings"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine/eviction"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type Engine struct {
	store    *Store
	executor *Executor
	memory   *MemoryTracker
	eviction *eviction.Manager
	metrics  metrics.Sink
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

func (e *Engine) Store() *Store              { return e.store }
func (e *Engine) Memory() *MemoryTracker     { return e.memory }
func (e *Engine) Eviction() *eviction.Manager { return e.eviction }
func (e *Engine) Metrics() metrics.Sink       { return e.metrics }

func (e *Engine) Execute(cmd *resp.Command) resp.Frame {
	if e.memory.HasLimit() && e.memory.ExceedsLimit() {
		overflow := e.memory.Used() - e.memory.Limit()
		evicted := e.eviction.Run(overflow)
		if evicted > 0 {
			e.metrics.IncCounter("eviction.count")
		}
	}
	start := time.Now()
	frame := e.executor.Execute(cmd)
	e.metrics.ObserveLatency("cmd."+strings.ToLower(cmd.Name), time.Since(start))
	e.metrics.IncCounter("cmd.total")
	return frame
}
