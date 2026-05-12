# ADR 001: System Modes

## Status

Accepted (baseline milestone).

## Context

The system supports multiple feature flags that combine into distinct operating modes. Without an explicit, exhaustive matrix, configuration combinations can quietly produce undefined behavior (e.g. enabling automatic failover without replication).

## Decision

Only the following startup mode combinations are supported. Any other combination must fail validation at startup.

| Mode | `cluster.enabled` | `cluster.shardingEnabled` | `cluster.replicationEnabled` | `cluster.autoFailover` | `cluster.writeSafetyMode` |
|------|---|---|---|---|---|
| standalone | false | false | false | false | n/a |
| clustered-sharded | true | true | false | false | async |
| clustered-replicated | true | true | true | false | async \| strong |
| clustered-failover | true | true | true | true | strong |

Rules:

1. `autoFailover=true` requires `replicationEnabled=true`.
2. `replicationEnabled=true` requires `shardingEnabled=true`.
3. `writeSafetyMode=strong` requires `replicationEnabled=true`.
4. `cluster.enabled=false` forces every other cluster flag to false.
5. The baseline milestone (this commit) ships only the standalone mode end-to-end. Cluster fields are accepted by the config layer but their distributed behaviors are implemented in later phases.
