# 7. Cluster Routing, Replication, Repair, And Failover

## Supported Cluster, Precisely

MnemoKV supports a static two-to-five-node educational cluster. Every node is both a data node and
a gateway. It uses fixed slots, proxy routing, one synchronous replica per slot, local heartbeat
observations, and manual failover. It does not use Raft or another consensus protocol.

Read [ADR 004](../adr/004-cluster-write-safety.md) and
[ADR 005](../adr/005-failover-semantics.md) before changing this package.

## Authoritative Metadata

[`internal/cluster/metadata.go`](../../internal/cluster/metadata.go) is the only ownership source.
It stores cluster ID, metadata version, sorted peers, and each slot's:

- leader and optional replica;
- term;
- leader sequence and local applied sequence;
- replica readiness.

At initial startup, peer IDs are sorted and the fixed slot count is split into contiguous ranges as
evenly as possible. The next peer in the ring is the replica for each leader range. All nodes with
the same validated peer list compute the same table.

Keys use FNV-1a modulo slot count. This is whole-key hashing; Redis hash tags are not implemented.

## Manager And Coordinator

[`internal/cluster/manager.go`](../../internal/cluster/manager.go) composes metadata, router, RESP
proxy, replicator, membership table, probe loop, and engine attachment.

[`internal/cluster/coordinator.go`](../../internal/cluster/coordinator.go) sits in front of the
engine for both RESP and HTTP:

1. Intercept internal replication, metadata, and shard-snapshot commands.
2. Reject global flushes because there is no cluster-wide transaction.
3. Extract command keys and enforce one slot.
4. Look up the leader in authoritative metadata.
5. Proxy to another node or execute on the local leader.

[`internal/cluster/proxy.go`](../../internal/cluster/proxy.go) maintains one cached RESP connection
per peer. A mutex on each connection keeps request/reply pairs ordered when multiple goroutines use
the same peer.

## Synchronous Write Sequence

Engine attachment in [`attach.go`](../../internal/cluster/attach.go) installs the replicator as a
synchronous write hook. For a leader write:

```text
gateway -> slot leader coordinator
  -> engine admission lock and write plan
  -> prepare next slot sequence
  -> send REPLICATE(source, slot, term, sequence, command) to assigned replica
  -> replica validates source/term/next sequence
  -> replica applies through Engine.ApplyReplicated
  -> replica commits applied sequence and replies OK
  -> leader commits sequence
  -> leader performs local mutation
  -> client receives response
```

If the replica cannot acknowledge, the leader mutation has not happened. This is the central write
safety guarantee. An OK means the value exists in memory on leader and replica, not that it is on
disk or accepted by a quorum.

[`metadata.go`](../../internal/cluster/metadata.go) treats duplicate sequences idempotently, rejects
gaps, rejects stale terms, and verifies the configured source leader. [`replicator.go`](../../internal/cluster/replicator.go)
serializes leader replication to preserve per-slot ordering.

Eviction follows the same stream: the leader replicates explicit victim `DEL` commands before
deleting locally, then replicates the admitted write.

## Membership Is A Hint, Not Authority

[`gossip.go`](../../internal/cluster/gossip.go) performs all-to-all periodic `PING` probes despite
its historical name. [`membership.go`](../../internal/cluster/membership.go) marks peers healthy,
suspect, or unavailable based on failures and elapsed time.

These states appear in `/cluster/state` and the frontend. They never elect a leader or modify slot
ownership. Metadata remains authoritative.

## Manual Failure Recovery

When a leader fails, its slots are unavailable until the assigned replica is promoted. Promotion:

- makes the old replica leader;
- clears the replica assignment;
- increments term and metadata version;
- resets replication sequences.

The operator must then assign a replacement replica. Assignment increments the term again and marks
the replica not ready. Writes remain unavailable until full-slot synchronization succeeds.

[`internal/cluster/repair.go`](../../internal/cluster/repair.go) implements synchronization:

1. The current leader snapshots all engine entries.
2. It filters entries belonging to the requested slot.
3. It sends `CLUSTERSNAPSHOT` to the assigned replacement.
4. The replacement validates source, slot, and term.
5. It rebuilds the local dataset atomically with that slot's entries replaced.
6. The leader marks the replica ready and broadcasts the new metadata version.

The APIs and CLI intentionally keep promotion, assignment, and sync as three separate operations.

## Returning Nodes And Metadata Distribution

Admin changes are broadcast through `CLUSTERAPPLY`. Nodes reject mismatched clusters, peers, slot
counts, older versions, and invalid terms. At startup a node requests `CLUSTERMETA` from peers and
adopts the newest compatible version. A stale returning node therefore cannot automatically reclaim
leadership.

This is not consensus. Concurrent operators can still create coordination problems, and metadata
availability is not quorum-backed. That is why failover is manual and the supported scope is small.

## Best Tests To Read

- [`internal/cluster/cluster_test.go`](../../internal/cluster/cluster_test.go): deterministic slots and metadata.
- [`internal/cluster/attach_test.go`](../../internal/cluster/attach_test.go): synchronous writes and eviction convergence.
- [`test/cluster/cluster_test.go`](../../test/cluster/cluster_test.go): multi-node routing, failure, repair, and stale rejoin.
- [`test/failover/failover_test.go`](../../test/failover/failover_test.go): stale metadata and term fencing.
- [`scripts/demo-cluster.ps1`](../../scripts/demo-cluster.ps1): repeatable process-level demonstration.
