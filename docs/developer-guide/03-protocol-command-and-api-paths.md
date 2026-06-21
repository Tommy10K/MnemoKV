# 3. RESP, HTTP, And Command Request Paths

## One Command Model, Two Public Transports

RESP clients and HTTP clients both produce `resp.Command` values and consume the same active
executor. This is the key integration rule: transport code parses and formats; the engine or
cluster coordinator decides behavior.

## RESP2 Path

The TCP flow is:

```text
client socket
  -> server.connectionHandler
  -> resp.Parser.Next
  -> resp.Command
  -> engine.Engine or cluster.Coordinator
  -> resp.Frame
  -> resp.Writer
  -> client socket
```

Start with [`internal/server/server.go`](../../internal/server/server.go). It binds the address,
enforces `network.maxConnections`, tracks sockets for shutdown, and creates one goroutine per
accepted connection.

[`internal/server/connection.go`](../../internal/server/connection.go) owns the per-connection
buffered reader, parser, writer, and deadlines. Nothing in that object is shared across connection
goroutines. It treats an empty line as ignorable, returns a safe error for an empty command, closes
on framing errors, and closes after a valid `QUIT` response.

[`internal/resp/parser.go`](../../internal/resp/parser.go) accepts RESP arrays and a limited inline
debug form. It bounds argument count, argument size, and aggregate request size. Parsed commands
come from [`internal/resp/pool.go`](../../internal/resp/pool.go); the connection loop releases them
after execution to reduce allocation pressure.

[`internal/resp/frame.go`](../../internal/resp/frame.go) defines simple strings, errors, integers,
bulk strings, and arrays. [`internal/resp/writer.go`](../../internal/resp/writer.go) serializes them
and sanitizes error lines so user data cannot inject additional RESP frames.

## Command Dispatch

[`internal/engine/executor.go`](../../internal/engine/executor.go) uses a direct switch. The command
set is deliberately small, so a switch keeps the hot path and stack traces easy to follow.

Handlers are split by concern:

- [`command_handlers.go`](../../internal/engine/command_handlers.go): utility and generic key commands.
- [`string_ops.go`](../../internal/engine/string_ops.go): `SET`, `GET`, and `INCR`.
- [`list_ops.go`](../../internal/engine/list_ops.go): list commands.
- [`zset_ops.go`](../../internal/engine/zset_ops.go): sorted-set commands.
- [`commands.go`](../../internal/engine/commands.go): which commands mutate state.

Read [ADR 002](../adr/002-command-semantics.md) before changing visible behavior. Important details
include atomic `SET NX/XX`, canonical integers, overflow-safe expiration, lazy expiration, strict
sorted-set options, and utility-command arity.

## Key Extraction And Cluster Routing

[`internal/resp/command.go`](../../internal/resp/command.go) normalizes command names and extracts
keys. The cluster coordinator uses this to hash commands without duplicating parsing rules. A
multi-key command is legal in cluster mode only when all extracted keys hash to one slot.

Commands with no key, such as `PING`, execute locally. Internal cluster commands are intercepted by
the coordinator and never exposed as ordinary engine operations.

## HTTP Path

The HTTP flow for `/commands` is:

```text
JSON {"args":[...]}
  -> request size/method/JSON validation
  -> resp.Command
  -> same active executor as RESP
  -> resp.Frame
  -> typed JSON command result
```

[`internal/api/server.go`](../../internal/api/server.go) creates the standard-library HTTP server
and CORS wrapper. [`internal/api/routes.go`](../../internal/api/routes.go) is the route inventory.
[`internal/api/request.go`](../../internal/api/request.go) enforces methods, a 1 MiB body limit,
unknown-field rejection, and exactly one JSON value.

[`internal/api/commands.go`](../../internal/api/commands.go) converts arguments to a `resp.Command`
and recursively converts the resulting frame into browser-friendly JSON. Engine errors remain a
successful HTTP response containing `{type: "error"}` because they are command results, not
transport failures.

## HTTP And SSE Surface

| Method | Path | Implementation |
| --- | --- | --- |
| GET | `/health` | node identity and mode in `handlers.go` |
| GET | `/engine/state` | memory and eviction state in `handlers.go` |
| GET | `/metrics/summary` | counter snapshot in `handlers.go` |
| GET | `/cluster/state` | authoritative slots plus local membership hints |
| GET | `/events` | periodic SSE observability snapshots in `websocket.go` |
| POST | `/commands` | command execution in `commands.go` |
| POST | `/engine/eviction-policy` | live future-policy switch in `eviction.go` |
| POST | `/admin/snapshot` | manual snapshot in `snapshot.go` |
| POST | `/cluster/promote` | manual promotion in `cluster_admin.go` |
| POST | `/cluster/replica` | replacement assignment in `cluster_admin.go` |
| POST | `/cluster/sync` | full-slot repair in `cluster_admin.go` |

The file is named `websocket.go` for historical reasons, but the current implementation is
server-sent events, not WebSocket.

## Trace Exercise

Set a breakpoint or add temporary logging at these points for `SET example value`:

1. `connectionHandler.serve` or `Server.handleCommands`.
2. `Coordinator.Execute` in cluster mode.
3. `Engine.Execute` and `executeWithAdmission`.
4. `Executor.cmdSet`.
5. `Store.setString`.

That trace shows the intended separation between transport, routing, admission, command semantics,
and storage.
