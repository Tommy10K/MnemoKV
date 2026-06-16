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

	opts, frame := parseSetOptions(cmd.Args[2:])
	if frame != nil {
		return frame
	}
	if !x.store.setString(key, value, opts.expiresAtNs, opts.condition) {
		return resp.NullBulk
	}
	return resp.OK
}

type setOptions struct {
	expiresAtNs int64
	condition   setCondition
}

func parseSetOptions(args [][]byte) (setOptions, resp.Frame) {
	opts := setOptions{condition: setAlways}
	hasExpiration := false
	for i := 0; i < len(args); i++ {
		switch upperASCII(args[i]) {
		case "EX":
			if hasExpiration || i+1 >= len(args) {
				return opts, resp.NewError("ERR", "syntax error")
			}
			secs, ok := parseInt64(args[i+1])
			if !ok {
				return opts, resp.NewError("ERR", "value is not an integer or out of range")
			}
			opts.expiresAtNs, ok = expirationFromNow(secs, int64(1_000_000_000))
			if !ok {
				return opts, resp.NewError("ERR", "invalid expire time in 'set' command")
			}
			hasExpiration = true
			i++
		case "PX":
			if hasExpiration || i+1 >= len(args) {
				return opts, resp.NewError("ERR", "syntax error")
			}
			ms, ok := parseInt64(args[i+1])
			if !ok {
				return opts, resp.NewError("ERR", "value is not an integer or out of range")
			}
			opts.expiresAtNs, ok = expirationFromNow(ms, int64(1_000_000))
			if !ok {
				return opts, resp.NewError("ERR", "invalid expire time in 'set' command")
			}
			hasExpiration = true
			i++
		case "NX":
			if opts.condition == setIfPresent {
				return opts, resp.NewError("ERR", "syntax error")
			}
			opts.condition = setIfMissing
		case "XX":
			if opts.condition == setIfMissing {
				return opts, resp.NewError("ERR", "syntax error")
			}
			opts.condition = setIfPresent
		default:
			return opts, resp.NewError("ERR", "syntax error")
		}
	}
	return opts, nil
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

// parseInt64 parses Redis's canonical base-10 integer representation. It
// accepts "0" and negative non-zero values, but rejects plus signs, leading
// zeros, negative zero, non-digits, and values outside int64.
func parseInt64(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	if len(b) == 1 && b[0] == '0' {
		return 0, true
	}
	i := 0
	if b[0] == '-' {
		if len(b) == 1 {
			return 0, false
		}
		i = 1
	}
	if b[i] < '1' || b[i] > '9' {
		return 0, false
	}
	for i++; i < len(b); i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, false
		}
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
