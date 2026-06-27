# ADR 006: Automatic Recovery Control Plane

## Status

Accepted and implemented.

## Context

ADR 005 deliberately made failover manual because membership observations alone cannot safely
change ownership. MnemoKV now needs an opt-in automatic demonstration without moving ordinary data
commands into consensus or weakening the existing synchronous-copy guarantee.

## Decision

Automatic recovery is an embedded, control-plane-only Raft group in `internal/controller`. It is
selected statically with `cluster.failoverMode: automatic`; manual mode remains the default and ADR
005 remains valid for that mode. Automatic mode requires at least three consistently configured
peers, at least three available Raft voters to commit ownership progress, separate per-node
`controller.raftDir` storage, and a shared non-empty request-signing secret. The checked-in
acceptance/demo topology uses five peers so one data-node failure still leaves a controller
majority and enough eligible data nodes for repair plus rebalancing.

Raft contains observations, deterministic recovery/rebalance plans, step progress, unavailable-slot
records, and returning-node admission decisions. It never contains values and never sits on the RESP
or HTTP command path. The controller leader executes committed decisions through the existing
promote, replica-assignment, full-slot-sync, and returning-node admission operations. Every mutation
carries an authenticated, persisted control index; automatic nodes reject unsigned manual topology
mutations and stale or conflicting indexes.

Recovery always restores a ready replica and then rebalances eligible nodes. With no controller
majority, ownership freezes. The supported data guarantee is one node failure at a time with repair
in between. During the degraded window, writes whose slot lacks a ready replica are rejected. A
second destructive failure before repair can remove the last reachable copy; the controller reports
`potential_data_loss` and never creates an empty replacement.

A returning automatic node preserves its configured node ID and Raft directory, but starts behind a
data admission gate. Its engine is cleared and application snapshots are removed without inspection;
current metadata is installed and validated before Raft commits admission. Old returning-node data is
never merged or used to recover an unavailable slot. Normal full-slot synchronization and rebalance
then restore its share of ownership.

## Consequences

- Manual and automatic operation are mutually exclusive for topology mutation.
- Raft availability controls metadata progress, not data-command throughput for unaffected slots.
- Application snapshots remain format-compatible and separate from Raft logs, snapshots, votes,
  fencing indexes, and the returning-node marker.
- Operators can inspect recovery through `/cluster/state`, SSE, metrics/logs, and
  `GET /controller/state`.
- Returning-node preparation and admission use controller-authenticated
  `/cluster/returning/prepare` and `/cluster/returning/admit`; they are not manual operator APIs.
