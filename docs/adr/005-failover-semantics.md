# ADR 005: Failover Semantics

## Status

Accepted and implemented.

## Decision

Failover is manual. Membership heartbeats provide health hints but never elect a leader.

For an unavailable leader, an operator promotes the assigned replica, which advances the slot
term and removes the old replica assignment. The operator then assigns a replacement replica and
performs a full-slot snapshot synchronization. Writes remain unavailable until that replica is
ready. Metadata changes advance a cluster-wide version and are broadcast to peers.

Returning nodes load persisted metadata and fetch newer metadata from healthy peers before serving
cluster traffic. Newer metadata versions and slot terms fence stale leaders. There is no automatic
election, quorum metadata plane, automatic rebalancing, or stale-read mode.
