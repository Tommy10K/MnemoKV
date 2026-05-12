# ADR 002: Command Semantics

## Status

Accepted (baseline milestone).

## Context

Commands must behave predictably and compatibly with `redis-cli`. Locking semantics up front avoids divergence between handlers.

## Decision

### Common rules

- Command names are case-insensitive on the wire and normalized to uppercase internally.
- Wrong-type operations return `WRONGTYPE Operation against a key holding the wrong kind of value`.
- Operating on a missing key returns the type-appropriate empty result (nil bulk for `GET`, `0` for counters, etc.).
- Lazy expiration: any read or write that touches an expired key first deletes it and treats it as missing.

### Per-command semantics (baseline)

- `PING` — returns `PONG`. With one argument, returns the argument as a bulk string.
- `ECHO message` — returns the message as a bulk string.
- `SET key value [EX seconds | PX milliseconds] [NX | XX]`
  - `EX`/`PX` set absolute expiration.
  - `NX` sets only if the key is absent; `XX` only if present.
  - Returns `OK` on success, nil bulk if `NX`/`XX` conditions fail.
  - A successful `SET` without `KEEPTTL` clears any previous TTL. (Baseline does not implement `KEEPTTL`.)
- `GET key` — returns the string value, nil bulk if missing or wrong type. Wrong type returns `WRONGTYPE`.
- `INCR key`
  - If missing, treats current value as `0`.
  - Value must be a base-10 64-bit signed integer; otherwise `ERR value is not an integer or out of range`.
  - Returns the new integer.
  - Overflow (beyond `int64`) returns `ERR increment or decrement would overflow`.
- `DEL key [key ...]` — returns the number of keys actually deleted.
- `EXISTS key [key ...]` — returns the count of existing keys (duplicates count repeatedly).
- `EXPIRE key seconds`
  - Returns `1` if the TTL was applied, `0` if the key does not exist.
  - Negative or zero seconds delete the key immediately and return `1`.
- `TTL key`
  - Returns `-2` if the key does not exist.
  - Returns `-1` if the key has no associated expiration.
  - Otherwise the remaining time in whole seconds (rounded up so a key never reports `0` while still alive).
- `FLUSHDB` — clears all keys, returns `OK`. In standalone mode it acts on the local store. (Cluster fan-out is deferred.)
- `COMMAND [DOCS ...]` — returns an empty array. This is enough for `redis-cli` to start interactively.
- `CLIENT ...` — returns `OK`. Sufficient for `redis-cli` `CLIENT SETNAME` and `CLIENT GETNAME` probes.
- `QUIT` — returns `OK`, then closes the connection.

Unknown commands return `ERR unknown command '<name>'`.
