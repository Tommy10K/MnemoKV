package engine

import (
	"sync"
	"testing"

	"github.com/mnemokv/mnemokv/internal/resp"
)

func TestZSetBasic(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "ZADD", "lb", "1.0", "alice", "2.5", "bob", "1.8", "carol"), 3)
	mustInt(t, exec(t, e, "ZCARD", "lb"), 3)

	f := exec(t, e, "ZRANGE", "lb", "0", "-1")
	arr, ok := f.(resp.Array)
	if !ok {
		t.Fatalf("expected array, got %#v", f)
	}
	if len(arr.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(arr.Items))
	}
	assertBulk(t, arr.Items[0], "alice")
	assertBulk(t, arr.Items[1], "carol")
	assertBulk(t, arr.Items[2], "bob")
}

func TestZSetScoreUpdate(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "ZADD", "s", "1", "a", "2", "b"), 2)
	mustInt(t, exec(t, e, "ZADD", "s", "3", "a"), 0) // update, not new

	f := exec(t, e, "ZRANGE", "s", "0", "-1")
	arr := f.(resp.Array)
	assertBulk(t, arr.Items[0], "b")
	assertBulk(t, arr.Items[1], "a")
}

func TestZSetWithScores(t *testing.T) {
	e := newTestEngine()
	exec(t, e, "ZADD", "z", "1.5", "x", "2.5", "y")
	f := exec(t, e, "ZRANGE", "z", "0", "-1", "WITHSCORES")
	arr := f.(resp.Array)
	if len(arr.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(arr.Items))
	}
	assertBulk(t, arr.Items[0], "x")
	assertBulk(t, arr.Items[1], "1.5")
	assertBulk(t, arr.Items[2], "y")
	assertBulk(t, arr.Items[3], "2.5")
}

func TestZScore(t *testing.T) {
	e := newTestEngine()
	exec(t, e, "ZADD", "z", "3.14", "pi")
	mustBulk(t, exec(t, e, "ZSCORE", "z", "pi"), "3.14")
	mustNullBulk(t, exec(t, e, "ZSCORE", "z", "missing"))
	mustNullBulk(t, exec(t, e, "ZSCORE", "nokey", "x"))
}

func TestZSetWrongType(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	f := exec(t, e, "ZADD", "k", "1", "m")
	if _, ok := f.(resp.Error); !ok {
		t.Fatalf("expected error, got %#v", f)
	}
}

func TestZSetConcurrent(t *testing.T) {
	e := newTestEngine()
	const n = 200
	const goroutines = 4
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				exec(t, e, "ZADD", "cz", "1", string(rune('A'+offset))+string(rune(i)))
			}
		}(g)
	}
	wg.Wait()
	mustInt(t, exec(t, e, "ZCARD", "cz"), int64(goroutines*n))
}

func assertBulk(t *testing.T, f resp.Frame, want string) {
	t.Helper()
	b, ok := f.(resp.BulkString)
	if !ok || string(b.Value) != want {
		t.Fatalf("expected bulk %q, got %#v", want, f)
	}
}
