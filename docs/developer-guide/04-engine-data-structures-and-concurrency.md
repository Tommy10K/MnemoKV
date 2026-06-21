# 4. Engine, Data Structures, And Concurrency

## Engine Ownership

[`internal/engine/engine.go`](../../internal/engine/engine.go) groups the store, executor, memory
tracker, eviction manager, metrics sink, admission mutex, and optional cluster write hook.

`Engine.Execute` sends writes through memory admission and sends reads directly to the executor.
`Engine.ApplyReplicated` bypasses admission and write hooks because the leader already admitted and
ordered the mutation.

## Lock-Striped Dictionary

[`internal/engine/store.go`](../../internal/engine/store.go) is a hash table divided into independent
[`Stripe`](../../internal/engine/stripe.go) objects. Each stripe owns a `map[string]*Entry` and a
`sync.RWMutex`. FNV-1a maps a key to one stripe.

The practical result is:

- Operations on the same key serialize under the same stripe lock.
- Operations on unrelated stripes can proceed concurrently.
- The store avoids one global lock for ordinary reads and writes.
- Multi-stripe operations such as snapshotting and flushing lock stripes in a stable order.

The configured stripe count is honored. A power-of-two count uses a mask; other counts use modulo.

## Entry Model And TTL

[`internal/engine/entry.go`](../../internal/engine/entry.go) stores:

- key and value type;
- the concrete string, list, or sorted-set value;
- absolute expiration in Unix nanoseconds;
- approximate accounted size;
- creation, update, and last-access timestamps;
- access frequency and version.

Expiration is lazy. A command touching an expired key removes it under the stripe lock and treats it
as missing. There is no background expiration scanner.

`Store.Get` updates access metadata; `Store.Peek` does not. Value reads such as `GET`, pops,
`ZRANGE`, and `ZSCORE` protect a key under LRU/LFU. Administrative reads such as `EXISTS`, `TTL`,
`LLEN`, and `ZCARD` do not.

The store returns an `Entry` snapshot rather than the mutable entry pointer. This avoids races with
metadata updates after the stripe lock is released.

## Strings

[`internal/engine/string_value.go`](../../internal/engine/string_value.go) wraps immutable copied
bytes. `Store.setString` checks NX/XX and writes while holding one stripe lock, making condition and
mutation atomic. `Store.IncrementBy` parses, overflow-checks, and replaces the string in the same
critical section.

`SET` parsing in [`string_ops.go`](../../internal/engine/string_ops.go) accepts one EX or PX option
and one NX or XX condition. Relative TTL calculation is overflow-safe.

## Lists

[`internal/engine/list_value.go`](../../internal/engine/list_value.go) implements a doubly linked
list with head and tail pointers. Push and pop at either end are O(1). The list has its own mutex,
so container mutation remains safe even when callers hold only a copied `Entry`.

The owning stripe lock protects entry replacement and memory-size reconciliation. The list mutex
protects node links and length. Keep this lock layering in mind when adding list commands.

## Sorted Sets

[`internal/engine/zset_value.go`](../../internal/engine/zset_value.go) combines:

- a member-to-score map for direct lookup;
- a probabilistic skip list for ordered traversal.

[`internal/engine/skiplist.go`](../../internal/engine/skiplist.go) provides expected O(log n)
insertion/deletion and ordered ranges. Equal scores are ordered by member text. Updating a member's
score removes its old skip-list node and inserts a new one. `NaN` is rejected because it cannot
participate in a total ordering; infinities are allowed.

## Go Concurrency Techniques Used

- `sync.RWMutex` protects maps and container internals.
- `sync.Mutex` serializes admission and ordered replication.
- `atomic.Uint64` exposes memory usage without taking every stripe lock.
- Per-connection goroutines isolate parser and writer state.
- Copies of byte slices prevent client buffers from becoming mutable stored values.
- Stable lock order prevents deadlocks during whole-dataset snapshots and replacement.

## Correctness Tests To Read

- [`engine_test.go`](../../internal/engine/engine_test.go): strings, TTL, arity, races, and atomic NX.
- [`list_test.go`](../../internal/engine/list_test.go): list behavior and concurrency.
- [`zset_test.go`](../../internal/engine/zset_test.go): ordering, score validation, and concurrency.
- [`snapshot_test.go`](../../internal/engine/snapshot_test.go): value encoding and restore.

When adding a command, first decide its arity, missing-key result, wrong-type result, access-metadata
behavior, write classification, memory delta, cluster key extraction, and snapshot implications.
