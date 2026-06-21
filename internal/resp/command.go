package resp

import (
	"bytes"
	"strings"
	"sync"
)

// Command is the parsed form of a single RESP request. It is allocated from a
// pool so the per-request cost stays low. Args[0] is the primary key for
// commands that operate on a single key.
type Command struct {
	Name string   // upper-cased command name
	Args [][]byte // command arguments, including the key

	// raw holds the underlying buffer the parser allocated for this command's
	// argument bytes. Pool.Release zeroes the slices but keeps the backing
	// arrays alive for the next request.
	raw [][]byte
}

// commandPool is the package-level pool. Connections share it because
// Command values are released back into the pool after the response is
// written; nothing in the engine retains a reference past Execute.
var commandPool = sync.Pool{
	New: func() any { return &Command{} },
}

// acquireCommand pulls a Command out of the pool ready to be filled in.
func acquireCommand() *Command {
	c := commandPool.Get().(*Command)
	c.Name = ""
	c.Args = c.Args[:0]
	return c
}

// Release returns the command to the pool. It is safe to call with a nil
// command. After release the caller must not touch the command.
func Release(c *Command) {
	if c == nil {
		return
	}
	for i := range c.Args {
		c.Args[i] = nil
	}
	c.Args = c.Args[:0]
	c.Name = ""
	commandPool.Put(c)
}

// Key returns the primary key argument for the command, or nil if the command
// takes no key. Supported key commands place their primary key in Args[0].
func (c *Command) Key() []byte {
	if len(c.Args) == 0 {
		return nil
	}
	return c.Args[0]
}

// ArgString returns the i-th argument as a string. It returns ("", false) if
// the index is out of range.
func (c *Command) ArgString(i int) (string, bool) {
	if i < 0 || i >= len(c.Args) {
		return "", false
	}
	return string(c.Args[i]), true
}

// EqualFoldArg reports whether the i-th argument equals the given ASCII
// string case-insensitively. It avoids allocating a Go string for the
// argument, which matters on the parse-and-dispatch hot path.
func (c *Command) EqualFoldArg(i int, s string) bool {
	if i < 0 || i >= len(c.Args) {
		return false
	}
	return equalFoldASCII(c.Args[i], s)
}

func equalFoldASCII(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := 0; i < len(b); i++ {
		bc := b[i]
		sc := s[i]
		if bc >= 'a' && bc <= 'z' {
			bc -= 'a' - 'A'
		}
		if sc >= 'a' && sc <= 'z' {
			sc -= 'a' - 'A'
		}
		if bc != sc {
			return false
		}
	}
	return true
}

// ExtractPrimaryKey returns the canonical key for routing decisions. Keeping
// this in one place means the cluster router never duplicates command parsing
// logic. For commands that have no key (e.g. PING) it returns nil.
func ExtractPrimaryKey(c *Command) []byte {
	keys := ExtractKeys(c)
	if len(keys) == 0 {
		return nil
	}
	return keys[0]
}

// ExtractKeys returns every key participating in a routing decision.
func ExtractKeys(c *Command) [][]byte {
	switch c.Name {
	case "PING", "ECHO", "QUIT", "FLUSHDB", "FLUSHALL", "COMMAND", "CLIENT", "HELLO",
		"REPLICATE", "CLUSTERMETA", "CLUSTERAPPLY", "CLUSTERSNAPSHOT":
		return nil
	case "DEL", "EXISTS":
		return c.Args
	default:
		if len(c.Args) == 0 {
			return nil
		}
		return c.Args[:1]
	}
}

// upper normalizes a command name to upper case using only ASCII rules.
// Unknown bytes pass through unchanged so we still emit a useful error.
func upper(b []byte) string {
	hasLower := false
	for _, c := range b {
		if c >= 'a' && c <= 'z' {
			hasLower = true
			break
		}
	}
	if !hasLower {
		return string(b)
	}
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// trimTrailingCRLF removes a trailing CR/LF pair if present. It is used by the
// inline-command parser, which lets users type commands directly into a raw
// telnet session.
func trimTrailingCRLF(b []byte) []byte {
	return bytes.TrimRight(b, "\r\n")
}

// splitInline splits an inline command on whitespace, mirroring Redis's
// behaviour for raw-text input. Quotes are not interpreted; this path is for
// human typing during debugging, not for client libraries.
func splitInline(line []byte) [][]byte {
	parts := strings.Fields(string(line))
	out := make([][]byte, len(parts))
	for i, p := range parts {
		out[i] = []byte(p)
	}
	return out
}
