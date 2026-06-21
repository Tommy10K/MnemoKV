// Package persistence manages periodic and manual versioned snapshots.
package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/snapshot"
)

var (
	ErrDisabled   = errors.New("snapshot persistence is disabled")
	ErrNoSnapshot = errors.New("no snapshot found")
)

type MetadataProvider func() snapshot.ClusterMetadata
type MetadataRestorer func(snapshot.ClusterMetadata) error

type Result struct {
	Path       string    `json:"path"`
	Format     string    `json:"format"`
	CreatedAt  time.Time `json:"createdAt"`
	EntryCount int       `json:"entryCount"`
	Checksum   string    `json:"checksum"`
}

type RestoreResult struct {
	Result
	RestoredEntries int `json:"restoredEntries"`
}

type Manager struct {
	cfg             config.PersistenceConfig
	nodeID          string
	engine          *engine.Engine
	metadata        MetadataProvider
	restoreMetadata MetadataRestorer
	now             func() time.Time

	mu sync.Mutex
	wg sync.WaitGroup
}

func New(cfg config.PersistenceConfig, nodeID string, eng *engine.Engine, metadata MetadataProvider) *Manager {
	return &Manager{cfg: cfg, nodeID: nodeID, engine: eng, metadata: metadata, now: time.Now}
}

func (m *Manager) SetMetadataRestorer(restorer MetadataRestorer) {
	m.mu.Lock()
	m.restoreMetadata = restorer
	m.mu.Unlock()
}

// Snapshot writes one atomic snapshot and applies valid-snapshot retention.
func (m *Manager) Snapshot() (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return Result{}, ErrDisabled
	}
	if m.engine == nil {
		return Result{}, fmt.Errorf("snapshot engine is nil")
	}
	if err := os.MkdirAll(m.cfg.DataDir, 0o750); err != nil {
		return Result{}, fmt.Errorf("create snapshot directory: %w", err)
	}

	entries, err := m.engine.SnapshotEntries()
	if err != nil {
		return Result{}, err
	}
	createdAt := m.now().UTC()
	model := &snapshot.Model{
		Format: m.cfg.Format, FormatVersion: snapshot.FormatVersion, NodeID: m.nodeID,
		CreatedAt: createdAt, Entries: entries,
	}
	if m.metadata != nil {
		meta := m.metadata()
		model.ClusterID = meta.ClusterID
		model.SlotCount = meta.SlotCount
		model.MetadataVersion = meta.MetadataVersion
		model.Peers = append([]snapshot.Peer(nil), meta.Peers...)
		model.Slots = append([]snapshot.Slot(nil), meta.Slots...)
	}
	if err := model.Seal(); err != nil {
		return Result{}, err
	}

	ext := "bin"
	if m.cfg.Format == snapshot.FormatJSON {
		ext = "json"
	}
	name := fmt.Sprintf("snapshot-%020d-%s.snapshot.%s", createdAt.UnixNano(), model.Checksum[:12], ext)
	path, err := availableSnapshotPath(m.cfg.DataDir, name)
	if err != nil {
		return Result{}, err
	}
	if err := writeAtomic(path, model); err != nil {
		return Result{}, err
	}
	if err := m.pruneValidSnapshots(); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Format: model.Format, CreatedAt: createdAt, EntryCount: len(entries), Checksum: model.Checksum}, nil
}

// RestoreLatest loads the newest valid snapshot for this node. Invalid newer
// files are skipped so an older valid snapshot remains usable.
func (m *Manager) RestoreLatest() (RestoreResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return RestoreResult{}, ErrDisabled
	}
	if m.engine == nil {
		return RestoreResult{}, fmt.Errorf("snapshot engine is nil")
	}
	valid, candidateCount, lastErr, err := m.validSnapshots()
	if err != nil {
		return RestoreResult{}, err
	}
	if len(valid) == 0 {
		if candidateCount == 0 {
			return RestoreResult{}, ErrNoSnapshot
		}
		return RestoreResult{}, fmt.Errorf("no valid snapshot found among %d files: %w", candidateCount, lastErr)
	}
	latest := valid[0]
	if m.restoreMetadata != nil {
		meta := snapshot.ClusterMetadata{
			ClusterID: latest.model.ClusterID, SlotCount: latest.model.SlotCount,
			MetadataVersion: latest.model.MetadataVersion, Peers: append([]snapshot.Peer(nil), latest.model.Peers...),
			Slots: append([]snapshot.Slot(nil), latest.model.Slots...),
		}
		if err := m.restoreMetadata(meta); err != nil {
			return RestoreResult{}, fmt.Errorf("restore snapshot metadata %q: %w", latest.path, err)
		}
	}
	restored, err := m.engine.RestoreSnapshotEntries(latest.model.Entries, m.now())
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restore snapshot %q: %w", latest.path, err)
	}
	return RestoreResult{
		Result:          Result{Path: latest.path, Format: latest.model.Format, CreatedAt: latest.model.CreatedAt, EntryCount: len(latest.model.Entries), Checksum: latest.model.Checksum},
		RestoredEntries: restored,
	}, nil
}

// Start launches periodic snapshots until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	if !m.cfg.Enabled || m.cfg.SnapshotIntervalSec <= 0 {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Duration(m.cfg.SnapshotIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := m.Snapshot(); err != nil {
					logging.Warnf("persistence: periodic snapshot failed: %v", err)
				}
			}
		}
	}()
}

func (m *Manager) Wait() { m.wg.Wait() }

type validSnapshot struct {
	path  string
	model *snapshot.Model
}

func (m *Manager) validSnapshots() ([]validSnapshot, int, error, error) {
	entries, err := os.ReadDir(m.cfg.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil, nil
	}
	if err != nil {
		return nil, 0, nil, fmt.Errorf("read snapshot directory: %w", err)
	}
	valid := make([]validSnapshot, 0)
	candidates := 0
	var lastErr error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		format, ok := formatForFilename(entry.Name())
		if !ok {
			continue
		}
		candidates++
		path := filepath.Join(m.cfg.DataDir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			lastErr = err
			continue
		}
		model, decodeErr := snapshot.Decode(file, format)
		closeErr := file.Close()
		if decodeErr != nil {
			lastErr = fmt.Errorf("%s: %w", entry.Name(), decodeErr)
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if model.NodeID != m.nodeID {
			lastErr = fmt.Errorf("%s belongs to node %q", entry.Name(), model.NodeID)
			continue
		}
		if err := m.engine.ValidateSnapshotEntries(model.Entries); err != nil {
			lastErr = fmt.Errorf("%s has invalid entries: %w", entry.Name(), err)
			continue
		}
		valid = append(valid, validSnapshot{path: path, model: model})
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].model.CreatedAt.After(valid[j].model.CreatedAt) })
	return valid, candidates, lastErr, nil
}

func (m *Manager) pruneValidSnapshots() error {
	valid, _, _, err := m.validSnapshots()
	if err != nil {
		return err
	}
	if m.cfg.MaxSnapshots <= 0 || len(valid) <= m.cfg.MaxSnapshots {
		return nil
	}
	for _, old := range valid[m.cfg.MaxSnapshots:] {
		if err := os.Remove(old.path); err != nil {
			return fmt.Errorf("remove old snapshot %q: %w", old.path, err)
		}
	}
	return nil
}

func writeAtomic(path string, model *snapshot.Model) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary snapshot: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err = tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("set snapshot permissions: %w", err)
	}
	if err = snapshot.Encode(tmp, model); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit snapshot: %w", err)
	}
	return nil
}

func availableSnapshotPath(dir, name string) (string, error) {
	compoundExt := ".snapshot" + filepath.Ext(name)
	stem := strings.TrimSuffix(name, compoundExt)
	for suffix := 0; ; suffix++ {
		candidate := name
		if suffix > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, suffix, compoundExt)
		}
		path := filepath.Join(dir, candidate)
		_, err := os.Stat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return path, nil
		case err != nil:
			return "", fmt.Errorf("inspect snapshot path %q: %w", path, err)
		}
	}
}

func formatForFilename(name string) (string, bool) {
	if !strings.HasPrefix(name, "snapshot-") {
		return "", false
	}
	switch {
	case strings.HasSuffix(name, ".snapshot.json"):
		return snapshot.FormatJSON, true
	case strings.HasSuffix(name, ".snapshot.bin"):
		return snapshot.FormatBinary, true
	default:
		return "", false
	}
}
