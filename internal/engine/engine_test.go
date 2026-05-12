package engine

import (
	"sync"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func newTestEngine() *Engine {
	return New(config.EngineConfig{StripeCount: 16, EvictionPolicy: "noop"})
}

func exec(t *testing.T, e *Engine, name string, args ...string) resp.Frame {
	t.Helper()
	c := &resp.Command{Name: name}
	for _, a := range args {
		c.Args = append(c.Args, []byte(a))
	}
	return e.Execute(c)
}

func mustOK(t *testing.T, f resp.Frame) {
	t.Helper()
	s, ok := f.(resp.SimpleString)
	if !ok || s != "OK" {
		t.Fatalf("expected +OK, got %#v", f)
	}
}

func mustInt(t *testing.T, f resp.Frame, want int64) {
	t.Helper()
	n, ok := f.(resp.Integer)
	if !ok || int64(n) != want {
		t.Fatalf("expected :%d, got %#v", want, f)
	}
}

func mustBulk(t *testing.T, f resp.Frame, want string) {
	t.Helper()
	b, ok := f.(resp.BulkString)
	if !ok || b.Null || string(b.Value) != want {
		t.Fatalf("expected bulk %q, got %#v", want, f)
	}
}

func mustNullBulk(t *testing.T, f resp.Frame) {
	t.Helper()
	b, ok := f.(resp.BulkString)
	if !ok || !b.Null {
		t.Fatalf("expected nil bulk, got %#v", f)
	}
}

func TestStringRoundTrip(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	mustBulk(t, exec(t, e, "GET", "k"), "v")
	mustInt(t, exec(t, e, "EXISTS", "k"), 1)
	mustInt(t, exec(t, e, "DEL", "k"), 1)
	mustInt(t, exec(t, e, "EXISTS", "k"), 0)
	mustNullBulk(t, exec(t, e, "GET", "k"))
}

func TestSetWithNXXX(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v", "NX"))
	mustNullBulk(t, exec(t, e, "SET", "k", "w", "NX")) // already exists
	mustBulk(t, exec(t, e, "GET", "k"), "v")
	mustOK(t, exec(t, e, "SET", "k", "w", "XX"))
	mustBulk(t, exec(t, e, "GET", "k"), "w")
	mustNullBulk(t, exec(t, e, "SET", "missing", "x", "XX"))
}

func TestIncr(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "INCR", "c"), 1)
	mustInt(t, exec(t, e, "INCR", "c"), 2)
	mustInt(t, exec(t, e, "INCR", "c"), 3)
	mustOK(t, exec(t, e, "SET", "junk", "abc"))
	if _, ok := exec(t, e, "INCR", "junk").(resp.Error); !ok {
		t.Fatal("expected error on non-integer INCR")
	}
}

func TestExpireAndTTL(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "TTL", "missing"), -2)
	mustOK(t, exec(t, e, "SET", "k", "v"))
	mustInt(t, exec(t, e, "TTL", "k"), -1)
	mustInt(t, exec(t, e, "EXPIRE", "k", "100"), 1)
	if n, _ := exec(t, e, "TTL", "k").(resp.Integer); n <= 0 || int64(n) > 100 {
		t.Fatalf("unexpected TTL: %v", n)
	}
	mustInt(t, exec(t, e, "EXPIRE", "k", "0"), 1) // negative TTL deletes
	mustInt(t, exec(t, e, "EXISTS", "k"), 0)
	mustInt(t, exec(t, e, "EXPIRE", "missing", "10"), 0)
}

func TestPing(t *testing.T) {
	e := newTestEngine()
	if s, ok := exec(t, e, "PING").(resp.SimpleString); !ok || s != "PONG" {
		t.Fatalf("ping: %#v", exec(t, e, "PING"))
	}
	mustBulk(t, exec(t, e, "PING", "hello"), "hello")
}

func TestWrongType(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	// We do not have list commands yet, but INCR on a non-numeric string is a
	// good proxy for the error pipeline.
	if _, ok := exec(t, e, "INCR", "k").(resp.Error); !ok {
		t.Fatal("expected error")
	}
}

func TestConcurrentIncr(t *testing.T) {
	e := newTestEngine()
	const n = 1000
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				exec(t, e, "INCR", "shared")
			}
		}()
	}
	wg.Wait()
	got := exec(t, e, "GET", "shared")
	mustBulk(t, got, "8000")
}

func TestFlush(t *testing.T) {
	e := newTestEngine()
	for i := 0; i < 50; i++ {
		mustOK(t, exec(t, e, "SET", string([]byte{byte(i)}), "v"))
	}
	mustOK(t, exec(t, e, "FLUSHDB"))
	mustInt(t, exec(t, e, "EXISTS", "a"), 0)
}
