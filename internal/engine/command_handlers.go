package engine

import (
	"github.com/mnemokv/mnemokv/internal/resp"
)

// command_handlers.go owns the small handlers that are not specific to one
// data type: PING, ECHO, DEL, EXISTS, EXPIRE, TTL. The string handlers live
// in string_ops.go to keep this file focused.

func (x *Executor) cmdPing(cmd *resp.Command) resp.Frame {
	switch len(cmd.Args) {
	case 0:
		return resp.Pong
	case 1:
		return resp.BulkBytes(cmd.Args[0])
	default:
		return wrongArgs("ping")
	}
}

func (x *Executor) cmdEcho(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("echo")
	}
	return resp.BulkBytes(cmd.Args[0])
}

func (x *Executor) cmdDel(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 1 {
		return wrongArgs("del")
	}
	var deleted int64
	for _, k := range cmd.Args {
		if x.store.Delete(k) {
			deleted++
		}
	}
	return resp.Integer(deleted)
}

func (x *Executor) cmdExists(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 1 {
		return wrongArgs("exists")
	}
	var count int64
	for _, k := range cmd.Args {
		if x.store.Exists(k) {
			count++
		}
	}
	return resp.Integer(count)
}

func (x *Executor) cmdExpire(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 2 {
		return wrongArgs("expire")
	}
	seconds, ok := parseInt64(cmd.Args[1])
	if !ok {
		return resp.NewError("ERR", "value is not an integer or out of range")
	}
	if seconds <= 0 {
		// Redis behaviour: a non-positive TTL deletes the key immediately and
		// reports success only if the key existed.
		if x.store.Delete(cmd.Args[0]) {
			return resp.Integer(1)
		}
		return resp.Integer(0)
	}
	expiresAt := nowNanos() + seconds*int64(1_000_000_000)
	if x.store.SetExpireAt(cmd.Args[0], expiresAt) {
		return resp.Integer(1)
	}
	return resp.Integer(0)
}

func (x *Executor) cmdTTL(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("ttl")
	}
	e, ok := x.store.Get(cmd.Args[0])
	if !ok {
		return resp.Integer(-2)
	}
	if e.ExpiresAtNs == 0 {
		return resp.Integer(-1)
	}
	remaining := e.ExpiresAtNs - nowNanos()
	if remaining <= 0 {
		return resp.Integer(-2)
	}
	// Round up so a key that is still alive never reports 0 seconds.
	seconds := (remaining + int64(1_000_000_000) - 1) / int64(1_000_000_000)
	return resp.Integer(seconds)
}

// wrongArgs builds the canonical "wrong number of arguments" error.
func wrongArgs(cmdName string) resp.Frame {
	return resp.NewError("ERR", "wrong number of arguments for '"+cmdName+"' command")
}
