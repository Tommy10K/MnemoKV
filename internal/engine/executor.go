package engine

import (
	"github.com/mnemokv/mnemokv/internal/resp"
)

// Executor dispatches a parsed RESP command to the appropriate handler. The
// dispatch is a single switch instead of a map of function pointers because:
//
//   - The set of commands is small and known at compile time.
//   - A switch keeps stack traces meaningful.
//   - Inlining stays simple, so the hot path stays predictable.
type Executor struct {
	store *Store
}

func newExecutor(store *Store) *Executor {
	return &Executor{store: store}
}

// Execute dispatches the command. It always returns a non-nil frame.
func (x *Executor) Execute(cmd *resp.Command) resp.Frame {
	switch cmd.Name {
	// Connection / utility
	case "PING":
		return x.cmdPing(cmd)
	case "ECHO":
		return x.cmdEcho(cmd)
	case "QUIT":
		// QUIT's "close the connection after replying" behaviour is enforced by
		// the connection loop; here we just produce the OK reply.
		return resp.OK
	case "COMMAND":
		// redis-cli sends COMMAND DOCS on connect. An empty array is enough to
		// keep the interactive client happy.
		return resp.Array{Items: []resp.Frame{}}
	case "CLIENT":
		return resp.OK
	case "HELLO":
		// We do not negotiate RESP3; reply with an error so clients fall back
		// to plain RESP2.
		return resp.NewError("ERR", "HELLO not supported (RESP2 only)")
	case "FLUSHDB", "FLUSHALL":
		x.store.Flush()
		return resp.OK

	// Generic key commands
	case "DEL":
		return x.cmdDel(cmd)
	case "EXISTS":
		return x.cmdExists(cmd)
	case "EXPIRE":
		return x.cmdExpire(cmd)
	case "TTL":
		return x.cmdTTL(cmd)

	// String commands
	case "SET":
		return x.cmdSet(cmd)
	case "GET":
		return x.cmdGet(cmd)
	case "INCR":
		return x.cmdIncr(cmd)

	// List commands
	case "LPUSH":
		return x.cmdLPush(cmd)
	case "RPUSH":
		return x.cmdRPush(cmd)
	case "LPOP":
		return x.cmdLPop(cmd)
	case "RPOP":
		return x.cmdRPop(cmd)
	case "LLEN":
		return x.cmdLLen(cmd)
	}

	return resp.NewError("ERR", "unknown command '"+cmd.Name+"'")
}
