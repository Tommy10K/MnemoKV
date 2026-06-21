# MnemoKV Next Steps

Last reviewed against the codebase: June 21, 2026.

## Completed Baseline

MnemoKV now has one shared command engine for standalone and cluster operation, strict RESP and
HTTP boundaries, race-safe reads and writes, hard accounted-memory admission, and deterministic
FIFO/LRU/LFU/random eviction. The public no-eviction policy is `noeviction`.

Snapshot persistence is complete for JSON and binary formats. Snapshots are checksummed, written
atomically, retained by policy, restored at startup, and include cluster metadata where relevant.
Manual snapshot triggering is available through HTTP and `adminctl`.

Standalone mode includes API smoke coverage, low-memory and persistence presets, repeatable command
demos, and deterministic presentation data.

The supported two-to-five-node cluster uses one fixed-slot metadata model for routing, fencing,
persistence, and status. Any node proxies commands to the slot leader; writes synchronously reach
one ready replica; ordering uses slot terms and sequences; and manual promotion, replacement-replica
assignment, and full-slot repair are covered by multi-process tests.

The React frontend now includes all learning and operational routes, persisted API targeting,
runtime validation for HTTP and SSE payloads, distinct offline and malformed-response states,
accessible chart summaries and controls, keyboard focus handling, reduced-motion support, and
responsive layouts. It ships a built-in benchmark sample and a deterministic demo dataset. Edge
end-to-end tests cover navigation, API switching, a live dashboard and command console, invalid API
shapes, offline behavior, benchmark loading, WCAG A/AA checks, and laptop, projector, and tablet
viewports.

The tracked developer guide now provides an ordered implementation walkthrough and a complete demo
flow for contributors who only know the project's general purpose.

## Verification Baseline

For backend changes, run the focused package tests followed by `go test ./...`; use race tests for
concurrent engine, persistence, or cluster paths. Cluster claims additionally require the
multi-process cluster demo or equivalent integration coverage.

For frontend changes, run:

```powershell
cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

The end-to-end runner builds and starts an isolated standalone node and Vite server, then cleans up
both processes. The deterministic presentation dataset can be loaded with
`scripts/load-demo-dataset.ps1` while a standalone node is running.

## Experimental Automatic Cluster Recovery

Automatic recovery should be an optional control-plane feature built beside the current cluster,
not inside its command, routing, or replication paths. The existing fixed-slot metadata, manual
admin operations, and synchronous data replication should remain the stable default.

The least coupled design is a separate `cluster-controller` process or package running a small Raft
group. Raft should replicate only control-plane decisions: metadata versions, slot terms,
promotions, replacement-replica assignments, and repair intents. It should not become the data
command log and should not sit on the normal RESP/HTTP request path.

The controller leader would observe nodes through existing health and cluster-state APIs. After a
failure timeout, it would propose a recovery plan to Raft. Only a majority-committed plan may be
executed; without quorum, ownership must remain unchanged. Execution should use a narrow adapter
over the existing promote, assign-replica, and full-slot-sync operations, making each step
idempotent and resumable after controller restart or leadership change.

Keep the experiment disabled by default and isolate it behind interfaces so current nodes need, at
most, a committed control index or fencing token when accepting metadata updates. Manual recovery
must continue to work unchanged when the controller is absent. Test the controller first with an
in-memory Raft transport and simulated failures, then with separate multi-process tests covering
leader loss, minority partitions, duplicate plans, restart during repair, and stale-node rejoin.
