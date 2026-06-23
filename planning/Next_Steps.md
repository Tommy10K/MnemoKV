# MnemoKV Next Steps

Last reviewed against the codebase: June 22, 2026.

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

## Implemented Automatic Cluster Recovery

The optional five-node automatic mode is implemented beside the data path in `internal/controller`.
Its embedded Raft group commits observations, recovery/rebalance plans, progress, unavailable-slot
records, and fresh-node admission. It reuses the existing promote, replica-assignment, and full-slot
sync operations through authenticated, persistently fenced controller requests. Manual mode remains
the default and preserves unsigned operator-driven repair.

The honest guarantee is **one node failure at a time with full repair in between**. There is a
degraded window after failure detection: affected writes are rejected until a ready replica exists,
while unaffected slots continue. If a second destructive failure removes both copies before repair,
the slot becomes `potential_data_loss`; it is not recreated empty and returning-node data is not
trusted as recovery input.

Five-manager/in-memory-Raft scenarios cover promotion, repair, rebalance, partitions, leadership
changes, duplicate execution, sequential and overlapping failures, writes during recovery, and
fresh returning-node admission. `scripts/demo-automatic-recovery.ps1` is the real five-process
acceptance path; use `-ReturnNode` to include the 4-to-5 scale-back.
