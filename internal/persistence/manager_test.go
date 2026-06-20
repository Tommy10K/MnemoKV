package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

func TestJSONAndBinarySnapshotsRestoreSameDataset(t *testing.T) {
	for _, format := range []string{snapshot.FormatJSON, snapshot.FormatBinary} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			cfg := testPersistenceConfig(dir, format)
			source := testDataset(t)
			manager := New(cfg, "node-1", source, nil)
			result, err := manager.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			if result.Format != format || result.EntryCount != 3 {
				t.Fatalf("unexpected result: %+v", result)
			}

			target := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
			restoreManager := New(cfg, "node-1", target, nil)
			restored, err := restoreManager.RestoreLatest()
			if err != nil {
				t.Fatal(err)
			}
			if restored.RestoredEntries != 3 {
				t.Fatalf("restored entries = %d, want 3", restored.RestoredEntries)
			}
			want, _ := source.SnapshotEntries()
			got, _ := target.SnapshotEntries()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("restored dataset differs\ngot:  %#v\nwant: %#v", got, want)
			}
		})
	}
}

func TestRestoreSkipsEntriesThatExpiredAfterSnapshot(t *testing.T) {
	dir := t.TempDir()
	cfg := testPersistenceConfig(dir, snapshot.FormatJSON)
	source := engine.New(config.EngineConfig{StripeCount: 2, EvictionPolicy: "noeviction"})
	execute(t, source, "SET", "soon", "gone", "PX", "20")
	manager := New(cfg, "node-1", source, nil)
	if _, err := manager.Snapshot(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(25 * time.Millisecond)

	target := engine.New(config.EngineConfig{StripeCount: 2, EvictionPolicy: "noeviction"})
	restored, err := New(cfg, "node-1", target, nil).RestoreLatest()
	if err != nil {
		t.Fatal(err)
	}
	if restored.RestoredEntries != 0 || target.Store().Exists([]byte("soon")) {
		t.Fatalf("expired key was restored: %+v", restored)
	}
}

func TestRetentionKeepsNewestValidSnapshotsAndLeavesInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := testPersistenceConfig(dir, snapshot.FormatJSON)
	cfg.MaxSnapshots = 2
	manager := New(cfg, "node-1", testDataset(t), nil)
	now := time.Unix(100, 0)
	manager.now = func() time.Time { return now }
	invalidPath := filepath.Join(dir, "snapshot-invalid.snapshot.json")
	if err := os.WriteFile(invalidPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := manager.Snapshot(); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	validCount := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary snapshot was left behind: %s", entry.Name())
		}
		if entry.Name() != filepath.Base(invalidPath) {
			validCount++
		}
	}
	if validCount != 2 {
		t.Fatalf("valid snapshot count = %d, want 2", validCount)
	}
	if _, err := os.Stat(invalidPath); err != nil {
		t.Fatalf("invalid snapshot should not count toward retention: %v", err)
	}
}

func TestRestoreFallsBackPastInvalidNewerFile(t *testing.T) {
	dir := t.TempDir()
	cfg := testPersistenceConfig(dir, snapshot.FormatBinary)
	manager := New(cfg, "node-1", testDataset(t), nil)
	if _, err := manager.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot-newer.snapshot.bin"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := engine.New(config.EngineConfig{StripeCount: 2, EvictionPolicy: "noeviction"})
	result, err := New(cfg, "node-1", target, nil).RestoreLatest()
	if err != nil {
		t.Fatal(err)
	}
	if result.RestoredEntries != 3 {
		t.Fatalf("restored entries = %d, want 3", result.RestoredEntries)
	}
}

func TestSnapshotIncludesClusterMetadata(t *testing.T) {
	dir := t.TempDir()
	cfg := testPersistenceConfig(dir, snapshot.FormatJSON)
	provider := func() snapshot.ClusterMetadata {
		return snapshot.ClusterMetadata{
			ClusterID: "cluster-1", SlotCount: 2, MetadataVersion: 4,
			Slots: []snapshot.Slot{{Number: 0, Role: "leader", Term: 3, LastAppliedSequence: 8}, {Number: 1, Role: "replica", Term: 4, LastAppliedSequence: 9}},
		}
	}
	result, err := New(cfg, "node-1", testDataset(t), provider).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	model, err := snapshot.Decode(file, snapshot.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if model.ClusterID != "cluster-1" || model.SlotCount != 2 || model.MetadataVersion != 4 || len(model.Slots) != 2 {
		t.Fatalf("cluster metadata missing: %+v", model)
	}
}

func TestDisabledAndMissingSnapshotsReturnTypedErrors(t *testing.T) {
	eng := engine.New(config.EngineConfig{StripeCount: 1, EvictionPolicy: "noeviction"})
	disabled := New(config.PersistenceConfig{}, "node-1", eng, nil)
	if _, err := disabled.Snapshot(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled snapshot error = %v", err)
	}
	cfg := testPersistenceConfig(t.TempDir(), snapshot.FormatJSON)
	if _, err := New(cfg, "node-1", eng, nil).RestoreLatest(); !errors.Is(err, ErrNoSnapshot) {
		t.Fatalf("missing restore error = %v", err)
	}
}

func testPersistenceConfig(dir, format string) config.PersistenceConfig {
	return config.PersistenceConfig{Enabled: true, DataDir: dir, SnapshotIntervalSec: 60, MaxSnapshots: 5, LoadOnStart: true, Format: format}
}

func testDataset(t *testing.T) *engine.Engine {
	t.Helper()
	eng := engine.New(config.EngineConfig{StripeCount: 4, EvictionPolicy: "noeviction"})
	execute(t, eng, "SET", "string", "value")
	execute(t, eng, "RPUSH", "list", "one", "two")
	execute(t, eng, "ZADD", "zset", "2", "b", "1", "a")
	return eng
}

func execute(t *testing.T, eng *engine.Engine, name string, args ...string) {
	t.Helper()
	cmd := &resp.Command{Name: name, Args: make([][]byte, len(args))}
	for i := range args {
		cmd.Args[i] = []byte(args[i])
	}
	if _, ok := eng.Execute(cmd).(resp.Error); ok {
		t.Fatalf("%s failed", name)
	}
}
