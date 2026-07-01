# MnemoKV Developer Guide

This guide is the implementation-oriented onboarding path for a developer who understands the
general idea of an in-memory database but does not yet know MnemoKV. Read the chapters in order.
Each chapter first establishes a concept, then points to the code that implements it, and finally
suggests concrete files or tests to inspect.

## Recommended Reading Order

1. [Project purpose and system map](01-project-purpose-and-system-map.md)
2. [Startup, configuration, and lifecycle](02-startup-configuration-and-lifecycle.md)
3. [RESP, HTTP, and command request paths](03-protocol-command-and-api-paths.md)
4. [RESP command reference](12-resp-command-reference.md)
5. [Engine, data structures, and concurrency](04-engine-data-structures-and-concurrency.md)
6. [Memory limits, eviction, and observability](05-memory-eviction-and-observability.md)
7. [Snapshots and recovery](06-snapshots-and-recovery.md)
8. [Cluster routing, replication, repair, and failover](07-cluster-routing-replication-and-failover.md)
9. [Automatic failure recovery control plane](11-automatic-failure-recovery-control-plane.md)
10. [React frontend architecture and features](08-frontend-architecture-and-features.md)
11. [Testing, debugging, and extending the system](09-testing-debugging-and-extending.md)
12. [Recommended full-project demonstration](10-demo-walkthrough.md)

## Sources Of Truth

Use this precedence when two explanations appear to disagree:

1. Current code, tests, and checked-in configs.
2. Accepted decisions under [`docs/adr`](../adr/).
3. The root [`README.md`](../../README.md) for supported run commands.
4. This guide for explanation and navigation.

The ADRs are short on purpose. Read them before changing system modes, command behavior, memory
semantics, cluster write safety, or failover:

- [ADR 001: system modes](../adr/001-system-modes.md)
- [ADR 002: command semantics](../adr/002-command-semantics.md)
- [ADR 003: memory and eviction](../adr/003-memory-and-eviction-semantics.md)
- [ADR 004: cluster write safety](../adr/004-cluster-write-safety.md)
- [ADR 005: failover](../adr/005-failover-semantics.md)
- [ADR 006: automatic recovery control plane](../adr/006-automatic-recovery-control-plane.md)

After completing this guide, a developer should be able to trace a command from a socket or HTTP
request to the in-memory value, explain concurrency and memory guarantees, describe snapshot and
cluster failure behavior without overstating it, work on the React UI, and run a complete demo.
