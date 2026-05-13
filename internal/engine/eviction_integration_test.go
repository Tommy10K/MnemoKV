package engine

import (
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestEvictionTriggersOnMemoryLimit(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 200, EvictionPolicy: "lru"})

	for i := 0; i < 50; i++ {
		exec(t, e, "SET", "key"+string(rune('A'+i)), "value-that-takes-some-space")
	}

	used := e.Memory().Used()
	limit := e.Memory().Limit()
	if used > limit*2 {
		t.Fatalf("eviction did not keep memory in check: used=%d limit=%d", used, limit)
	}
}
