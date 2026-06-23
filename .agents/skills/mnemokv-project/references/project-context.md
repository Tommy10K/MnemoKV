# MnemoKV Project Context

Snapshot date: 2026-06-22. Verify volatile details against the repository before relying on them.

## Purpose

MnemoKV is a diploma project and educational platform: an observable, Redis-compatible, in-memory key/value store written in Go with a React frontend. It demonstrates data structures, lock striping, memory accounting and eviction, RESP2, workloads and benchmarks, sharding concepts, replication, gossip, and failover. Clarity and demonstrability matter more than competing with Redis on production performance.

The project is now in a polish phase. Default work should focus on bugs, incomplete wiring, race safety, operational reliability, UI quality, documentation accuracy, and a reproducible diploma demo rather than following old implementation phases mechanically.

## Current Truth And Historical Plans

- Current code, tests, configs, and accepted ADRs are stronger evidence than planning checkboxes.
- `planning/` is local and gitignored. It contains backend and frontend status documents, next-step
  notes, and a combined run-and-use guide.
- The tracked root `README.md` is the current standalone run, demo, persistence, API, and frontend guide.
- `docs/developer-guide/README.md` is the ordered implementation-onboarding path and demo guide.
- Some source comments still say "baseline" or "placeholder" after later features were added. Verify behavior instead of trusting those comments.

## Runtime Architecture

Main backend startup in `cmd/node/main.go`:

`YAML config -> metrics sink -> engine -> cluster manager + snapshot manager -> RESP server + HTTP API`

Primary request paths:

`standalone RESP/HTTP -> engine executor -> striped store`

`cluster RESP/HTTP -> coordinator -> fixed-slot leader -> synchronous replica -> engine executor`

The frontend is a React/Vite local web application that talks to the HTTP API and SSE stream. The backend remains usable independently through `redis-cli` or another RESP2 client.

## Backend Map

- `cmd/node`: node composition and lifecycle.
- `cmd/workload`: synthetic RESP workload generator.
- `cmd/adminctl`: HTTP observability, snapshot, and manual cluster-administration CLI.
- `internal/config`: YAML model, defaults, loading, and startup validation.
- `internal/resp`: RESP2 frames, parser/writer, command pooling, and key extraction.
- `internal/server`: TCP listener and per-connection command loop.
- `internal/engine`: command dispatch, striped store, values, TTL, memory tracking, and write hooks.
- `internal/engine/eviction`: noeviction, FIFO, LRU, LFU, and random policies behind a small policy interface.
- `internal/metrics`: in-memory counters, gauges, latency observations, and events.
- `internal/logging`: level-aware wrapper around the standard logger.
- `internal/api`: health, engine, metrics, cluster, SSE, command, and eviction-policy endpoints.
- `internal/workload`: profiles, clients, generator, and runner.
- `internal/cluster`: authoritative fixed-slot metadata, RESP proxy routing, ordered synchronous
  replication, membership hints, manual failover, stale-node fencing, and full-slot repair.
- `internal/controller`: optional embedded Raft control plane for automatic observation, recovery,
  repair, and mandatory rebalancing; it remains outside the RESP/HTTP command path.
- `internal/controlplane`: shared HMAC request authentication, persistent control-index fencing,
  and recovery-status contracts used by the controller and node API.
- `internal/snapshot`: versioned logical snapshot model plus JSON and binary codecs and checksum validation.
- `internal/persistence`: atomic snapshot writing, retention, periodic/manual triggers, and startup restore.
- `test/integration`, `test/cluster`, `test/failover`: cross-package scenarios.
- `docs/adr`: accepted semantic decisions. Read these before changing system modes, command behavior, eviction semantics, write safety, or failover.

The store uses lock striping. Entries carry a type tag, value, expiration, approximate size,
recency, frequency, and creation metadata. Reads receive an entry snapshot so metadata and string
value-pointer updates remain race-safe after the stripe lock is released. Value-accessing reads
update access metadata; administrative reads such as `EXISTS`, `TTL`, `LLEN`, and `ZCARD` do not.
Lists use a doubly linked list. Sorted sets combine a skip list with a member index.

## Supported Interfaces

RESP commands currently dispatched by the engine:

- Connection/compatibility: `PING`, `ECHO`, `QUIT`, `COMMAND`, `CLIENT`, RESP2-only `HELLO` rejection.
- Keys: `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `FLUSHDB`, `FLUSHALL`.
- Strings: `SET` with `EX`, `PX`, `NX`, and `XX`; `GET`; `INCR`.
- Lists: `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LLEN`.
- Sorted sets: `ZADD`, `ZRANGE`, `ZCARD`, `ZSCORE`.
- Internal cluster protocol: `REPLICATE`, `CLUSTERMETA`, `CLUSTERAPPLY`, and `CLUSTERSNAPSHOT`.
- `FLUSHDB` and `FLUSHALL` are standalone-only.

HTTP API routes:

- `GET /health`
- `GET /engine/state`
- `GET /metrics/summary`
- `GET /cluster/state`
- `GET /controller/state` in automatic mode (Raft role/leader/term, committed view, plan progress,
  unavailable slots, and last rebalance)
- `GET /events` for SSE snapshots
- `POST /commands`
- `POST /engine/eviction-policy`
- `POST /admin/snapshot`
- `POST /cluster/promote`
- `POST /cluster/replica`
- `POST /cluster/sync`

With static startup `cluster.failoverMode: automatic`, topology mutations require an authenticated
non-decreasing controller index and manual mutation calls are rejected. `/cluster/state` and SSE
include recovery status, affected slots, and potential-data-loss detail. Manual mode remains the
default and preserves the existing unsigned administration behavior.

Automatic mode is valid only for five consistently configured automatic peers with unique control
addresses, a common bootstrap node, separate non-empty `controller.raftDir` paths, positive
observation/failure/rebalance settings, and one shared non-empty `controlPlane.requestSigningSecret`.
Three Raft voters are required for ownership progress. A restarted automatic node keeps its node ID
and Raft directory but is data-fenced until its engine and application snapshots are cleared, current
metadata is installed, and admission is committed.

The API server enables browser access; keep CORS and method/error behavior in mind when changing it.
Handlers enforce their documented methods, cap JSON request bodies at 1 MiB, and reject trailing
JSON after the first request object.

## Frontend Map

The frontend stack is React 19, TypeScript, Vite, React Router, Zustand, Recharts, React Flow
(`@xyflow/react`), and Tailwind/PostCSS.

Main routes:

- `/`: project overview.
- `/learn` and twelve chapter routes: educational content and interactive visuals.
- `/use`: configuration/YAML guidance.
- `/use/dashboard`: live health, memory, metrics, and SSE data.
- `/use/console`: browser command console using `POST /commands`.
- `/use/workloads`: workload CLI command builder.
- `/use/cluster`: topology, authoritative slots, metadata-version history, and membership hints.
- `/use/benchmarks`: benchmark JSON import and charts.
- `/use/eviction`: runtime eviction-policy lab.

API calls belong in `frontend/src/api/`, global API base URL state in
`frontend/src/store/appStore.ts`, and event lifecycle in hooks. The API target is persisted in the
browser, defaults to `http://127.0.0.1:7380`, and can be initialized with `VITE_API_BASE_URL`.
HTTP and SSE payloads are runtime-validated in `frontend/src/api/validate.ts`; malformed contracts
remain distinguishable from an offline backend. Edge end-to-end coverage verifies navigation, API
switching, live dashboard and console behavior, malformed/offline states, built-in benchmarks,
responsive viewports, keyboard focus, reduced motion, and WCAG A/AA checks.

## Configuration And Running

Standalone defaults:

- RESP2: `127.0.0.1:6380`
- HTTP API: `127.0.0.1:7380`
- Frontend Vite URL: normally `http://localhost:5173`

PowerShell development flow:

```powershell
go run ./cmd/node -config configs/standalone.yaml

cd frontend
npm.cmd install
npm.cmd run dev
```

Build backend binaries directly on Windows when GNU Make or shell utilities are unavailable:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/mnemokv-node.exe ./cmd/node
go build -o bin/mnemokv-workload.exe ./cmd/workload
go build -o bin/mnemokv-adminctl.exe ./cmd/adminctl
```

Manual cluster configs use RESP ports `6381`-`6383` and API ports `7381`-`7383`; automatic presets
extend both ranges through `6385`/`7385` and use controller ports `7481`-`7485`. PowerShell demo scripts
cover standalone commands/workloads, low-memory admission, JSON/binary restart restore, and a live
three-process cluster check (`scripts/demo-cluster.ps1`). The five-process automatic acceptance
script is `scripts/demo-automatic-recovery.ps1`; `-ReturnNode` adds fresh admission and 4-to-5 rebalance.

Standalone demo presets include `standalone-low-memory.yaml`, `standalone-persistence-json.yaml`,
and `standalone-persistence-binary.yaml`; they reuse the default standalone ports and should be run
one at a time.

Optional snapshot persistence is configured under `persistence` with `enabled`, `dataDir`,
`snapshotIntervalSec`, `maxSnapshots`, `loadOnStart`, and `format` (`json` or `binary`). Snapshots
are atomic, checksummed, retained by newest valid creation time, and restore strings, lists, sorted
sets, absolute expirations, peer identity/addresses, and cluster slot metadata. This is snapshot
persistence, not a write-ahead log.

## Validation

Backend baseline:

```powershell
go test ./...
go test -race ./...
go test -race ./internal/controller/... ./internal/cluster/... ./internal/api/...
```

Frontend baseline:

```powershell
cd frontend
npm.cmd install
npm.cmd run build
npm.cmd run lint
npm.cmd run test:e2e
```

Use `redis-cli`, HTTP requests, workloads, and browser verification for end-to-end checks. Unit tests alone are not enough for cluster or frontend claims.

As of this snapshot, the focused engine/RESP/server/integration tests and focused race suite pass.
The full Go suite, full race suite, and vet should be rerun after backend changes. Frontend
`npm ci`, lint, build, and the isolated Edge end-to-end suite pass from committed metadata.

Shared engine semantics now include atomic `SET NX`/`XX`, canonical Redis-style integer parsing,
overflow-safe relative expirations, `NaN` rejection for sorted sets, strict `ZRANGE` options,
utility-command arity checks, bounded RESP requests, safe handling of empty commands, hard
accounted memory admission, and the public `noeviction` policy.

Common operational behavior now includes `cmd.total` as the canonical command counter, enforced
`network.maxConnections` in the RESP accept path, `observability.logLevel` for node/server/API
logs, and benchmark exports with `nsPerOp`, `bytesPerOp`, and `allocsPerOp`.

## Important Verification Hotspots

1. One `cluster.Metadata` model must remain authoritative for routing, fencing, replication, admin
   changes, repair, status, and persistence.
2. Cluster writes must fail before leader mutation whenever the assigned replica cannot acknowledge.
3. Automatic ownership changes require a three-of-five committed plan and authenticated monotonic
   fencing; Raft remains control-plane-only.
4. Returning automatic nodes are always fresh data members; never use old engine/snapshot data to
   resolve `potential_data_loss`.
5. Promotion, replica assignment, and full-slot synchronization remain separate data-node
   operations; automatic mode may invoke them only from a committed, fenced controller plan.
6. Run standalone and both snapshot codecs after cluster changes because all modes share the engine
   and snapshot model.

## Engineering Expectations

- Keep parsing, routing, execution, metrics, and transport responsibilities separate.
- Keep Go code direct and idiomatic; avoid clean-architecture layers and speculative interfaces.
- Preserve thread safety and run race tests for concurrent paths.
- Keep React components shallow, typed, and resilient when the backend is unavailable.
- Prefer focused fixes and tests over broad rewrites.
- Explain meaningful design/behavior mismatches rather than silently guessing.
