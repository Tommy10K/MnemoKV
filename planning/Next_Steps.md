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

## Suggested Further Improvements

- Add component-level tests for parser edge cases and complex visual interactions; the current
  browser suite intentionally concentrates on user-visible integration behavior.
- Add visual-regression snapshots for the main presentation routes and high-contrast mode.
- Generate or share an explicit API schema so backend and frontend response contracts cannot drift.
- Add CI jobs for Go tests, frontend lint/build, and Edge/Chromium end-to-end tests on Windows and
  Linux.
- Expand responsive testing to phone-sized layouts if mobile use becomes a presentation goal.
- Add longer cluster partition and recovery runs plus replication-lag observability.
- Consider an admin-planned rebalance workflow. Automatic failover still requires a quorum-backed
  metadata design and must not be added as a local-election shortcut.
- Consider a write-ahead log only if recovery-point guarantees beyond periodic snapshots become a
  requirement.
- Treat authentication, authorization, TLS, RESP3, consensus, and production hardening as separate
  scope rather than incremental polish.
