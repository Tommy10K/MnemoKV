package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func newTestEngine() *Engine {
	return New(config.EngineConfig{StripeCount: 16, EvictionPolicy: "noeviction"})
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

func TestConcurrentSetNXIsAtomic(t *testing.T) {
	e := newTestEngine()
	const workers = 32
	start := make(chan struct{})
	var successes atomic.Int64
	var failures atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(value string) {
			defer wg.Done()
			<-start
			switch f := exec(t, e, "SET", "shared", value, "NX").(type) {
			case resp.SimpleString:
				if f == "OK" {
					successes.Add(1)
				}
			case resp.BulkString:
				if f.Null {
					failures.Add(1)
				}
			}
		}(fmt.Sprintf("value-%d", i))
	}
	close(start)
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly one successful NX write, got %d", got)
	}
	if got := failures.Load(); got != workers-1 {
		t.Fatalf("expected %d failed NX writes, got %d", workers-1, got)
	}
}

func TestSetOptionValidation(t *testing.T) {
	e := newTestEngine()
	cases := [][]string{
		{"k", "v", "EX", "1", "EX", "2"},
		{"k", "v", "PX", "1", "PX", "2"},
		{"k", "v", "EX", "1", "PX", "2"},
		{"k", "v", "EX", "9223372036854775807"},
		{"k", "v", "PX", "9223372036854775807"},
	}
	for _, args := range cases {
		if _, ok := exec(t, e, "SET", args...).(resp.Error); !ok {
			t.Fatalf("expected SET error for %q", args)
		}
	}
	mustInt(t, exec(t, e, "EXISTS", "k"), 0)
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

func TestIncrRejectsNonCanonicalIntegers(t *testing.T) {
	e := newTestEngine()
	for _, value := range []string{"01", "+1", "-0"} {
		mustOK(t, exec(t, e, "SET", "k", value))
		if _, ok := exec(t, e, "INCR", "k").(resp.Error); !ok {
			t.Fatalf("expected INCR to reject %q", value)
		}
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

func TestExpireRejectsOverflow(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	if _, ok := exec(t, e, "EXPIRE", "k", "9223372036854775807").(resp.Error); !ok {
		t.Fatal("expected overflow error")
	}
	mustBulk(t, exec(t, e, "GET", "k"), "v")
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

func TestUtilityCommandArity(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	for _, name := range []string{"QUIT", "FLUSHDB", "FLUSHALL"} {
		if _, ok := exec(t, e, name, "extra").(resp.Error); !ok {
			t.Fatalf("expected %s arity error", name)
		}
	}
	mustBulk(t, exec(t, e, "GET", "k"), "v")
}

func TestConcurrentReadsAndMetadataWrites(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "0"))
	var wg sync.WaitGroup
	wg.Add(4)
	for i := 0; i < 4; i++ {
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				switch worker {
				case 0:
					exec(t, e, "GET", "k")
				case 1:
					exec(t, e, "INCR", "k")
				case 2:
					exec(t, e, "TTL", "k")
				case 3:
					exec(t, e, "EXPIRE", "k", "60")
				}
			}
		}(i)
	}
	wg.Wait()
}
