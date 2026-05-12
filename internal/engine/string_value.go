package engine

// StringValue is the engine's representation of a Redis-style string value.
// Storing it as a struct (rather than a bare []byte) gives us a stable place
// to add metadata later (encoding hints, integer caching, etc.) without
// rewriting every handler.
type StringValue struct {
	Data []byte
}

// NewStringValue wraps b in a StringValue. The caller transfers ownership of
// b; nothing else may mutate it after the call.
func NewStringValue(b []byte) *StringValue {
	return &StringValue{Data: b}
}

// stringEntrySize approximates the memory cost of a string entry. Constants
// here are conservative and match the simple model documented in the
// memory-and-eviction ADR.
const stringEntryOverhead = 64 // bytes for entry struct + map slot estimate

func stringEntrySize(key, value []byte) uint64 {
	return uint64(stringEntryOverhead + len(key) + len(value))
}
