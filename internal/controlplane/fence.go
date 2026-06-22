package controlplane

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrStaleControlIndex    = errors.New("stale control index")
	ErrControlIndexConflict = errors.New("control index was already used for a different operation")
)

const fenceStateFilename = "fence-state.json"

type fenceState struct {
	ControlIndex    uint64 `json:"controlIndex"`
	OperationDigest string `json:"operationDigest"`
}

// FenceStore serializes authenticated topology operations and persists the
// highest accepted control index independently from application snapshots.
type FenceStore struct {
	mu    sync.Mutex
	path  string
	state fenceState
}

func OpenFenceStore(raftDir string) (*FenceStore, error) {
	if raftDir == "" {
		return nil, errors.New("raft directory is empty")
	}
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		return nil, fmt.Errorf("create control-plane directory: %w", err)
	}
	store := &FenceStore{path: filepath.Join(raftDir, fenceStateFilename)}
	raw, err := os.ReadFile(store.path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &store.state); err != nil {
			return nil, fmt.Errorf("decode fencing state: %w", err)
		}
		if store.state.ControlIndex > 0 {
			digest, decodeErr := hex.DecodeString(store.state.OperationDigest)
			if decodeErr != nil || len(digest) != 32 {
				return nil, errors.New("decode fencing state: invalid operation digest")
			}
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return nil, fmt.Errorf("read fencing state: %w", err)
	}
	return store, nil
}

// Begin holds the store lock until release is called, serializing the
// corresponding topology mutation with control-index acceptance.
func (s *FenceStore) Begin(index uint64, digest [32]byte) (replay bool, release func(), err error) {
	s.mu.Lock()
	release = s.mu.Unlock
	if index == 0 || index < s.state.ControlIndex {
		release()
		return false, nil, ErrStaleControlIndex
	}
	digestText := hex.EncodeToString(digest[:])
	if index == s.state.ControlIndex {
		if digestText != s.state.OperationDigest {
			release()
			return false, nil, ErrControlIndexConflict
		}
		return true, release, nil
	}
	next := fenceState{ControlIndex: index, OperationDigest: digestText}
	if err := s.persist(next); err != nil {
		release()
		return false, nil, err
	}
	s.state = next
	return false, release, nil
}

func (s *FenceStore) State() (uint64, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.ControlIndex, s.state.OperationDigest
}

func (s *FenceStore) persist(state fenceState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open temporary fencing state: %w", err)
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(temporary)
		}
	}()
	if _, err := file.Write(raw); err != nil {
		return fmt.Errorf("write fencing state: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync fencing state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close fencing state: %w", err)
	}
	if err := os.Rename(temporary, s.path); err != nil {
		return fmt.Errorf("replace fencing state: %w", err)
	}
	cleanup = false
	return nil
}
