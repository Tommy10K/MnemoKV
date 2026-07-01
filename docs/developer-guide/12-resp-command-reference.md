# 12. RESP Command Reference

This page lists every RESP command currently supported by MnemoKV. It is written for someone using
`redis-cli` or reading the command handlers for the first time.

Start a standalone node first:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

Then use:

```powershell
redis-cli -p 6380 <COMMAND> <ARGUMENTS>
```

The examples below show typical `redis-cli` output. MnemoKV command names are case-insensitive, so
`get key`, `GET key`, and `Get key` all mean the same thing.

## Quick Command List

| Group | Commands |
| --- | --- |
| Connection / compatibility | `PING`, `ECHO`, `QUIT`, `COMMAND`, `CLIENT`, `HELLO` |
| Keys | `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `FLUSHDB`, `FLUSHALL` |
| Strings | `SET`, `GET`, `INCR` |
| Lists | `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LLEN` |
| Sorted sets | `ZADD`, `ZRANGE`, `ZCARD`, `ZSCORE` |
| Internal cluster protocol | `REPLICATE`, `CLUSTERMETA`, `CLUSTERAPPLY`, `CLUSTERSNAPSHOT` |

## Response Types In Simple Terms

MnemoKV returns normal RESP2-style values:

- Simple string: usually `OK` or `PONG`.
- Bulk string: a stored string value, such as `"hello"`.
- Integer: a number, such as `(integer) 1`.
- Array: a list of returned values.
- Nil bulk: shown by `redis-cli` as `(nil)`.
- Error: shown with an error prefix, such as `(error) ERR ...`.

## Connection And Compatibility Commands

### `PING [message]`

Checks whether the server is reachable.

Arguments:

- No arguments: return `PONG`.
- Optional `message`: return that message.

Examples:

```powershell
redis-cli -p 6380 PING
```

Expected output:

```text
PONG
```

```powershell
redis-cli -p 6380 PING hello
```

Expected output:

```text
"hello"
```

### `ECHO message`

Returns the exact message you send.

Arguments:

- `message`: the text to echo.

Example:

```powershell
redis-cli -p 6380 ECHO diploma
```

Expected output:

```text
"diploma"
```

### `QUIT`

Returns `OK` and then closes the connection.

Arguments:

- None.

Example:

```powershell
redis-cli -p 6380 QUIT
```

Expected output:

```text
OK
```

### `COMMAND [DOCS ...]`

Compatibility command used by `redis-cli` when it connects. MnemoKV returns an empty array.

Arguments:

- Any arguments are accepted by the current implementation.

Example:

```powershell
redis-cli -p 6380 COMMAND
```

Expected output:

```text
(empty array)
```

### `CLIENT ...`

Compatibility command for simple `redis-cli` client setup. MnemoKV returns `OK`.

Arguments:

- Any arguments are accepted by the current implementation.

Example:

```powershell
redis-cli -p 6380 CLIENT SETNAME demo-client
```

Expected output:

```text
OK
```

### `HELLO ...`

RESP3 negotiation is not supported. MnemoKV is RESP2-only.

Arguments:

- Any arguments produce the same unsupported response.

Example:

```powershell
redis-cli -p 6380 HELLO 3
```

Expected output:

```text
(error) ERR HELLO not supported (RESP2 only)
```

## Key Commands

### `DEL key [key ...]`

Deletes one or more keys.

Arguments:

- `key`: one key to delete.
- Optional additional keys.

Returns:

- Number of keys actually deleted.

Examples:

```powershell
redis-cli -p 6380 SET a one
redis-cli -p 6380 SET b two
redis-cli -p 6380 DEL a b missing
```

Expected output from `DEL`:

```text
(integer) 2
```

```powershell
redis-cli -p 6380 GET a
```

Expected output:

```text
(nil)
```

### `EXISTS key [key ...]`

Checks whether keys exist.

Arguments:

- `key`: one key to check.
- Optional additional keys.

Returns:

- Count of existing keys. Duplicate keys count repeatedly, matching Redis behavior.

Example:

```powershell
redis-cli -p 6380 SET present yes
redis-cli -p 6380 EXISTS present missing
```

Expected output:

```text
(integer) 1
```

Example with duplicates:

```powershell
redis-cli -p 6380 EXISTS present present
```

Expected output:

```text
(integer) 2
```

### `EXPIRE key seconds`

Sets a time-to-live on a key.

Arguments:

- `key`: the key to expire.
- `seconds`: integer number of seconds.

Returns:

- `1` if the TTL was applied.
- `0` if the key does not exist.

Notes:

- `seconds <= 0` deletes the key immediately if it exists.

Examples:

```powershell
redis-cli -p 6380 SET temp alive
redis-cli -p 6380 EXPIRE temp 30
```

Expected output from `EXPIRE`:

```text
(integer) 1
```

```powershell
redis-cli -p 6380 EXPIRE missing 30
```

Expected output:

```text
(integer) 0
```

### `TTL key`

Shows how long a key has before it expires.

Arguments:

- `key`: the key to inspect.

Returns:

- `-2` if the key does not exist.
- `-1` if the key exists but has no TTL.
- A positive number of seconds if the key has a TTL.

Examples:

```powershell
redis-cli -p 6380 SET permanent value
redis-cli -p 6380 TTL permanent
```

Expected output:

```text
(integer) -1
```

```powershell
redis-cli -p 6380 SET temp value EX 60
redis-cli -p 6380 TTL temp
```

Expected output:

```text
(integer) 60
```

The exact number may be lower if time has passed.

### `FLUSHDB`

Deletes all keys from the local database.

Arguments:

- None.

Returns:

- `OK`.

Important:

- Supported only in standalone mode.
- Rejected in cluster mode because MnemoKV does not have a cluster-wide transaction for flushing
  every node safely.

Example:

```powershell
redis-cli -p 6380 SET a one
redis-cli -p 6380 FLUSHDB
redis-cli -p 6380 GET a
```

Expected output from `FLUSHDB`:

```text
OK
```

Expected output from `GET a`:

```text
(nil)
```

### `FLUSHALL`

Same behavior as `FLUSHDB` in MnemoKV: deletes all local keys.

Arguments:

- None.

Returns:

- `OK`.

Important:

- Supported only in standalone mode.
- Rejected in cluster mode.

Example:

```powershell
redis-cli -p 6380 SET a one
redis-cli -p 6380 FLUSHALL
redis-cli -p 6380 EXISTS a
```

Expected output from `EXISTS a`:

```text
(integer) 0
```

## String Commands

### `SET key value [EX seconds | PX milliseconds] [NX | XX]`

Stores a string value.

Arguments:

- `key`: key name.
- `value`: string value to store.
- Optional `EX seconds`: expire after seconds.
- Optional `PX milliseconds`: expire after milliseconds.
- Optional `NX`: set only if the key does not already exist.
- Optional `XX`: set only if the key already exists.

Returns:

- `OK` when the value is stored.
- `(nil)` when `NX` or `XX` prevents the write.

Notes:

- You can use at most one of `EX` or `PX`.
- You can use at most one of `NX` or `XX`.
- `KEEPTTL`, `EXAT`, and `PXAT` are not implemented.

Examples:

```powershell
redis-cli -p 6380 SET greeting hello
```

Expected output:

```text
OK
```

```powershell
redis-cli -p 6380 GET greeting
```

Expected output:

```text
"hello"
```

Conditional example:

```powershell
redis-cli -p 6380 SET only-once first NX
redis-cli -p 6380 SET only-once second NX
```

Expected output from the second command:

```text
(nil)
```

TTL example:

```powershell
redis-cli -p 6380 SET short-lived value EX 10
redis-cli -p 6380 TTL short-lived
```

Expected output from `TTL`:

```text
(integer) 10
```

The exact number may be lower if time has passed.

### `GET key`

Reads a string value.

Arguments:

- `key`: key to read.

Returns:

- Bulk string if the key exists and holds a string.
- `(nil)` if the key does not exist.
- `WRONGTYPE` error if the key holds a list or sorted set.

Examples:

```powershell
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
```

Expected output:

```text
"hello"
```

```powershell
redis-cli -p 6380 GET missing
```

Expected output:

```text
(nil)
```

### `INCR key`

Increments an integer string by 1.

Arguments:

- `key`: key to increment.

Returns:

- New integer value.

Notes:

- Missing keys are treated as `0`.
- Existing value must be a valid signed 64-bit integer string.
- Overflow returns an error.

Examples:

```powershell
redis-cli -p 6380 INCR visits
redis-cli -p 6380 INCR visits
```

Expected output from the second command:

```text
(integer) 2
```

Error example:

```powershell
redis-cli -p 6380 SET not-number hello
redis-cli -p 6380 INCR not-number
```

Expected output:

```text
(error) ERR value is not an integer or out of range
```

## List Commands

Lists are ordered sequences of string elements.

### `LPUSH key value [value ...]`

Pushes one or more values to the left/front of a list.

Arguments:

- `key`: list key.
- `value`: value to push.
- Optional additional values.

Returns:

- New list length.

Example:

```powershell
redis-cli -p 6380 LPUSH tasks one two three
```

Expected output:

```text
(integer) 3
```

```powershell
redis-cli -p 6380 LPOP tasks
```

Expected output:

```text
"three"
```

`LPUSH` pushes each value to the left, so the last value in the command becomes the first value
popped from the left.

### `RPUSH key value [value ...]`

Pushes one or more values to the right/back of a list.

Arguments:

- `key`: list key.
- `value`: value to push.
- Optional additional values.

Returns:

- New list length.

Example:

```powershell
redis-cli -p 6380 RPUSH queue first second third
```

Expected output:

```text
(integer) 3
```

```powershell
redis-cli -p 6380 LPOP queue
```

Expected output:

```text
"first"
```

### `LPOP key`

Removes and returns the left/front value.

Arguments:

- `key`: list key.

Returns:

- Bulk string for the popped value.
- `(nil)` if the list does not exist or is empty.

Example:

```powershell
redis-cli -p 6380 RPUSH queue first second
redis-cli -p 6380 LPOP queue
```

Expected output:

```text
"first"
```

### `RPOP key`

Removes and returns the right/back value.

Arguments:

- `key`: list key.

Returns:

- Bulk string for the popped value.
- `(nil)` if the list does not exist or is empty.

Example:

```powershell
redis-cli -p 6380 RPUSH queue first second
redis-cli -p 6380 RPOP queue
```

Expected output:

```text
"second"
```

### `LLEN key`

Returns the length of a list.

Arguments:

- `key`: list key.

Returns:

- Number of elements.
- `0` if the key does not exist.

Example:

```powershell
redis-cli -p 6380 RPUSH queue first second
redis-cli -p 6380 LLEN queue
```

Expected output:

```text
(integer) 2
```

## Sorted Set Commands

Sorted sets store unique members ordered by numeric score. If two members have the same score, they
are ordered by member text.

### `ZADD key score member [score member ...]`

Adds or updates members in a sorted set.

Arguments:

- `key`: sorted-set key.
- `score`: floating-point score. `NaN` is rejected. `inf` and `-inf` are accepted.
- `member`: member name.
- Optional additional score/member pairs.

Returns:

- Number of newly added members.
- Updating an existing member's score does not count as newly added.

Examples:

```powershell
redis-cli -p 6380 ZADD leaderboard 10 alice 20 bob
```

Expected output:

```text
(integer) 2
```

```powershell
redis-cli -p 6380 ZADD leaderboard 30 alice
```

Expected output:

```text
(integer) 0
```

Alice already existed, so her score was updated but no new member was added.

### `ZRANGE key start stop [WITHSCORES]`

Returns members ordered by score.

Arguments:

- `key`: sorted-set key.
- `start`: zero-based start index.
- `stop`: zero-based stop index. Negative indexes count from the end; `-1` means the last element.
- Optional `WITHSCORES`: include scores after each member.

Returns:

- Array of members.
- With `WITHSCORES`, array alternates member, score, member, score.
- Empty array if the key does not exist.

Examples:

```powershell
redis-cli -p 6380 ZADD leaderboard 10 alice 20 bob 15 carol
redis-cli -p 6380 ZRANGE leaderboard 0 -1
```

Expected output:

```text
1) "alice"
2) "carol"
3) "bob"
```

With scores:

```powershell
redis-cli -p 6380 ZRANGE leaderboard 0 -1 WITHSCORES
```

Expected output:

```text
1) "alice"
2) "10"
3) "carol"
4) "15"
5) "bob"
6) "20"
```

### `ZCARD key`

Returns the number of members in a sorted set.

Arguments:

- `key`: sorted-set key.

Returns:

- Number of members.
- `0` if the key does not exist.

Example:

```powershell
redis-cli -p 6380 ZADD leaderboard 10 alice 20 bob
redis-cli -p 6380 ZCARD leaderboard
```

Expected output:

```text
(integer) 2
```

### `ZSCORE key member`

Returns the score for one member.

Arguments:

- `key`: sorted-set key.
- `member`: member name.

Returns:

- Bulk string score if the member exists.
- `(nil)` if the key or member does not exist.

Examples:

```powershell
redis-cli -p 6380 ZADD leaderboard 10 alice
redis-cli -p 6380 ZSCORE leaderboard alice
```

Expected output:

```text
"10"
```

```powershell
redis-cli -p 6380 ZSCORE leaderboard missing
```

Expected output:

```text
(nil)
```

## Cluster Behavior For Normal Commands

In cluster mode, clients can connect to any node's RESP port:

```powershell
redis-cli -p 6381 SET cluster:key value
redis-cli -p 6382 GET cluster:key
redis-cli -p 6383 GET cluster:key
```

MnemoKV hashes the key to a fixed slot and routes the command to that slot's leader.

Important cluster rules:

- Commands with one key are routed by that key.
- Commands with multiple keys, such as `DEL a b`, are allowed only when all keys hash to the same
  slot.
- If a multi-key command uses different slots, MnemoKV returns a `CROSSSLOT` error.
- `FLUSHDB` and `FLUSHALL` are rejected in cluster mode.
- Writes are rejected if the assigned replica is unavailable or not ready.

Example cross-slot result:

```powershell
redis-cli -p 6381 DEL key:one key:two
```

Expected output when the keys map to different slots:

```text
(error) CROSSSLOT keys in request do not hash to the same slot
```

## Internal Cluster Commands

These commands are part of MnemoKV's internal node-to-node protocol. They are listed here so the
code is understandable, but users should not type them manually.

| Command | Used for |
| --- | --- |
| `REPLICATE` | Leader sends an ordered write record to a slot replica. |
| `CLUSTERMETA` | A node asks another node for its current cluster metadata. |
| `CLUSTERAPPLY` | A node applies a newer metadata snapshot from another node. |
| `CLUSTERSNAPSHOT` | A leader sends a full-slot data snapshot to a replacement replica. |

These are intercepted by the cluster coordinator before normal engine dispatch. They are not normal
user commands.

## Common Errors

Wrong number of arguments:

```powershell
redis-cli -p 6380 GET
```

Expected output:

```text
(error) ERR wrong number of arguments for 'get' command
```

Wrong type:

```powershell
redis-cli -p 6380 RPUSH list a
redis-cli -p 6380 GET list
```

Expected output:

```text
(error) WRONGTYPE Operation against a key holding the wrong kind of value
```

Unknown command:

```powershell
redis-cli -p 6380 MGET a b
```

Expected output:

```text
(error) ERR unknown command 'MGET'
```
