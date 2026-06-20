// Package snapshot defines the versioned logical snapshot shared by all
// persistence codecs.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const FormatVersion uint32 = 1

const (
	FormatJSON   = "json"
	FormatBinary = "binary"
)

// Model is the complete logical state persisted for one node.
type Model struct {
	Format          string    `json:"format"`
	FormatVersion   uint32    `json:"formatVersion"`
	NodeID          string    `json:"nodeId"`
	ClusterID       string    `json:"clusterId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	SlotCount       uint32    `json:"slotCount,omitempty"`
	MetadataVersion uint64    `json:"metadataVersion,omitempty"`
	Slots           []Slot    `json:"slots,omitempty"`
	Entries         []Entry   `json:"entries"`
	Checksum        string    `json:"checksum"`
}

// ClusterMetadata is supplied by the cluster package when cluster mode is
// active. Standalone snapshots use its zero value.
type ClusterMetadata struct {
	ClusterID       string
	SlotCount       uint32
	MetadataVersion uint64
	Slots           []Slot
}

// Slot captures the local node's durable view of one cluster slot.
type Slot struct {
	Number              uint32 `json:"number"`
	Role                string `json:"role"`
	Term                uint64 `json:"term"`
	LastAppliedSequence uint64 `json:"lastAppliedSequence"`
}

// Entry contains one engine value in a type-specific byte representation.
type Entry struct {
	Key         string `json:"key"`
	ValueType   string `json:"valueType"`
	Value       []byte `json:"value"`
	ApproxSize  uint64 `json:"approxSize"`
	ExpiresAtNs int64  `json:"expiresAtNs"`
}

// Seal canonicalizes the model and attaches its SHA-256 checksum.
func (m *Model) Seal() error {
	if err := m.validateMetadata(false); err != nil {
		return err
	}
	m.Checksum = m.expectedChecksum()
	return nil
}

// Verify validates the model and its checksum.
func (m *Model) Verify() error {
	if err := m.validateMetadata(true); err != nil {
		return err
	}
	expected := m.expectedChecksum()
	if m.Checksum != expected {
		return fmt.Errorf("snapshot checksum mismatch: got %q, want %q", m.Checksum, expected)
	}
	return nil
}

func (m *Model) validateMetadata(requireChecksum bool) error {
	if m.Format != FormatJSON && m.Format != FormatBinary {
		return fmt.Errorf("unsupported snapshot format %q", m.Format)
	}
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported snapshot format version %d", m.FormatVersion)
	}
	if m.NodeID == "" {
		return fmt.Errorf("snapshot node ID is empty")
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("snapshot creation timestamp is empty")
	}
	if requireChecksum {
		if len(m.Checksum) != sha256.Size*2 {
			return fmt.Errorf("snapshot checksum is not a SHA-256 digest")
		}
		if _, err := hex.DecodeString(m.Checksum); err != nil {
			return fmt.Errorf("snapshot checksum is invalid: %w", err)
		}
	}

	keys := make(map[string]struct{}, len(m.Entries))
	for _, entry := range m.Entries {
		if _, exists := keys[entry.Key]; exists {
			return fmt.Errorf("snapshot contains duplicate key %q", entry.Key)
		}
		keys[entry.Key] = struct{}{}
		switch entry.ValueType {
		case "string", "list", "zset":
		default:
			return fmt.Errorf("snapshot key %q has unsupported value type %q", entry.Key, entry.ValueType)
		}
	}

	seenSlots := make(map[uint32]struct{}, len(m.Slots))
	for _, slot := range m.Slots {
		if m.SlotCount == 0 || slot.Number >= m.SlotCount {
			return fmt.Errorf("snapshot slot %d is outside slot count %d", slot.Number, m.SlotCount)
		}
		if _, exists := seenSlots[slot.Number]; exists {
			return fmt.Errorf("snapshot contains duplicate slot %d", slot.Number)
		}
		seenSlots[slot.Number] = struct{}{}
		switch slot.Role {
		case "leader", "replica", "none":
		default:
			return fmt.Errorf("snapshot slot %d has unsupported role %q", slot.Number, slot.Role)
		}
	}
	return nil
}

func (m *Model) expectedChecksum() string {
	canonical := *m
	canonical.Checksum = ""
	canonical.CreatedAt = canonical.CreatedAt.UTC()
	canonical.Entries = append([]Entry(nil), m.Entries...)
	canonical.Slots = append([]Slot(nil), m.Slots...)
	sort.Slice(canonical.Entries, func(i, j int) bool { return canonical.Entries[i].Key < canonical.Entries[j].Key })
	sort.Slice(canonical.Slots, func(i, j int) bool { return canonical.Slots[i].Number < canonical.Slots[j].Number })
	raw, _ := json.Marshal(canonical)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
