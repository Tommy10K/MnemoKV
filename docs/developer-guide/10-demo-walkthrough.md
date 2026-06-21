# 10. Recommended Full-Project Demonstration

This flow moves from the simplest mental model to the distributed and visual features. A complete
version takes roughly 20-30 minutes; skip the optional deep dives for a shorter presentation.

## 1. Introduce The Purpose

Explain that MnemoKV is an educational in-memory store: RESP-compatible command execution in Go,
observable internals, snapshot persistence, a deliberately small distributed mode, and a React
learning/operations UI. State the limits early: teaching-grade, snapshots rather than WAL, and
manual rather than automatic failover.

## 2. Start Standalone Backend And Frontend

Terminal 1:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

Terminal 2:

```powershell
cd frontend
npm.cmd ci
npm.cmd run dev
```

Open the Vite URL. Show Home, then the Learn index to establish that the UI connects concepts to
the implementation.

## 3. Show RESP Compatibility And Data Types

Use `redis-cli` if installed:

```powershell
redis-cli -p 6380 PING
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
redis-cli -p 6380 INCR visits
redis-cli -p 6380 RPUSH queue ingest index replicate
redis-cli -p 6380 ZADD scores 120 alice 300 bob 200 carol
redis-cli -p 6380 ZRANGE scores 0 -1 WITHSCORES
```

Then execute one command in `/use/console`. Explain that RESP and HTTP create the same command and
use the same executor.

For a repeatable automated tour of strings, TTL, lists, sorted sets, malformed RESP, workloads, and
metrics, run:

```powershell
./scripts/demo-standalone.ps1
```

## 4. Load Presentation Data And Show Observability

```powershell
./scripts/load-demo-dataset.ps1
```

Open `/use/dashboard`. Point out node identity, memory, command count, active policy, SSE status,
and rolling charts. Generate traffic in another terminal:

```powershell
go run ./cmd/workload -addr 127.0.0.1:6380 -profile mixed -concurrency 4 -duration 15s -keyspan 100 -seed 42
```

Watch throughput and memory change. Explain that the browser derives throughput from successive
`cmd.total` SSE samples.

## 5. Connect UI Visuals To Engine Structures

Visit the Strings, Lists, Sorted Sets, and Lock Striping chapters. Use the linked-list, skip-list,
and striping visuals. Relate them to `ListValue`, `ZSetValue` plus `SkipList`, and `Store.stripes`.

The key explanation is that lock striping permits unrelated keys to progress concurrently while
same-key operations remain atomic.

## 6. Demonstrate Hard Memory Admission And Eviction

Stop the default node. Start the low-memory preset:

```powershell
go run ./cmd/node -config configs/standalone-low-memory.yaml
```

In another terminal:

```powershell
./scripts/demo-low-memory.ps1
```

Open `/use/eviction`. Explain that `noeviction` rejects growth without deleting data, while LRU
chooses victims before committing the new write. Emphasize that the limit covers accounted dataset
bytes, not total process RAM.

## 7. Demonstrate Snapshot Persistence

Stop the low-memory node, then run:

```powershell
./scripts/demo-persistence.ps1
```

The script tests JSON and binary. Explain temporary-file plus rename, checksums, retention, restore
before serving, expiration filtering, and the loss window after the latest snapshot.

Optionally open a generated JSON snapshot under `data/` to show the readable logical model.

## 8. Show Configuration Generation

Restart the normal standalone node and frontend if needed. Open `/use`, switch between standalone
and clustered modes, inspect validation, and download YAML. Clarify that this page generates a file;
it does not mutate a running node.

## 9. Demonstrate The Cluster

Stop any node using ports 6381-6383 or 7381-7383, then run:

```powershell
./scripts/demo-cluster.ps1 -KeepRunning
```

The script proves identical slot metadata, any-node routing, synchronous replica acknowledgement,
failure-before-leader-mutation when a replica is down, and CROSSSLOT rejection.

While nodes are running, point the frontend API target to `http://127.0.0.1:7381` and open
`/use/cluster`. Explain:

- fixed FNV-1a slots;
- one leader and one replica per slot;
- local role, term, and sequence fields;
- membership as local health hints only;
- any-node proxy routing.

The script intentionally stops one replica and then cleans up; it does not perform the complete
manual repair. To demonstrate repair interactively, run a topology with a live replacement node,
inspect the real affected slot, and use the API of a node holding current metadata:

```powershell
go run ./cmd/adminctl -port 7382 cluster
go run ./cmd/adminctl -port 7382 cluster-promote 42
go run ./cmd/adminctl -port 7382 cluster-assign-replica 42 node-3
go run ./cmd/adminctl -port 7382 cluster-sync 42 node-3
```

Replace `42` and `node-3` with the real affected slot and live replacement. Explain why promotion,
assignment, and sync are separate and why writes remain unavailable until the replacement is ready.

## 10. Finish With Benchmarks And Scope

Open `/use/benchmarks` and select **Load built-in example**. Compare latency, bytes, and allocations
across command families. If time permits, import fresh output from the engine benchmarks.

Close by restating the implemented guarantees and intentional omissions. This prevents the demo
from accidentally presenting heartbeat hints as consensus, synchronous in-memory replication as
disk durability, or snapshots as zero-loss persistence.

## Pre-Demo Verification

Run this before presenting:

```powershell
go test ./...

cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

Also verify the required ports are free and that PowerShell permits the demo scripts.
