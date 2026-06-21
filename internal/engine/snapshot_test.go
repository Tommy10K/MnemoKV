package engine

import (
	"reflect"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func TestSnapshotEntriesRestoreAllValueTypesAndSkipExpired(t *testing.T) {
	source := New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	executeSnapshotCommand(t, source, "SET", "string", "value")
	executeSnapshotCommand(t, source, "RPUSH", "list", "one", "two")
	executeSnapshotCommand(t, source, "ZADD", "zset", "2", "b", "1", "a")
	executeSnapshotCommand(t, source, "SET", "expired", "gone", "PX", "1")
	time.Sleep(2 * time.Millisecond)

	entries, err := source.SnapshotEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("snapshot entry count = %d, want 3", len(entries))
	}

	target := New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	restored, err := target.RestoreSnapshotEntries(entries, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var snapshotBytes uint64
	for _, entry := range entries {
		snapshotBytes += entry.ApproxSize
	}
	if restored != 3 || target.Store().UsedBytes() != snapshotBytes {
		t.Fatalf("restore count=%d bytes=%d, snapshot bytes=%d", restored, target.Store().UsedBytes(), snapshotBytes)
	}

	assertSnapshotFrame(t, target, "GET", []string{"string"}, resp.BulkFromString("value"))
	assertSnapshotFrame(t, target, "LPOP", []string{"list"}, resp.BulkFromString("one"))
	assertSnapshotFrame(t, target, "ZRANGE", []string{"zset", "0", "-1", "WITHSCORES"}, resp.Array{Items: []resp.Frame{resp.BulkFromString("a"), resp.BulkFromString("1"), resp.BulkFromString("b"), resp.BulkFromString("2")}})
}

func executeSnapshotCommand(t *testing.T, eng *Engine, name string, args ...string) {
	t.Helper()
	cmd := &resp.Command{Name: name, Args: make([][]byte, len(args))}
	for i := range args {
		cmd.Args[i] = []byte(args[i])
	}
	if frame := eng.Execute(cmd); isErrorFrame(frame) {
		t.Fatalf("%s failed: %#v", name, frame)
	}
}

func assertSnapshotFrame(t *testing.T, eng *Engine, name string, args []string, want resp.Frame) {
	t.Helper()
	cmd := &resp.Command{Name: name, Args: make([][]byte, len(args))}
	for i := range args {
		cmd.Args[i] = []byte(args[i])
	}
	got := eng.Execute(cmd)
	if !framesEqual(got, want) {
		t.Fatalf("%s result = %#v, want %#v", name, got, want)
	}
}

func framesEqual(a, b resp.Frame) bool {
	return reflect.DeepEqual(a, b)
}
