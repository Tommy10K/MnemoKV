package engine

import (
	"sync"
	"testing"

	"github.com/mnemokv/mnemokv/internal/resp"
)

func TestListPushPop(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "LPUSH", "q", "a", "b", "c"), 3)
	mustInt(t, exec(t, e, "LLEN", "q"), 3)
	mustBulk(t, exec(t, e, "LPOP", "q"), "c")
	mustBulk(t, exec(t, e, "RPOP", "q"), "a")
	mustInt(t, exec(t, e, "LLEN", "q"), 1)
	mustBulk(t, exec(t, e, "LPOP", "q"), "b")
	mustNullBulk(t, exec(t, e, "LPOP", "q"))
	mustInt(t, exec(t, e, "EXISTS", "q"), 0)
}

func TestListRPush(t *testing.T) {
	e := newTestEngine()
	mustInt(t, exec(t, e, "RPUSH", "q", "x", "y"), 2)
	mustBulk(t, exec(t, e, "LPOP", "q"), "x")
	mustBulk(t, exec(t, e, "LPOP", "q"), "y")
}

func TestListWrongType(t *testing.T) {
	e := newTestEngine()
	mustOK(t, exec(t, e, "SET", "k", "v"))
	f := exec(t, e, "LPUSH", "k", "a")
	if _, ok := f.(resp.Error); !ok {
		t.Fatalf("expected WRONGTYPE error, got %#v", f)
	}
}

func TestListConcurrent(t *testing.T) {
	e := newTestEngine()
	const n = 500
	const goroutines = 4
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				exec(t, e, "LPUSH", "cq", "v")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				exec(t, e, "RPUSH", "cq", "v")
			}
		}()
	}
	wg.Wait()
	mustInt(t, exec(t, e, "LLEN", "cq"), int64(goroutines*2*n))
}
