# 9. Testing, Debugging, And Extending

## Test Layers

MnemoKV uses several layers because a distributed or browser feature is not proven by a unit test
alone.

### Package Tests

Tests next to implementation files cover parsing, command semantics, data structures, memory
admission, codecs, config validation, metrics, workloads, and cluster state transitions.

Run a focused package first:

```powershell
go test ./internal/engine/...
go test ./internal/cluster/...
go test ./internal/persistence/...
```

### Cross-Package And Black-Box Tests

- [`test/integration`](../../test/integration/) starts a real RESP server and exercises clients.
- [`test/api`](../../test/api/) checks HTTP methods, body limits, JSON strictness, and availability.
- [`test/cluster`](../../test/cluster/) runs multiple managers and RESP servers through routing,
  replica loss, promotion, repair, and stale rejoin.
- [`internal/controller`](../../internal/controller/) includes the in-process five-node automatic
  recovery harness with real cluster managers and in-memory Raft voters.
- [`test/failover`](../../test/failover/) focuses on stale version and term rejection.

### Race Tests

Run `go test -race ./...` after changes involving maps, values, expiry, admission, eviction, metrics,
connections, replication, membership, or persistence. Race tests are especially important because
ordinary test success does not prove lock correctness.

For automatic recovery work, also run the focused concurrent control-plane path:

```powershell
go test -race ./internal/controller/... ./internal/cluster/... ./internal/api/...
```

### Frontend Tests

```powershell
cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

The E2E runner builds an isolated Go node, starts Vite, runs Edge, and cleans up both processes. It
covers navigation, API target switching, live and offline states, real commands, malformed API
payloads, the built-in benchmark, three viewport sizes, keyboard focus, reduced motion, and WCAG
A/AA Axe rules.

## Benchmarks And Workloads

Go microbenchmarks live in [`internal/engine/benchmark_test.go`](../../internal/engine/benchmark_test.go).
They separate hot-key contention from distributed-key parallel access.

The workload CLI is composition around [`internal/workload`](../../internal/workload/):

- `profiles.go` defines weighted command mixes;
- `generator.go` creates deterministic keys, values, counters, and scores;
- `client.go` is a minimal RESP client;
- `runner.go` starts concurrent workers and aggregates operations/errors.

Use a fixed seed for reproducible demonstrations:

```powershell
go run ./cmd/workload -addr 127.0.0.1:6380 -profile mixed -concurrency 4 -duration 10s -keyspan 100 -seed 42
```

## Debugging A Command

1. Reproduce through the smallest public surface, usually RESP.
2. Confirm parsing in `internal/resp` and command normalization.
3. Check cluster key extraction and routing if clustered.
4. Check admission planning for writes.
5. Inspect the concrete handler and store/container operation.
6. Verify the returned `resp.Frame` and transport serialization.
7. Add the narrowest regression test, then broader tests for shared behavior.

## Debugging Cluster Behavior

Always record key, computed slot, metadata version, leader, replica, term, readiness, leader
sequence, and replica applied sequence. Membership state alone is not ownership evidence.

For a rejected write, determine whether the failure is routing, missing leader, unavailable/not
ready replica, stale term, sequence gap, or memory admission. Do not add retries that hide a
metadata or ordering violation.

For automatic recovery, distinguish data-node health from controller quorum. A slot owner can be
unreachable while the controller still has a Raft majority and can repair ownership; conversely, a
controller minority must not mutate ownership even if some data nodes are healthy.

## Adding A Public Command

Use this checklist:

1. Define semantics and compatibility behavior; update ADR 002 if the decision is durable.
2. Add dispatch and a focused handler in `internal/engine`.
3. Implement an atomic store/container operation with clear locking.
4. Classify it as read or write in `commands.go`.
5. Add memory planning if it can grow the dataset.
6. Decide whether it updates access metadata.
7. Add key extraction for cluster routing and same-slot validation.
8. Ensure replicated application bypasses leader-only admission safely.
9. Ensure snapshot encoding already covers its value changes or extend the model deliberately.
10. Add command, concurrency, RESP, HTTP, cluster, and documentation coverage as applicable.

## Adding An API Or Frontend Feature

For HTTP, register a route, enforce method and strict body decoding, keep command execution behind
the active executor, and add black-box coverage. For frontend data, define a TypeScript shape and a
runtime parser before using it in hooks or components.

For a page, add a lazy route, navigation entry, clear live/offline/malformed states, keyboard focus,
responsive behavior, and an E2E scenario if it is presentation-critical.

## Documentation Maintenance

- Update the root README when startup commands or public features change.
- Update the relevant ADR when an architectural decision changes.
- Update this guide when package ownership, request paths, invariants, or demo order changes.
- Keep examples executable. Prefer linking to source files over copying large implementation blocks.
- Never describe local membership observations as consensus or snapshots as a WAL.
- Never describe automatic recovery as multi-failure data durability; the v1 guarantee is one
  failure at a time with repair in between.
