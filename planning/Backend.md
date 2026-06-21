# Backend Status And Maintenance Plan

## Architecture

`cmd/node` loads strict YAML, creates metrics and the engine, restores the latest valid snapshot,
attaches the cluster coordinator when enabled, and starts RESP plus HTTP/SSE listeners.

The engine remains the only command implementation. Standalone servers call it directly. Cluster
servers place `internal/cluster.Coordinator` in front of both RESP and HTTP so both interfaces use
identical key extraction, slot checks, proxying, and failure behavior.

## Storage And Commands

The striped store implements strings, lists, and sorted sets with TTLs. Writes use hard accounted
memory admission. FIFO, LRU, LFU, and random eviction decisions are made before the incoming write;
in cluster mode, the leader sends every selected victim as an explicit replicated `DEL`.

Supported public RESP2 commands are documented in `Run_And_Use.md`. Internal cluster commands are
`REPLICATE`, `CLUSTERMETA`, `CLUSTERAPPLY`, and `CLUSTERSNAPSHOT`; standalone execution does not
special-case them. Cluster mode rejects global flush commands because no cluster-wide transaction
protocol exists.

## Persistence

`internal/snapshot` defines one logical model encoded as JSON or binary. Snapshots contain node and
cluster identity, peer addresses, metadata version, slot ownership/terms/sequences/readiness,
entries, absolute expirations, approximate size, and checksum. The persistence manager writes by
temporary file plus rename, retains the newest configured count, skips corrupt newer candidates,
and restores metadata before data.

Snapshot persistence is not a write-ahead log. Writes after the newest snapshot can be lost.

## Cluster Contract

- Supported size: 2–5 nodes.
- Partitioning: fixed slot count, default 1,024; whole-key FNV-1a hashing.
- Initial ownership: peer IDs sorted, contiguous ranges split evenly, next peer is the replica.
- Routing: any RESP/API node proxies key commands to the current slot leader.
- Multi-key behavior: all keys must share a slot, otherwise `CROSSSLOT`.
- Replication: one synchronous replica; success only after replica apply and leader mutation.
- Ordering: source, slot, term, and next sequence are validated; duplicates are idempotent and gaps
  are rejected.
- Failure: no automatic election. A missing leader or ready replica makes affected operations
  unavailable while unaffected slots continue.
- Recovery: promote assigned replica, assign replacement, transfer a full slot snapshot, then mark
  ready. Metadata versions and terms fence stale returning nodes.
- Membership: all-to-all heartbeat observations are operational hints only.

`internal/cluster.Metadata` is the only ownership representation. The old consistent-hash ring,
async queue, local election, and separate control plane have been removed so routing, fencing,
replication, admin operations, persistence, and status cannot consult conflicting maps.

## HTTP And Admin Surfaces

| Method/path | Purpose |
|---|---|
| `GET /health` | Node health and identity |
| `GET /engine/state` | Memory and eviction state |
| `GET /metrics/summary` | Counters |
| `GET /cluster/state` | Metadata version, peers, membership hints, and every slot state |
| `POST /commands` | Execute through the active standalone/cluster coordinator |
| `POST /admin/snapshot` | Write a snapshot |
| `POST /cluster/promote` | Promote the assigned replica for a slot |
| `POST /cluster/replica` | Assign a replacement replica |
| `POST /cluster/sync` | Full-slot snapshot repair |

`adminctl` exposes the same cluster status and mutation operations.

## Verification Expectations

Run targeted tests while editing, then:

```powershell
go test ./...
go test -race ./...
go vet ./...
./scripts/demo-cluster.ps1
```

Cluster changes are incomplete without multi-process verification of routing, ordered replication,
peer loss, repair, and stale rejoin. Standalone and snapshot suites must remain green because all
modes share the engine and snapshot model.

## Remaining Backend Work

- Authentication, TLS, RESP3, write-ahead logging, quorum consensus, automatic failover, and online
  rebalancing remain explicitly out of scope.
- Add longer partition/chaos runs if the educational cluster evolves beyond local demonstrations.
- Consider replication lag/event metrics without introducing a second source of topology truth.
