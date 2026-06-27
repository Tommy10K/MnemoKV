# ADR 002: Command Semantics

## Status

Accepted and implemented.

## Context

Commands must behave predictably and compatibly with `redis-cli`. Locking semantics up front avoids
divergence between handlers.

## Decision

### Common rules

- Command names are case-insensitive on the wire and normalized to uppercase internally.
- Wrong-type operations return `WRONGTYPE Operation against a key holding the wrong kind of value`.
- Operating on a missing key returns the type-appropriate empty result (nil bulk for `GET`, `0` for
  counters, etc.).
- Lazy expiration: any read or write that touches an expired key first deletes it and treats it as
  missing.
- Integer arguments use canonical signed base-10 syntax: `0` or a non-zero value without a plus
  sign or leading zeros. Negative zero is rejected.
- Relative expiration arithmetic rejects multiplication or timestamp overflow rather than wrapping.

### Per-command semantics

- `PING` - returns `PONG`. With one argument, returns the argument as a bulk string.
- `ECHO message` - returns the message as a bulk string.
- `SET key value [EX seconds | PX milliseconds] [NX | XX]`
  - `EX`/`PX` set absolute expiration.
  - `NX` sets only if the key is absent; `XX` only if present.
  - Returns `OK` on success, nil bulk if `NX`/`XX` conditions fail.
  - A successful `SET` without `KEEPTTL` clears any previous TTL. `KEEPTTL` is not implemented.
  - The existence condition and write occur atomically under the owning stripe lock.
  - At most one expiration option is accepted; repeated or conflicting `EX`/`PX` is a syntax error.
- `GET key` - returns the string value, nil bulk if missing, and `WRONGTYPE` for another value type.
- `INCR key`
  - If missing, treats current value as `0`.
  - Value must be a base-10 64-bit signed integer; otherwise `ERR value is not an integer or out of range`.
  - Returns the new integer.
  - Overflow beyond `int64` returns `ERR increment or decrement would overflow`.
- `DEL key [key ...]` - returns the number of keys actually deleted.
- `EXISTS key [key ...]` - returns the count of existing keys; duplicate keys count repeatedly.
- `EXPIRE key seconds`
  - Returns `1` if the TTL was applied, `0` if the key does not exist.
  - Negative or zero seconds delete the key immediately and return `1`.
- `TTL key`
  - Returns `-2` if the key does not exist.
  - Returns `-1` if the key has no associated expiration.
  - Otherwise returns the remaining time in whole seconds, rounded up so a key never reports `0`
    while still alive.
- `FLUSHDB` and `FLUSHALL` - clear all local keys and return `OK` in standalone mode. Cluster mode
  rejects them because no cluster-wide transaction protocol exists.
- `COMMAND [DOCS ...]` - returns an empty array. This is enough for `redis-cli` to start interactively.
- `CLIENT ...` - returns `OK`. This is sufficient for `redis-cli` `CLIENT SETNAME` and
  `CLIENT GETNAME` probes.
- `QUIT` - returns `OK`, then closes the connection.
- `QUIT`, `FLUSHDB`, and `FLUSHALL` reject extra arguments. An invalid `QUIT` does not close the
  connection.

### Sorted-set validation

- `ZADD` rejects `NaN` because it has no total ordering. Positive and negative infinity are valid
  scores.
- Duplicate members in one `ZADD` are processed in order and count as newly added at most once.
- `ZRANGE` accepts exactly `key start stop` or the same arguments followed by `WITHSCORES`.

Unknown commands return `ERR unknown command '<name>'`.
