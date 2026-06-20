package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mnemokv/mnemokv/internal/snapshot"
)

// SnapshotEntries returns a stable, key-sorted copy of the current unexpired
// dataset without changing eviction access metadata.
func (e *Engine) SnapshotEntries() ([]snapshot.Entry, error) {
	e.admissionMu.Lock()
	defer e.admissionMu.Unlock()
	return e.store.snapshotEntries(nowNanos())
}

// RestoreSnapshotEntries atomically replaces the in-memory dataset. Expired
// entries are skipped and the configured memory limit is enforced.
func (e *Engine) RestoreSnapshotEntries(entries []snapshot.Entry, now time.Time) (int, error) {
	e.admissionMu.Lock()
	defer e.admissionMu.Unlock()

	decoded := make([]*Entry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	var total uint64
	for _, item := range entries {
		if _, exists := seen[item.Key]; exists {
			return 0, fmt.Errorf("restore: duplicate key %q", item.Key)
		}
		seen[item.Key] = struct{}{}
		if item.ExpiresAtNs != 0 && item.ExpiresAtNs <= now.UnixNano() {
			continue
		}

		entry, err := decodeSnapshotEntry(item, now.UnixNano())
		if err != nil {
			return 0, err
		}
		if ^uint64(0)-total < entry.SizeBytes {
			return 0, fmt.Errorf("restore: dataset size overflows uint64")
		}
		total += entry.SizeBytes
		decoded = append(decoded, entry)
	}
	if e.memory.HasLimit() && total > e.memory.Limit() {
		return 0, fmt.Errorf("restore: snapshot uses %d bytes, exceeding memory limit %d", total, e.memory.Limit())
	}

	e.store.replaceAll(decoded, total)
	return len(decoded), nil
}

// ValidateSnapshotEntries checks type encodings and accounted sizes without
// mutating the engine. Persistence uses it to distinguish valid snapshots
// from files whose outer checksum is valid but whose value payload is not.
func (e *Engine) ValidateSnapshotEntries(entries []snapshot.Entry) error {
	seen := make(map[string]struct{}, len(entries))
	for _, item := range entries {
		if _, exists := seen[item.Key]; exists {
			return fmt.Errorf("snapshot contains duplicate key %q", item.Key)
		}
		seen[item.Key] = struct{}{}
		if _, err := decodeSnapshotEntry(item, 0); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) snapshotEntries(nowNs int64) ([]snapshot.Entry, error) {
	for _, stripe := range s.stripes {
		stripe.mu.RLock()
	}
	defer func() {
		for i := len(s.stripes) - 1; i >= 0; i-- {
			s.stripes[i].mu.RUnlock()
		}
	}()

	entries := make([]snapshot.Entry, 0)
	for _, stripe := range s.stripes {
		for _, entry := range stripe.entries {
			if entry.IsExpired(nowNs) {
				continue
			}
			value, err := encodeSnapshotValue(entry)
			if err != nil {
				return nil, fmt.Errorf("snapshot key %q: %w", entry.Key, err)
			}
			entries = append(entries, snapshot.Entry{
				Key:         entry.Key,
				ValueType:   entry.Type.String(),
				Value:       value,
				ApproxSize:  entry.SizeBytes,
				ExpiresAtNs: entry.ExpiresAtNs,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

func (s *Store) replaceAll(entries []*Entry, total uint64) {
	for _, stripe := range s.stripes {
		stripe.mu.Lock()
	}
	defer func() {
		for i := len(s.stripes) - 1; i >= 0; i-- {
			s.stripes[i].mu.Unlock()
		}
	}()

	for _, stripe := range s.stripes {
		stripe.entries = make(map[string]*Entry, 64)
	}
	for _, entry := range entries {
		stripe := s.stripeFor([]byte(entry.Key))
		stripe.entries[entry.Key] = entry
	}
	s.usedBytes.Store(total)
}

func encodeSnapshotValue(entry *Entry) ([]byte, error) {
	switch entry.Type {
	case ValueTypeString:
		value, ok := entry.Value.(*StringValue)
		if !ok || value == nil {
			return nil, fmt.Errorf("invalid string value")
		}
		return append([]byte(nil), value.Data...), nil
	case ValueTypeList:
		value, ok := entry.Value.(*ListValue)
		if !ok || value == nil {
			return nil, fmt.Errorf("invalid list value")
		}
		return encodeListValue(value)
	case ValueTypeZSet:
		value, ok := entry.Value.(*ZSetValue)
		if !ok || value == nil {
			return nil, fmt.Errorf("invalid sorted-set value")
		}
		return encodeZSetValue(value)
	default:
		return nil, fmt.Errorf("unsupported value type %q", entry.Type.String())
	}
}

func decodeSnapshotEntry(item snapshot.Entry, nowNs int64) (*Entry, error) {
	entry := &Entry{
		Key: item.Key, SizeBytes: item.ApproxSize, ExpiresAtNs: item.ExpiresAtNs,
		CreatedAtNs: nowNs, UpdatedAtNs: nowNs, LastAccessNs: nowNs, AccessCount: 1, Version: 1,
	}
	var calculated uint64
	switch item.ValueType {
	case "string":
		entry.Type = ValueTypeString
		entry.Value = NewStringValue(append([]byte(nil), item.Value...))
		calculated = stringEntrySize([]byte(item.Key), item.Value)
	case "list":
		value, err := decodeListValue(item.Value)
		if err != nil {
			return nil, fmt.Errorf("restore key %q: %w", item.Key, err)
		}
		entry.Type = ValueTypeList
		entry.Value = value
		calculated = listEntrySize(item.Key, value)
	case "zset":
		value, err := decodeZSetValue(item.Value)
		if err != nil {
			return nil, fmt.Errorf("restore key %q: %w", item.Key, err)
		}
		entry.Type = ValueTypeZSet
		entry.Value = value
		calculated = zsetEntrySize(item.Key, value)
	default:
		return nil, fmt.Errorf("restore key %q: unsupported value type %q", item.Key, item.ValueType)
	}
	if calculated != item.ApproxSize {
		return nil, fmt.Errorf("restore key %q: approximate size %d does not match calculated size %d", item.Key, item.ApproxSize, calculated)
	}
	return entry, nil
}

func encodeListValue(value *ListValue) ([]byte, error) {
	value.mu.RLock()
	defer value.mu.RUnlock()
	if uint64(value.len) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("list has too many elements")
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint32(value.len))
	for node := value.head; node != nil; node = node.Next {
		if uint64(len(node.Value)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("list element is too large")
		}
		_ = binary.Write(&out, binary.BigEndian, uint32(len(node.Value)))
		_, _ = out.Write(node.Value)
	}
	return out.Bytes(), nil
}

func decodeListValue(raw []byte) (*ListValue, error) {
	r := bytes.NewReader(raw)
	var count uint32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return nil, fmt.Errorf("decode list length: %w", err)
	}
	if uint64(count) > uint64(r.Len())/4 {
		return nil, fmt.Errorf("decode list: element count %d exceeds payload size", count)
	}
	value := NewListValue()
	items := make([][]byte, 0, int(count))
	for i := uint32(0); i < count; i++ {
		item, err := readLengthPrefixed(r)
		if err != nil {
			return nil, fmt.Errorf("decode list element %d: %w", i, err)
		}
		items = append(items, item)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("decode list: %d trailing bytes", r.Len())
	}
	value.RPush(items...)
	return value, nil
}

func encodeZSetValue(value *ZSetValue) ([]byte, error) {
	members := value.Range(0, -1)
	if uint64(len(members)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("sorted set has too many members")
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.BigEndian, uint32(len(members)))
	for _, member := range members {
		if uint64(len(member.Member)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("sorted-set member is too large")
		}
		_ = binary.Write(&out, binary.BigEndian, math.Float64bits(member.Score))
		_ = binary.Write(&out, binary.BigEndian, uint32(len(member.Member)))
		_, _ = out.WriteString(member.Member)
	}
	return out.Bytes(), nil
}

func decodeZSetValue(raw []byte) (*ZSetValue, error) {
	r := bytes.NewReader(raw)
	var count uint32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return nil, fmt.Errorf("decode sorted-set length: %w", err)
	}
	if uint64(count) > uint64(r.Len())/12 {
		return nil, fmt.Errorf("decode sorted set: member count %d exceeds payload size", count)
	}
	value := NewZSetValue()
	for i := uint32(0); i < count; i++ {
		var scoreBits uint64
		if err := binary.Read(r, binary.BigEndian, &scoreBits); err != nil {
			return nil, fmt.Errorf("decode sorted-set score %d: %w", i, err)
		}
		score := math.Float64frombits(scoreBits)
		if math.IsNaN(score) {
			return nil, fmt.Errorf("decode sorted-set score %d: NaN is not supported", i)
		}
		member, err := readLengthPrefixed(r)
		if err != nil {
			return nil, fmt.Errorf("decode sorted-set member %d: %w", i, err)
		}
		value.Add(score, string(member))
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("decode sorted set: %d trailing bytes", r.Len())
	}
	return value, nil
}

func readLengthPrefixed(r *bytes.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if uint64(length) > uint64(r.Len()) {
		return nil, fmt.Errorf("declared length %d exceeds remaining %d bytes", length, r.Len())
	}
	value := make([]byte, int(length))
	if _, err := r.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}
