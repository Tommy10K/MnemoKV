// Package resp implements a small subset of the Redis Serialization Protocol
// (RESP2). It is deliberately self-contained: nothing in this package knows
// about the storage engine, the cluster, or even the network listener. Higher
// layers compose Parser and Writer with their own I/O.
package resp

// Frame is the marker interface implemented by every RESP2 reply type.
// We use a sealed interface so the writer can dispatch with a type switch and
// callers cannot accidentally introduce new wire types.
type Frame interface {
	respFrame()
}

// SimpleString is `+OK\r\n` style.
type SimpleString string

// Error is `-ERR msg\r\n` style. Construct via NewError to keep the prefix
// convention consistent.
type Error struct {
	Prefix  string // e.g. "ERR", "WRONGTYPE"
	Message string
}

// Integer is `:123\r\n`.
type Integer int64

// BulkString is `$<len>\r\n<bytes>\r\n`. A nil Value encodes the RESP2
// null bulk `$-1\r\n`.
type BulkString struct {
	Value []byte
	Null  bool
}

// Array is `*<len>\r\n<frame>...`. A nil Items slice with Null=true encodes
// the null array `*-1\r\n`. An empty (non-null) slice encodes `*0\r\n`.
type Array struct {
	Items []Frame
	Null  bool
}

func (SimpleString) respFrame() {}

func (Error) respFrame() {}

func (Integer) respFrame() {}

func (BulkString) respFrame() {}

func (Array) respFrame() {}

// NewError builds an error frame with the given prefix and message.
func NewError(prefix, message string) Error {
	return Error{Prefix: prefix, Message: message}
}

// OK is the canonical success reply.
var OK = SimpleString("OK")

// Pong is the canonical PING reply.
var Pong = SimpleString("PONG")

// NullBulk is the canonical nil bulk-string reply.
var NullBulk = BulkString{Null: true}

// BulkBytes constructs a non-nil bulk string from a byte slice. The slice is
// not copied; callers must not mutate it after passing it to the writer.
func BulkBytes(b []byte) BulkString { return BulkString{Value: b} }

// BulkString constructs a non-nil bulk string from a Go string.
func BulkFromString(s string) BulkString { return BulkString{Value: []byte(s)} }
