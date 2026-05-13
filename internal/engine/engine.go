package engine

import (
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine/eviction"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type Engine struct {
	store    *Store
	executor *Executor
	memory   *MemoryTracker
	eviction *eviction.Manager
}

func New(cfg config.EngineConfig) *Engine {
	store := NewStore(cfg.StripeCount)
	exec := newExecutor(store)
	mem := NewMemoryTracker(store, cfg.MemoryLimitBytes)
	policy := eviction.PolicyByName(cfg.EvictionPolicy)
	evMgr := eviction.NewManager(store, policy)
	return &Engine{store: store, executor: exec, memory: mem, eviction: evMgr}
}

func (e *Engine) Store() *Store              { return e.store }
func (e *Engine) Memory() *MemoryTracker     { return e.memory }
func (e *Engine) Eviction() *eviction.Manager { return e.eviction }

func (e *Engine) Execute(cmd *resp.Command) resp.Frame {
	if e.memory.HasLimit() && e.memory.ExceedsLimit() {
		e.eviction.Run(e.memory.Used() - e.memory.Limit())
	}
	return e.executor.Execute(cmd)
}
