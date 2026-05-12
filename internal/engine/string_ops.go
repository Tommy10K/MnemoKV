package engine

import (
	"errors"
	"strconv"

	"github.com/mnemokv/mnemokv/internal/resp"
)

// cmdSet implements `SET key value [EX seconds | PX milliseconds] [NX | XX]`.
// We deliberately keep the option set narrow at the baseline: KEEPTTL and
// EXAT/PXAT can be added later when the data model needs them.
func (x *Executor) cmdSet(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 2 {
		return wrongArgs("set")
	}
	key := cmd.Args[0]
	value := cmd.Args[1]

	var (
		expiresAtNs int64
		nx, xx      bool
	)

	for i := 2; i < len(cmd.Args); i++ {
		switch upperASCII(cmd.Args[i]) {
		case "EX":
			if i+1 >= len(cmd.Args) {
				return resp.NewError("ERR", "syntax error")
			}
			secs, ok := parseInt64(cmd.Args[i+1])
			if !ok || secs <= 0 {
				return resp.NewError("ERR", "invalid expire time in 'set' command")
			}
			expiresAtNs = nowNanos() + secs*int64(1_000_000_000)
			i++
		case "PX":
			if i+1 >= len(cmd.Args) {
				return resp.NewError("ERR", "syntax error")
			}
			ms, ok := parseInt64(cmd.Args[i+1])
			if !ok || ms <= 0 {
				return resp.NewError("ERR", "invalid expire time in 'set' command")
			}
			expiresAtNs = nowNanos() + ms*int64(1_000_000)
			i++
		case "NX":
			nx = true
		case "XX":
			xx = true
		default:
			return resp.NewError("ERR", "syntax error")
		}
	}
	if nx && xx {
		return resp.NewError("ERR", "syntax error")
	}

	if nx || xx {
		exists := x.store.Exists(key)
		if nx && exists {
			return resp.NullBulk
		}
		if xx && !exists {
			return resp.NullBulk
		}
	}

	entry := &Entry{
		Key:         string(key),
		Type:        ValueTypeString,
		Value:       NewStringValue(append([]byte(nil), value...)),
		SizeBytes:   stringEntrySize(key, value),
		ExpiresAtNs: expiresAtNs,
	}
	x.store.Put(entry)
	return resp.OK
}

func (x *Executor) cmdGet(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("get")
	}
	e, ok := x.store.Get(cmd.Args[0])
	if !ok {
		return resp.NullBulk
	}
	if e.Type != ValueTypeString {
		return resp.NewError("WRONGTYPE", "Operation against a key holding the wrong kind of value")
	}
	sv := e.Value.(*StringValue)
	// Copy so a later mutation by another goroutine cannot tear the bytes
	// we hand to the writer. The cost is one allocation per GET, which is
	// acceptable for the baseline.
	out := make([]byte, len(sv.Data))
	copy(out, sv.Data)
	return resp.BulkBytes(out)
}

// cmdIncr implements INCR. The actual mutation lives in store.IncrementBy so
// the operation is atomic under the stripe lock; this handler only translates
// engine errors into RESP error frames.
func (x *Executor) cmdIncr(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("incr")
	}
	next, err := x.store.IncrementBy(cmd.Args[0], 1)
	switch {
	case err == nil:
		return resp.Integer(next)
	case errors.Is(err, ErrWrongType):
		return resp.NewError("WRONGTYPE", "Operation against a key holding the wrong kind of value")
	case errors.Is(err, ErrNotInteger):
		return resp.NewError("ERR", "value is not an integer or out of range")
	case errors.Is(err, ErrIntOverflow):
		return resp.NewError("ERR", "increment or decrement would overflow")
	default:
		return resp.NewError("ERR", err.Error())
	}
}

// parseInt64 parses a base-10 signed integer from a byte slice. It rejects
// leading zeros only when followed by more digits (so "0" is fine but "01"
// is not, matching Redis).
func parseInt64(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// upperASCII returns the upper-case form of an ASCII byte slice. It allocates
// a new string only if at least one character actually needs to change.
func upperASCII(b []byte) string {
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
