# 1. Project Purpose And System Map

## What MnemoKV Is

MnemoKV is an educational, observable, Redis-compatible in-memory key/value store. It is a diploma
project designed to make database internals visible: RESP2 parsing, typed values, lock striping,
memory accounting, eviction, snapshots, fixed-slot sharding, synchronous replication, manual
failover, workloads, metrics, and a browser learning interface.

It is deliberately not a production Redis replacement. In particular, it does not provide
authentication, TLS, RESP3, a write-ahead log, quorum consensus, automatic failover, online
rebalancing, or production-grade operational hardening.

## The Three User-Facing Pieces

1. The Go node serves RESP2 on TCP and an HTTP/SSE observability API.
2. The Go command-line tools generate workloads and perform administrative operations.
3. The React frontend teaches concepts and operates a running node through HTTP/SSE.

The backend does not depend on the frontend. A normal RESP2 client such as `redis-cli` can use the
database directly.

## Runtime Shape

The composition root is [`cmd/node/main.go`](../../cmd/node/main.go). Its startup sequence is:

```text
YAML config
  -> logging level
  -> in-memory metrics sink
  -> engine and striped store
  -> cluster manager
  -> persistence manager and optional restore
  -> active command executor
  -> RESP server + HTTP/SSE server
  -> background membership probes + periodic snapshots
```

The active executor is important. In standalone mode, RESP and HTTP call the engine directly. In
cluster mode, both call the same `cluster.Coordinator`, which routes to the authoritative slot
leader before the engine runs. This prevents RESP and HTTP from having different cluster behavior.

## Package Ownership

| Location | Owns |
| --- | --- |
| [`cmd/node`](../../cmd/node/) | Process composition, startup, signals, and shutdown |
| [`cmd/workload`](../../cmd/workload/) | Synthetic traffic CLI |
| [`cmd/adminctl`](../../cmd/adminctl/) | Snapshot and cluster administration CLI |
| [`internal/config`](../../internal/config/) | YAML model, defaults, strict parsing, invariants |
| [`internal/resp`](../../internal/resp/) | RESP2 frames, parser, writer, command pooling, key extraction |
| [`internal/server`](../../internal/server/) | TCP listener and per-connection request loop |
| [`internal/engine`](../../internal/engine/) | Commands, values, striped storage, TTL, memory admission |
| [`internal/engine/eviction`](../../internal/engine/eviction/) | Victim-selection policies |
| [`internal/api`](../../internal/api/) | HTTP API, command JSON conversion, SSE, admin handlers |
| [`internal/metrics`](../../internal/metrics/) | Thread-safe counters, gauges, latencies, and events |
| [`internal/snapshot`](../../internal/snapshot/) | Shared logical snapshot and JSON/binary codecs |
| [`internal/persistence`](../../internal/persistence/) | Snapshot scheduling, atomic writes, retention, restore |
| [`internal/cluster`](../../internal/cluster/) | Slots, routing, proxying, replication, membership, repair |
| [`internal/workload`](../../internal/workload/) | Workload profiles, clients, generators, and runner |
| [`frontend`](../../frontend/) | React learning and operations UI |
| [`test`](../../test/) | Black-box and multi-package acceptance tests |

## Supported Features At A Glance

- Strings: `SET`, `GET`, and `INCR`.
- Generic keys: `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `FLUSHDB`, and `FLUSHALL`.
- Lists: `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, and `LLEN`.
- Sorted sets: `ZADD`, `ZRANGE`, `ZCARD`, and `ZSCORE`.
- Hard accounted-memory limits and `noeviction`, FIFO, LRU, LFU, or random admission behavior.
- JSON and binary snapshots with checksums, retention, startup restore, and manual triggers.
- Two-to-five-node fixed-slot clusters with proxy routing and one synchronous replica.
- Manual promotion, replacement-replica assignment, and full-slot synchronization.
- Health, engine, metrics, cluster, command, policy, snapshot, and cluster-admin HTTP endpoints.
- SSE dashboard data, workloads, benchmark import, config generation, and learning chapters.

## First Code Reading

Read these files before diving into a subsystem:

1. [`cmd/node/main.go`](../../cmd/node/main.go) for object construction and lifecycle.
2. [`internal/engine/engine.go`](../../internal/engine/engine.go) for the engine's owned components.
3. [`internal/engine/executor.go`](../../internal/engine/executor.go) for the command list.
4. [`internal/cluster/coordinator.go`](../../internal/cluster/coordinator.go) for clustered execution.
5. [`frontend/src/app/routes.tsx`](../../frontend/src/app/routes.tsx) for the complete UI map.

Do not infer a feature merely from a type or page name. The integration tests and composition root
show whether it is actually wired into the live application.
