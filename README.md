# MnemoKV

MnemoKV is an educational, observable in-memory key/value store written in Go. It supports a
standalone RESP2 server and a small fixed-slot cluster with a React dashboard and learning UI.

## What works

- RESP2 over TCP, compatible with normal `redis-cli` workflows.
- Strings: `SET` (`EX`, `PX`, `NX`, `XX`), `GET`, and `INCR`.
- Keys: `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `FLUSHDB`, and `FLUSHALL`.
- Lists: `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, and `LLEN`.
- Sorted sets: `ZADD`, `ZRANGE`, `ZCARD`, and `ZSCORE`.
- Hard accounted-memory limits with `noeviction`, FIFO, LRU, LFU, and random policies.
- Health, engine, metrics, cluster-state, command, event-stream, eviction-policy, and snapshot APIs.
- JSON and binary snapshots with checksums, atomic replacement, retention, and startup restore.
- Fixed-slot proxy routing through any node, with same-slot multi-key enforcement.
- Ordered synchronous replication to one replica and explicit manual failover/repair.
- Synthetic string, list, sorted-set, and mixed workload profiles.
- Runtime-validated React UI with accessible charts, responsive layouts, and Edge end-to-end tests.

MnemoKV is teaching-grade software, not a production Redis replacement. Snapshot persistence is
not a write-ahead log: writes after the latest snapshot can be lost.

## Developer documentation

New contributors should follow the ordered
[`docs/developer-guide`](docs/developer-guide/README.md) chapters. They trace startup, RESP and HTTP
requests, engine data structures, concurrency, memory admission, snapshots, cluster behavior,
frontend architecture, testing, extension workflows, and a complete demonstration.

## Requirements

- Go 1.22 or newer.
- Node.js and npm for the frontend.
- PowerShell 7 is recommended on Windows for the bundled demo scripts.
- `redis-cli` is optional; the demos use MnemoKV's HTTP API and raw RESP directly.

## Run standalone

From the repository root:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

The default listeners are:

- RESP2: `127.0.0.1:6380`
- HTTP API: `http://127.0.0.1:7380`

Try the RESP interface:

```powershell
redis-cli -p 6380 PING
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
```

Or use the HTTP command API:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/commands `
  -ContentType application/json `
  -Body '{"args":["SET","greeting","hello"]}'
```

## Run the frontend

Keep the backend running, then start Vite in another terminal:

```powershell
Set-Location frontend
npm.cmd ci
npm.cmd run dev
```

Open the URL printed by Vite, normally `http://localhost:5173`. The frontend defaults to
`http://127.0.0.1:7380`; change the API target in the UI or set `VITE_API_BASE_URL` before starting
Vite.

Useful routes include `/use/dashboard`, `/use/console`, `/use/workloads`, `/use/eviction`, and the
learning chapters under `/learn`.

The UI validates HTTP and SSE response shapes, distinguishes malformed data from an offline node,
and includes a built-in benchmark example.

## Repeatable standalone demos

With `configs/standalone.yaml` running, exercise strings, TTLs, lists, sorted sets, malformed RESP,
a deterministic mixed workload, and metrics:

```powershell
./scripts/demo-standalone.ps1
```

Load and verify deterministic presentation data for the dashboard and console:

```powershell
./scripts/load-demo-dataset.ps1
```

The source dataset is `examples/demo-dataset.json`.

For a 512-byte dataset limit, start the low-memory preset and run the policy demo in another
terminal. It first proves `noeviction` rejection preserves existing keys, then switches to LRU and
proves eviction happens before the new write is admitted.

```powershell
go run ./cmd/node -config configs/standalone-low-memory.yaml
./scripts/demo-low-memory.ps1
```

The persistence demo builds a node, creates data, writes a manual snapshot, terminates the node,
restarts it, and verifies the restored dataset in both formats:

```powershell
./scripts/demo-persistence.ps1                 # JSON and binary
./scripts/demo-persistence.ps1 -Format json    # one format only
```

The corresponding presets are:

- `configs/standalone-persistence-json.yaml`
- `configs/standalone-persistence-binary.yaml`

They use dedicated directories under `data/`, a one-hour automatic interval, three-snapshot
retention, and startup restore. Trigger a snapshot on any persistence-enabled node with:

```powershell
go run ./cmd/adminctl -host 127.0.0.1 -port 7380 snapshot
```

## Run the cluster

The checked-in three-node cluster listens on RESP ports `6381`–`6383` and API ports `7381`–`7383`.
On Windows, the repeatable demo builds and starts isolated nodes, verifies identical metadata,
any-node routing, strict replica acknowledgement, and cross-slot rejection, then cleans up:

```powershell
./scripts/demo-cluster.ps1
```

Manual operations use the API of a node with the current metadata. For example:

```powershell
./bin/mnemokv-adminctl.exe -port 7382 cluster
./bin/mnemokv-adminctl.exe -port 7382 cluster-promote 42
./bin/mnemokv-adminctl.exe -port 7382 cluster-assign-replica 42 node-3
./bin/mnemokv-adminctl.exe -port 7382 cluster-sync 42 node-3
```

Promotion, replica assignment, and full-slot synchronization are separate by design. Writes remain
unavailable for an affected slot until its replacement replica is synchronized.

## Build command-line tools

PowerShell:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/mnemokv-node.exe ./cmd/node
go build -o bin/mnemokv-workload.exe ./cmd/workload
go build -o bin/mnemokv-adminctl.exe ./cmd/adminctl
```

GNU Make environments can use `make build`, `make test`, `make race`, and `make vet`.

## HTTP API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Node health and identity |
| `GET` | `/engine/state` | Memory and eviction state |
| `GET` | `/metrics/summary` | Command and eviction counters |
| `GET` | `/cluster/state` | Authoritative metadata, slot roles, terms, sequences, and membership hints |
| `GET` | `/events` | Server-sent observability snapshots |
| `POST` | `/commands` | Execute one command from JSON arguments |
| `POST` | `/engine/eviction-policy` | Change the active eviction policy |
| `POST` | `/admin/snapshot` | Write a manual snapshot |
| `POST` | `/cluster/promote` | Promote an assigned slot replica |
| `POST` | `/cluster/replica` | Assign a replacement slot replica |
| `POST` | `/cluster/sync` | Transfer a full slot snapshot to its assigned replica |

JSON request bodies are limited to 1 MiB, reject unknown/trailing values, and return method errors
with an `Allow` header. The black-box API smoke test also verifies clean client failure after the
backend becomes unavailable.

## Test and verify

```powershell
go test ./...
go test -race ./...
go vet ./...

Set-Location frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

Focused backend coverage lives in `test/integration` for RESP and `test/api` for HTTP behavior. The
frontend Edge suite starts isolated backend/frontend processes and covers live, offline, malformed,
responsive, keyboard, reduced-motion, and WCAG A/AA flows.

## Repository layout

```text
cmd/                  node, workload, and adminctl binaries
configs/              standalone, persistence, and cluster YAML files
docs/developer-guide/ ordered implementation onboarding and demo walkthrough
frontend/             React/Vite learning and observability UI
internal/api/         HTTP API and SSE
internal/cluster/     fixed-slot metadata, routing, replication, membership, failover, and repair
internal/config/      YAML model, defaults, and validation
internal/engine/      striped store, commands, memory accounting, and eviction
internal/persistence/ snapshot lifecycle, retention, and restore
internal/resp/        RESP2 parser and writer
internal/server/      TCP listener and connection loop
internal/snapshot/    shared snapshot model and JSON/binary codecs
internal/workload/    synthetic workload generator
scripts/              demos and operational helpers
test/                 black-box integration scenarios
```

Cluster mode deliberately omits automatic failover, consensus, online rebalancing, and a
write-ahead log. It is designed and tested for two to five local educational nodes.
