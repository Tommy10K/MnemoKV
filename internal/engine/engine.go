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
}

// New builds an Engine from the given configuration. The eviction manager and
// metrics sink are wired up in later phases; for the baseline we just need a
// store and an executor on top of it.
func New(cfg config.EngineConfig) *Engine {
	store := NewStore(cfg.StripeCount)
	exec := newExecutor(store)
	return &Engine{store: store, executor: exec}
}

// Store returns the underlying store. It is exposed so future packages
// (metrics, eviction, replication) can inspect the dictionary without poking
// at unexported fields.
func (e *Engine) Store() *Store { return e.store }

// Execute is the single entry point used by the server. The returned frame is
// always non-nil; protocol-level errors come back as resp.Error frames.
func (e *Engine) Execute(cmd *resp.Command) resp.Frame {
	return e.executor.Execute(cmd)
}
