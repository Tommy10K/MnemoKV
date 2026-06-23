package controlplane

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const dataNodeInitializedMarker = "data-node.initialized"

// MarkDataNodeInitialized records the first automatic-mode boot in the
// controller directory. Later boots are returning-node boots and must start
// behind the admission gate. The marker is deliberately independent of
// application snapshots.
func MarkDataNodeInitialized(raftDir string) (returning bool, err error) {
	if err := os.MkdirAll(raftDir, 0o750); err != nil {
		return false, fmt.Errorf("create controller directory: %w", err)
	}
	path := filepath.Join(raftDir, dataNodeInitializedMarker)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	switch {
	case err == nil:
		if closeErr := file.Close(); closeErr != nil {
			return false, fmt.Errorf("close data-node marker: %w", closeErr)
		}
		return false, nil
	case errors.Is(err, os.ErrExist):
		return true, nil
	default:
		return false, fmt.Errorf("create data-node marker: %w", err)
	}
}
