// Package engine is the in-memory storage and command execution layer.
//
// It owns the data model (entries, value types) and the dispatch for
// individual RESP commands. The package intentionally exposes concrete types
// (Store, Engine, Executor) on the hot path: indirection is reserved for
// genuine extension boundaries such as the eviction policy contract.
package engine

// ValueType tags an Entry with the kind of payload it carries. We keep the
// enum tiny and explicit so wrong-type checks cost a single int comparison.
type ValueType uint8

const (
	// ValueTypeNone is the zero value and indicates an unset entry. It should
	// never appear in a stored entry; encountering it is a programming error.
	ValueTypeNone ValueType = iota
	ValueTypeString
	ValueTypeList
	ValueTypeZSet
)

// String renders the type for log messages and tests.
func (v ValueType) String() string {
	switch v {
	case ValueTypeString:
		return "string"
	case ValueTypeList:
		return "list"
	case ValueTypeZSet:
		return "zset"
	default:
		return "none"
	}
}
