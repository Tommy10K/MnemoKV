package engine

import (
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

// Engine bundles the store with its executor. Higher layers (the TCP server)
// hold an *Engine and call Execute; nothing outside this package should need
// to know about Store directly.
type Engine struct {
	store    *Store
	executor *Executor
	memory   *MemoryTracker
}

// New builds an Engine from the given configuration.
func New(cfg config.EngineConfig) *Engine {
	store := NewStore(cfg.StripeCount)
	exec := newExecutor(store)
	mem := NewMemoryTracker(store, cfg.MemoryLimitBytes)
	return &Engine{store: store, executor: exec, memory: mem}
}

// Store returns the underlying store.
func (e *Engine) Store() *Store { return e.store }

// Memory returns the memory tracker.
func (e *Engine) Memory() *MemoryTracker { return e.memory }

// Execute is the single entry point used by the server. The returned frame is
// always non-nil; protocol-level errors come back as resp.Error frames.
func (e *Engine) Execute(cmd *resp.Command) resp.Frame {
	return e.executor.Execute(cmd)
}
