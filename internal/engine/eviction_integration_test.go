package engine

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine/eviction"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func TestEvictionTriggersOnMemoryLimit(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 200, EvictionPolicy: "lru"})

	for i := 0; i < 50; i++ {
		exec(t, e, "SET", "key"+string(rune('A'+i)), "value-that-takes-some-space")
	}

	used := e.Memory().Used()
	limit := e.Memory().Limit()
	if used > limit {
		t.Fatalf("eviction exceeded hard limit: used=%d limit=%d", used, limit)
	}
}

func TestNoEvictionPreservesExistingKeysAndRejects(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 150, EvictionPolicy: "noeviction"})

	mustOK(t, exec(t, e, "SET", "a", strings.Repeat("a", 20)))
	if frame := exec(t, e, "SET", "b", strings.Repeat("b", 70)); !isOOMTestFrame(frame) {
		t.Fatalf("expected OOM, got %#v", frame)
	}
	mustBulk(t, exec(t, e, "GET", "a"), strings.Repeat("a", 20))
	mustNullBulk(t, exec(t, e, "GET", "b"))
	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("used memory crossed limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
}

func TestFailedWritesDoNotEvict(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 170, EvictionPolicy: "lru"})
	mustOK(t, exec(t, e, "SET", "existing", strings.Repeat("x", 20)))

	frame := exec(t, e, "SET", "existing", "ignored", "NX")
	if _, ok := frame.(resp.BulkString); !ok {
		t.Fatalf("expected nil bulk for failed NX, got %#v", frame)
	}
	mustBulk(t, exec(t, e, "GET", "existing"), strings.Repeat("x", 20))

	if frame := exec(t, e, "SET", "oversized", strings.Repeat("z", 200)); !isOOMTestFrame(frame) {
		t.Fatalf("expected OOM for oversized value, got %#v", frame)
	}
	mustBulk(t, exec(t, e, "GET", "existing"), strings.Repeat("x", 20))
	mustNullBulk(t, exec(t, e, "GET", "oversized"))
}

func TestEvictionBeforeCommitKeepsLimitHard(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 180, EvictionPolicy: "fifo"})

	mustOK(t, exec(t, e, "SET", "a", strings.Repeat("a", 20)))
	mustOK(t, exec(t, e, "SET", "b", strings.Repeat("b", 20)))
	mustOK(t, exec(t, e, "SET", "c", strings.Repeat("c", 20)))

	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("write committed above limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
	mustNullBulk(t, exec(t, e, "GET", "a"))
}

func TestAdmissionExcludesUpdatedKeyFromEviction(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 1, MemoryLimitBytes: 188, EvictionPolicy: "random"})

	mustOK(t, exec(t, e, "SET", "keep", strings.Repeat("k", 20)))
	mustOK(t, exec(t, e, "SET", "victim", strings.Repeat("v", 20)))
	mustOK(t, exec(t, e, "SET", "keep", strings.Repeat("K", 31)))

	mustBulk(t, exec(t, e, "GET", "keep"), strings.Repeat("K", 31))
	mustNullBulk(t, exec(t, e, "GET", "victim"))
	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("used memory crossed limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
}

func TestSizeReducingUpdateAllowedAtLimit(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 100, EvictionPolicy: "noeviction"})
	mustOK(t, exec(t, e, "SET", "k", strings.Repeat("x", 30)))

	mustOK(t, exec(t, e, "SET", "k", "small"))
	mustBulk(t, exec(t, e, "GET", "k"), "small")
	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("used memory crossed limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
}

func TestAdministrativeReadsDoNotUpdateAccessMetadata(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	before, ok := e.Store().Peek([]byte("k"))
	if !ok {
		t.Fatal("missing key")
	}

	mustInt(t, exec(t, e, "EXISTS", "k"), 1)
	mustInt(t, exec(t, e, "TTL", "k"), -1)
	afterAdmin, _ := e.Store().Peek([]byte("k"))
	if afterAdmin.LastAccessNs != before.LastAccessNs || afterAdmin.AccessCount != before.AccessCount {
		t.Fatalf("administrative reads changed access metadata: before=%+v after=%+v", before, afterAdmin)
	}

	mustBulk(t, exec(t, e, "GET", "k"), "v")
	afterGet, _ := e.Store().Peek([]byte("k"))
	if afterGet.AccessCount <= afterAdmin.AccessCount || afterGet.LastAccessNs < afterAdmin.LastAccessNs {
		t.Fatalf("GET did not update access metadata: before=%+v after=%+v", afterAdmin, afterGet)
	}

	mustInt(t, exec(t, e, "LPUSH", "list", "a"), 1)
	listBefore, _ := e.Store().Peek([]byte("list"))
	mustInt(t, exec(t, e, "LLEN", "list"), 1)
	listAfter, _ := e.Store().Peek([]byte("list"))
	if listAfter.AccessCount != listBefore.AccessCount {
		t.Fatalf("LLEN changed access count: before=%d after=%d", listBefore.AccessCount, listAfter.AccessCount)
	}

	mustInt(t, exec(t, e, "ZADD", "z", "1", "a"), 1)
	zBefore, _ := e.Store().Peek([]byte("z"))
	mustInt(t, exec(t, e, "ZCARD", "z"), 1)
	zAfter, _ := e.Store().Peek([]byte("z"))
	if zAfter.AccessCount != zBefore.AccessCount {
		t.Fatalf("ZCARD changed access count: before=%d after=%d", zBefore.AccessCount, zAfter.AccessCount)
	}
	frame := exec(t, e, "ZSCORE", "z", "a")
	if _, ok := frame.(resp.BulkString); !ok {
		t.Fatalf("expected ZSCORE bulk, got %#v", frame)
	}
	zScoreAfter, _ := e.Store().Peek([]byte("z"))
	if zScoreAfter.AccessCount <= zAfter.AccessCount {
		t.Fatalf("ZSCORE did not update access count")
	}
}

func TestConcurrentAdmissionDoesNotExceedLimit(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 16, MemoryLimitBytes: 800, EvictionPolicy: "lru"})

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			exec(t, e, "SET", fmt.Sprintf("k-%d", i), strings.Repeat("x", 40))
		}(i)
	}
	wg.Wait()

	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("concurrent writes exceeded limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
}

func TestConcurrentAdmissionAndPolicySwitching(t *testing.T) {
	e := New(config.EngineConfig{StripeCount: 16, MemoryLimitBytes: 800, EvictionPolicy: "lru"})
	done := make(chan struct{})

	var switcher sync.WaitGroup
	switcher.Add(1)
	go func() {
		defer switcher.Done()
		policies := []eviction.Policy{
			eviction.FIFO{},
			eviction.LRU{},
			eviction.LFU{},
			eviction.Random{},
			eviction.NoEviction{},
		}
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
				e.Eviction().SetPolicy(policies[i%len(policies)])
			}
		}
	}()

	var writers sync.WaitGroup
	for i := 0; i < 64; i++ {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			exec(t, e, "SET", fmt.Sprintf("switch-%d", i), strings.Repeat("x", 40))
		}(i)
	}
	writers.Wait()
	close(done)
	switcher.Wait()

	if e.Memory().Used() > e.Memory().Limit() {
		t.Fatalf("concurrent policy switching exceeded limit: used=%d limit=%d", e.Memory().Used(), e.Memory().Limit())
	}
}

func TestEvictionMetricsTrackAdmission(t *testing.T) {
	sink := metrics.NewInMemory(64)
	e := NewWithMetrics(config.EngineConfig{StripeCount: 4, MemoryLimitBytes: 180, EvictionPolicy: "fifo"}, sink)

	mustOK(t, exec(t, e, "SET", "a", strings.Repeat("a", 20)))
	mustOK(t, exec(t, e, "SET", "b", strings.Repeat("b", 20)))
	mustOK(t, exec(t, e, "SET", "c", strings.Repeat("c", 20)))
	if sink.Counter("eviction.attempts") == 0 {
		t.Fatal("expected eviction attempts metric")
	}
	if sink.Counter("eviction.keys_evicted") == 0 {
		t.Fatal("expected evicted keys metric")
	}
	if sink.Counter("eviction.bytes_freed") == 0 {
		t.Fatal("expected freed bytes metric")
	}

	if frame := exec(t, e, "SET", "huge", strings.Repeat("h", 200)); !isOOMTestFrame(frame) {
		t.Fatalf("expected OOM, got %#v", frame)
	}
	if sink.Counter("eviction.rejected_writes") == 0 {
		t.Fatal("expected rejected writes metric")
	}
}

func isOOMTestFrame(frame resp.Frame) bool {
	err, ok := frame.(resp.Error)
	return ok && err.Prefix == "OOM"
}
