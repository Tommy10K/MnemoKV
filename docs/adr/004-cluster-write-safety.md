# ADR 004: Cluster Write Safety

## Status

Accepted and implemented.

## Decision

Cluster writes use one fixed synchronous contract. Each slot has one leader and one assigned
replica. The leader validates admission, sends an ordered replication record to that replica, and
mutates locally only after the replica acknowledges application. If either owner is unavailable,
the write is rejected.

Records carry source leader ID, slot, term, sequence, and command payload. A follower accepts only
its current leader and term, applies the next sequence, treats duplicates idempotently, and rejects
gaps or stale records. Gaps are repaired by a full shard snapshot.

An OK response means the mutation exists in memory on the leader and replica. It does not imply
disk durability or quorum consensus. Durability remains bounded by snapshot frequency.
