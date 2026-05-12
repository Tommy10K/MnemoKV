# ADR 005: Failover Semantics

## Status

Accepted as forward direction. Not exercised by the baseline milestone.

## Decision

- Leadership is tracked by an authoritative control plane keyed by slot or shard with a monotonically increasing term.
- Gossip provides health hints but is not the source of truth for leader changes.
- During an election: writes for the affected slot pause and return `CLUSTERDOWN leader unavailable`; reads may continue against the last-known leader if the configured read mode allows stale reads, otherwise they pause as well.
- After election, the previous leader is fenced by term comparison: any write attempt at an older term is rejected.
- Recovered nodes rejoin as followers and must catch up via the replication queue before serving reads in strong mode.
