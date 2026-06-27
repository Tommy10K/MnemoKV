# 5. Memory Limits, Eviction, And Observability

## What The Memory Limit Means

MnemoKV limits accounted dataset bytes, not total Go process memory. Command buffers, goroutine
stacks, map overhead beyond the estimate, temporary allocations, and garbage-collector overhead are
outside the promise. Read [ADR 003](../adr/003-memory-and-eviction-semantics.md) before changing it.

[`internal/engine/memory.go`](../../internal/engine/memory.go) reads the store's atomic used-byte
counter and exposes limit, available bytes, and usage ratio. A limit of zero means unlimited.

Each value type has an explicit size estimate:

- strings: key + bytes + fixed entry overhead;
- lists: key + entry overhead + node overhead and element bytes;
- sorted sets: key + entry overhead + estimated member/index nodes.

Every create, update, expiration, pop, delete, flush, restore, and eviction adjusts the same store
counter.

## Admission Before Mutation

[`internal/engine/admission.go`](../../internal/engine/admission.go) serializes memory-growing writes
with `admissionMu`. The sequence is:

1. Parse and validate the write without mutating data.
2. Evaluate NX/XX and wrong-type conditions.
3. Estimate only positive growth relative to the existing entry.
4. Reject a single resulting entry larger than the whole limit.
5. If needed, free capacity according to the active policy.
6. In cluster mode, synchronously replicate selected eviction deletes and the write.
7. Execute the local mutation while admission is still serialized.

Invalid or conditional no-op writes do not evict. Size-reducing writes and deletes remain available
at the limit.

## Policy Separation

[`internal/engine/eviction`](../../internal/engine/eviction/) contains policy objects. A policy only
selects victims from sampled candidates:

- `noeviction`: selects nothing, causing OOM rejection.
- FIFO: oldest creation time first.
- LRU: oldest last-access time first.
- LFU: lowest access count first.
- random: accepts the store's unordered sample without ranking by metadata.

[`eviction/manager.go`](../../internal/engine/eviction/manager.go) owns the active policy and samples
the store. The engine owns deletion and admission. This separation prevents policy code from
mutating storage directly.

The admission loop is bounded to 64 passes and detects no progress. It excludes the key currently
being updated so the engine cannot evict that key while trying to replace it.

In cluster mode, the leader chooses victims and replicates explicit `DEL` commands. Replicas never
make independent random or LRU decisions, so acknowledged leader and replica datasets converge.

## Runtime Policy Switching

`POST /engine/eviction-policy` changes the policy object used by future admissions. It does not
delete data immediately or change the configured memory limit. The frontend Eviction Lab calls this
endpoint through `frontend/src/api/client.ts`.

## Metrics

[`internal/metrics/inmemory.go`](../../internal/metrics/inmemory.go) uses a mutex around in-memory
counters, gauges, latencies, and a bounded event list. The engine emits:

- `cmd.total` and per-command latency observations;
- replicated-command latency and `replication.applied`;
- `eviction.attempts`;
- `eviction.keys_evicted` and the compatibility count used by the dashboard;
- `eviction.bytes_freed`;
- `eviction.rejected_writes`.

The TCP server also observes transport-level command latency and connection rejection. API handlers
read a snapshot rather than exposing mutable maps.

When automatic recovery is enabled, the controller publishes one-hot gauges named
`controller.state.<state>` for the public recovery states such as `healthy`, `repairing`, and
`potential_data_loss`. `/metrics/summary`, `/cluster/state`, `/controller/state`, and SSE use the
same state names so the frontend and terminal demo do not translate between private controller
terms and public status terms.

## SSE Observability

[`internal/api/websocket.go`](../../internal/api/websocket.go) periodically emits JSON over
server-sent events. Events contain timestamp, memory state, active policy, rejected writes, and
counters. The frontend computes command throughput from successive `cmd.total` samples rather than
requiring another backend counter.

## Demonstrating The Guarantee

Start [`configs/standalone-low-memory.yaml`](../../configs/standalone-low-memory.yaml), then run
[`scripts/demo-low-memory.ps1`](../../scripts/demo-low-memory.ps1). It proves that `noeviction`
preserves existing keys while rejecting growth, then switches to LRU and proves eviction happens
before the new value is visible.

The strongest focused tests are in
[`internal/engine/eviction_integration_test.go`](../../internal/engine/eviction_integration_test.go)
and [`internal/cluster/attach_test.go`](../../internal/cluster/attach_test.go).
