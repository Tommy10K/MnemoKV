# MnemoKV Next Steps

Last reviewed against the codebase: June 20, 2026.

## Current Assessment

MnemoKV now has a technically solid common command core. The same engine and RESP path used by
standalone and clustered nodes has focused coverage for conditional writes, expiration parsing,
integer handling, sorted-set validation, concurrent reads, malformed RESP input, and utility
command arity.

Standalone mode is the stable product path. Cluster mode remains an experimental prototype:
useful components exist, but they do not yet form one authoritative distributed write path.

This file is the execution plan for the next backend phase. Treat unchecked items as decided work
unless they are explicitly marked as future work.

## Backend

### Common To Single Node And Cluster

#### Completed correctness work

- [x] Made `SET NX` and `SET XX` atomic under one stripe lock.
- [x] Rejected conflicting or repeated `EX`/`PX` options.
- [x] Added overflow-safe expiration calculation for `SET` and `EXPIRE`.
- [x] Made integer parsing match Redis-style canonical integers: no plus sign, leading zeros, or negative zero.
- [x] Rejected `NaN` sorted-set scores while explicitly allowing positive and negative infinity.
- [x] Made `ZRANGE` accept only its documented arities and `WITHSCORES` option.
- [x] Kept duplicate-member `ZADD` behavior atomic and counted each newly added member once.
- [x] Removed mutable-entry races between reads such as `GET`/`TTL` and writes such as `INCR`/`EXPIRE`.
- [x] Enforced arity for `QUIT`, `FLUSHDB`, and `FLUSHALL`; invalid `QUIT` no longer closes the connection.
- [x] Added RESP argument-count and aggregate-size limits, empty-command handling, and safe error-line serialization.

#### Remaining common work

- [x] Change access-metadata updates so only value-accessing or mutating commands affect eviction
  recency/frequency. `GET`, `LPOP`, `RPOP`, `ZRANGE`, `ZSCORE`, and successful writes must update
  access metadata. `EXISTS`, `TTL`, `LLEN`, and `ZCARD` must not protect a key from LRU/LFU
  eviction.
- [x] Add HTTP request-body limits, reject trailing JSON, and enforce method checks consistently
  across API handlers.
- [x] Consolidate `cmd.total` and `commands_total` into one public metric vocabulary. `cmd.total`
  is canonical; the TCP server no longer emits `commands_total`.
- [x] Enforce `network.maxConnections` in the TCP accept path.
- [x] Apply `observability.logLevel` to backend logging.
- [x] Fix the benchmark export fields and add separate hot-key and distributed-key parallel
  benchmarks.

#### Hard memory limit and eviction decision

The configured `memoryLimitBytes` will be a hard limit on MnemoKV's accounted dataset size. It is
not a promise about total Go process memory, because command buffers, temporary allocations, and
garbage-collector overhead are outside the store's approximate accounting.

The dataset must never be committed above the configured limit. A memory-growing write is admitted
only after enough capacity already exists. Reads never trigger eviction.

Policy behavior:

- `memoryLimitBytes: 0` means unlimited dataset memory and disables eviction.
- Rename `noop` to `noeviction`.
- `noeviction` never deletes keys automatically. It rejects a memory-growing write when there is
  insufficient capacity, while deletes, expirations, and size-reducing updates remain allowed.
- FIFO, LRU, LFU, and Random use the same admission process. They differ only in how they choose
  victims when capacity must be freed.
- If an individual value cannot fit within the entire configured limit, reject it without evicting
  unrelated keys.

Write admission order:

1. Parse and validate the command without changing data.
2. Evaluate conditions such as `SET NX` and `SET XX`. A command that will not write must not evict.
3. Estimate the new accounted entry size and calculate only the positive growth:
   `requiredGrowth = max(0, newSize - currentSize)`.
4. If `requiredGrowth` is zero, apply the write directly and release any reduced memory afterward.
5. Serialize memory-growing admission with one engine-level admission mutex.
6. Recheck current usage while holding that mutex.
7. With `noeviction`, return an OOM-style error if the required growth does not fit.
8. With an eviction policy, evict bounded sample batches in a loop until enough capacity exists.
9. Stop and reject the write if an eviction pass makes no progress or a configured safety bound is reached.
10. Commit the write while admission is still protected, then release the admission mutex.

The admission loop is required because one sampled batch may contain keys that are too small,
concurrently expired, already deleted, or otherwise unable to free the requested number of bytes.
It must detect no-progress conditions so it cannot loop forever.

#### Eviction implementation checklist

- [x] Rename the public policy value `noop` to `noeviction` in config validation, policy resolution,
  API responses, frontend controls, documentation, and tests.
- [x] Replace ADR 003's soft-limit decision with the hard accounted-dataset limit described above.
- [x] Define one canonical OOM RESP error and expose the equivalent structured HTTP command result.
- [x] Add an engine-level admission mutex for memory-growing writes.
- [x] Introduce a write-planning step that validates a command and reports its expected positive
  memory delta before mutation.
- [x] Implement exact or conservative delta planning for `SET`, `INCR`, list pushes, and `ZADD`.
- [x] Keep `DEL`, expiration, pops, flushes, and size-reducing updates available while at the limit.
- [x] Reject a single value larger than the entire limit before evicting any existing key.
- [x] Make `noeviction` reject writes without deleting existing data.
- [x] Change FIFO, LRU, LFU, and Random admission to evict before committing the incoming write.
- [x] Run eviction only for memory-growing writes, never for reads or commands that fail validation.
- [x] Implement a bounded eviction loop that rechecks available capacity after each batch and stops
  when enough space exists or no progress is possible.
- [x] Exclude the key being updated from eviction candidate selection during that write's own
  admission pass.
- [x] Preserve atomic `SET NX`/`XX` behavior while adding admission planning and locking.
- [x] Ensure concurrent writers cannot independently pass the capacity check and exceed the limit.
- [x] Track metrics for rejected writes, eviction attempts, evicted keys, and bytes freed.
- [x] Show active limit, used bytes, available bytes, policy, and rejected-write count in observability responses.
- [x] In cluster mode, make the leader choose victims and replicate explicit deletions so followers
  do not independently evict different keys.

#### Eviction acceptance tests

- [x] Accounted dataset memory never exceeds a positive configured limit, including under concurrent writes.
- [x] `noeviction` preserves all existing keys and rejects the write that would cross the limit.
- [x] A failed `NX`, failed `XX`, syntax error, invalid type, or oversized value causes no eviction.
- [x] Updating a value uses only the positive size delta rather than charging the full replacement size.
- [x] Size-reducing updates and memory-releasing commands succeed while the store is full.
- [x] Every active policy frees enough capacity before the incoming write becomes visible.
- [x] Eviction terminates with an error when no candidate or no-progress condition prevents admission.
- [x] Reads cannot cause eviction.
- [x] Race tests cover concurrent admission, deletion, expiration, and policy switching.
- [x] Cluster tests verify that replicated eviction decisions produce the same key set on leader and followers.

#### Snapshot persistence shared by standalone and cluster

Persistence is in scope for both standalone and cluster mode. Implement snapshot persistence, not a
write-ahead log.

Target config:

```yaml
persistence:
  enabled: true
  dataDir: ./data/node-1
  snapshotIntervalSec: 60
  maxSnapshots: 5
  loadOnStart: true
  format: json # json | binary
```

Snapshot rules:

- [x] Add a `persistence` config section with `enabled`, `dataDir`, `snapshotIntervalSec`,
  `maxSnapshots`, `loadOnStart`, and `format`.
- [x] Support `format: json` and `format: binary` with one shared logical snapshot model.
- [x] Include format version, node ID, cluster ID when present, creation timestamp, entries, value
  type, value bytes, approximate size, absolute expiration timestamp, and checksum.
- [x] In cluster snapshots, also include slot count, metadata version, per-slot role, term, and last
  applied sequence.
- [x] Write snapshots atomically using a temporary file followed by rename.
- [x] Keep only the newest `maxSnapshots` valid snapshots.
- [x] On startup, load the latest valid snapshot when `loadOnStart` is true.
- [x] Skip expired keys during restore.
- [x] Add a manual snapshot trigger through both `adminctl` and the HTTP admin API.
- [x] Validate that JSON and binary snapshots restore the same dataset.

### Single Node

Single-node mode is already the reliable MnemoKV lab. Remaining work is primarily operational and
demo hardening rather than architectural repair.

- [ ] Add a repeatable end-to-end demo script for strings, TTL, lists, sorted sets, workloads, metrics, and malformed commands.
- [ ] Add a low-memory standalone preset demonstrating both `noeviction` rejection and one active eviction policy.
- [ ] Add a standalone persistence preset demonstrating JSON and binary snapshot restore.
- [ ] Add HTTP/API smoke coverage for invalid methods, oversized bodies, trailing JSON, and unavailable backends.
- [ ] Update the root `README.md` to match the current standalone startup and frontend workflow.

### Cluster

#### What the problem is

The cluster currently has several individually tested components but no single authoritative data
path connecting them:

1. The hash ring can calculate an owner, but live RESP and HTTP commands execute on the receiving node.
2. Slot leaders are seeded through a different ownership calculation, so routing, fencing, replication, and status can disagree.
3. Async fencing happens after local mutation, so a rejected non-leader write can already have changed data.
4. Replication sends command arguments but does not enforce source, slot, term, sequence, duplicate, or ordering metadata on followers.
5. `strong` mode is synchronous fan-out, not quorum commit or consensus. A follower can apply a write that later fails elsewhere.
6. Membership is all-to-all heartbeat polling, not gossip convergence.
7. Elections are local decisions without voting, deterministic agreement, replica freshness, or split-brain protection.
8. Returning nodes have no snapshot/catch-up process before serving traffic.

The central design problem is that ownership is represented in more than one place and no agreed
log or control plane orders changes. Adding more retries or flags around the current fan-out will
not create safe failover.

#### Decided cluster shape

MnemoKV targets a small, explicit cluster design instead of a fully general distributed database.
The supported cluster size for the next phase is two to five nodes. Accepting and testing more than
five nodes is future work, and the product must not claim that behavior until it is covered.

Every node is both a data node and a cluster gateway. Clients can connect to any RESP port or API
port. If the contacted node is not the owner for a key, it proxies the command to the current slot
leader and relays the response.

Each node has its own YAML file. The files share the same cluster section but differ in `node.id`,
RESP port, API port, and data directory. A node knows it is `node-1`, `node-2`, and so on from its
own `node.id`.

Example target configuration:

```yaml
cluster:
  enabled: true
  shardingEnabled: true
  replicationEnabled: true
  slotCount: 1024
  routingMode: proxy
  failoverMode: manual
  peers:
    - id: node-1
      address: 127.0.0.1:6381
      apiAddress: 127.0.0.1:7381
    - id: node-2
      address: 127.0.0.1:6382
      apiAddress: 127.0.0.1:7382
    - id: node-3
      address: 127.0.0.1:6383
      apiAddress: 127.0.0.1:7383

persistence:
  enabled: true
  dataDir: ./data/node-1
  snapshotIntervalSec: 60
  maxSnapshots: 5
  loadOnStart: true
  format: json # json | binary
```

#### Slot ownership model

Partitioning is implicit when `shardingEnabled` is true. Do not expose a separate
`partitioning.enabled` option.

Use a fixed slot count, defaulting to `1024`. This is enough granularity for a small educational
cluster, easy to display, and cheaper to reason about than Redis's production-sized `16384` slots.

Ownership is computed deterministically from the shared peer list:

1. Sort peers by node ID.
2. Split `slotCount` as evenly as possible across peers.
3. Each node is leader for its primary slot range.
4. If replication is enabled, each primary range has exactly one replica: the next node in the
   sorted peer ring.

For three nodes:

```text
node-1 leads slots A, replicated by node-2
node-2 leads slots B, replicated by node-3
node-3 leads slots C, replicated by node-1
```

Each node stores the complete cluster metadata:

- cluster ID and metadata version
- slot count
- peer IDs and addresses
- slot leader
- slot replica
- slot term
- local role per slot: leader, replica, or none
- last applied sequence per slot

Healthy nodes must converge on the same metadata. Without Raft, metadata changes are manual admin
operations with a monotonically increasing version/term and stale-update rejection.

#### Routing model

Use `routingMode: proxy` for the first real implementation.

Command flow:

1. Client sends a command to any node.
2. The receiving node extracts the command key and computes its slot.
3. If the local node is the slot leader, it executes the command.
4. If another node is the slot leader, it forwards the command to that leader over the internal RESP proxy.
5. The leader returns the result; the gateway relays it to the client.

Multi-key commands are allowed only when all keys map to the same slot. Otherwise return a
`CROSSSLOT`-style error. Hash tag support such as `user:{42}:cart` is future work; the next phase
uses whole-key slot hashing only.

Do not implement redirect routing for now. It is more scalable, but proxy routing works with normal
`redis-cli`, the existing frontend, and simple HTTP clients.

#### Replication model

Replication is fixed and synchronous for the first reliable version:

- one leader and one replica per slot range
- no user-facing async/sync choice
- no configurable replication factor
- write succeeds only after leader and replica both apply or acknowledge the record

If the replica for a slot is unavailable, writes for that slot are rejected instead of silently
running under-replicated. This is stricter, but it keeps the safety story clear:

> If a write returns OK, it exists on the leader and the replica.

Replication records must include:

- slot
- term
- sequence number
- source leader ID
- command payload

Followers must accept records only for the current term and next expected sequence. They must
reject stale terms, ignore duplicates, and report gaps so the leader can repair them with a shard
snapshot.

#### Failure behavior without Raft

Manual failover is the target. Automatic failover remains out of scope until a real quorum metadata
plane exists.

If a node fails, two categories of slots are affected:

1. Slots where the failed node was leader become unavailable until an admin promotes the surviving
   replica.
2. Slots where the failed node was replica become write-unavailable while strict replication is
   required, unless an admin assigns and syncs a replacement replica.

Unaffected slots continue serving normally.

Manual repair flow:

1. Inspect cluster status.
2. Promote surviving replicas for slots whose leader failed.
3. Increment the affected slot terms so the old leader is fenced if it returns.
4. Assign replacement replicas for under-replicated slots.
5. Sync each replacement replica from the current leader using a full shard snapshot.
6. Resume writes for repaired slots.

When a failed node returns, it must not automatically reclaim leadership. It loads its snapshot,
observes newer terms, steps down for stale slots, and rejoins only after catch-up or full snapshot
repair.

#### Rebalancing model

Do not automatically resize or rebalance slots when a node fails.

Failure handling, repair, and rebalancing are separate operations:

- failover restores availability by promoting existing replicas
- repair restores the one-leader-one-replica safety property
- rebalancing restores even load after a node returns or a new node is added

A surviving node may temporarily lead more than one slot range. That is acceptable during failure
recovery. Automatic resizing would require coordinated metadata updates and data movement while the
cluster is already degraded, which is too risky without Raft.

Manual rebalance is future work. When it is implemented, it must be an admin-planned operation:

1. Produce a rebalance plan.
2. Copy slot data to the target node.
3. Pause or fence writes for the moving slots.
4. Switch slot leadership and replica metadata with a new term.
5. Resume writes.
6. Delete obsolete copies after the new owner is verified.

#### Snapshot persistence

Persistence is required in both standalone and cluster mode. It is snapshot persistence, not
a full write-ahead log.

Support both snapshot formats:

- `json`: readable, easy to debug, good for demos
- `binary`: smaller and faster, better for larger datasets

Both formats must represent the same logical snapshot:

- format name and version
- node ID
- cluster ID
- created timestamp
- slot count
- metadata version
- per-slot role, term, and last applied sequence
- entries with type, value, approximate size, and absolute expiration timestamp
- checksum

Snapshot retention:

- write new snapshots atomically through a temporary file and rename
- keep the newest `maxSnapshots`
- load the latest valid snapshot on start when `loadOnStart` is true
- skip expired keys on restore

For cluster repair, the first implementation must use full shard snapshot transfer instead of a
long write-ahead log. This is less efficient, but simpler and sufficient for clusters of up to five
nodes.

#### Cluster implementation checklist

- [ ] Replace separate ring and slot-leader ownership with one fixed-slot metadata model.
- [ ] Add `slotCount`, `routingMode: proxy`, and `failoverMode: manual` to config.
- [ ] Remove user-facing async/sync write-safety choices from the target cluster design.
- [ ] Make each node compute identical slot leadership and replica assignment from the peer list.
- [ ] Store and expose cluster metadata version, slot term, leader, replica, and local role.
- [ ] Put a command coordinator in front of both RESP and HTTP command execution.
- [ ] Implement proxy routing to the slot leader for single-key commands.
- [ ] Reject multi-key commands whose keys map to different slots.
- [ ] Add ordered synchronous replication with slot, term, source, and sequence.
- [ ] Reject writes when the slot leader is unavailable.
- [ ] Reject writes when the slot replica is unavailable and replication is enabled.
- [ ] Add manual admin operations through both `adminctl` and the HTTP admin API for cluster status,
  leader promotion, replica assignment, and replica sync.
- [ ] Persist cluster metadata and node snapshots.
- [ ] Implement snapshot codecs for both JSON and binary.
- [ ] Implement full shard snapshot transfer for replica repair and node rejoin.
- [ ] Ensure leader-chosen eviction decisions replicate as explicit deletes.

#### Cluster acceptance tests

- [ ] Every node computes the same slot map from the same peer list.
- [ ] A client can connect to any node and successfully write a key through proxy routing.
- [ ] The same key has one authoritative leader regardless of entry node.
- [ ] Multi-key commands across slots return a `CROSSSLOT`-style error.
- [ ] A write returns OK only after the replica acknowledges it.
- [ ] A leader failure makes its slots unavailable until manual promotion.
- [ ] A replica failure makes affected leader slots reject writes until repair.
- [ ] Manual promotion increments term and fences the old leader when it returns.
- [ ] Replica assignment plus full shard snapshot repair restores write availability.
- [ ] Restart from JSON and binary snapshots restores the same dataset.
- [ ] Rejoining stale nodes do not serve old leadership terms.
- [ ] Cluster-mode eviction produces the same key set on leader and replica.

## Frontend

### Completed

- [x] Repaired dependency metadata so `npm ci`, lint, and build pass.
- [x] Added a persisted API target selector and direct Configure-to-Dashboard link.
- [x] Corrected node startup commands and eviction/noeviction guidance.
- [x] Marked cluster behavior as experimental and local to the reporting node.
- [x] Replaced invented `recovering` states with `unknown` when observations are missing.
- [x] Corrected learning chapters to distinguish theory from current implementation.
- [x] Added route-level lazy loading and removed the oversized initial-chunk warning.

### Remaining

- [ ] Add browser smoke tests for navigation, API switching, offline states, dashboard data, and the command console.
- [ ] Runtime-validate API responses or provide clearer unexpected-shape errors.
- [ ] Review responsive layout at laptop and projector sizes.
- [ ] Complete keyboard, focus, label, contrast, chart, and reduced-motion accessibility checks.
- [ ] Add a built-in benchmark example and a reproducible demo dataset.
- [ ] Keep cluster wording synchronized with each backend milestone rather than updating claims ahead of behavior.

## Final Verification

Before a final release:

```text
go test ./...
go test -race ./...
go vet ./...
npm ci
npm run lint
npm run build
```

Also run manual standalone scenarios for every command family and malformed RESP input. Cluster
claims require multi-process routing, replication-ordering, peer-loss, restart, and partition tests.
