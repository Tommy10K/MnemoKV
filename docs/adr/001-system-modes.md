# ADR 001: System Modes

## Status

Accepted and implemented.

## Decision

MnemoKV supports two startup modes. Configuration outside this matrix fails validation.

| Mode | `cluster.enabled` | `shardingEnabled` | `replicationEnabled` | Routing | Failover |
|---|---:|---:|---:|---|---|
| Standalone | false | false | false | local | n/a |
| Cluster | true | true | true | `proxy` | `manual` |

Cluster mode requires a cluster ID, 2–5 peers, a fixed slot count, a unique RESP and API address for
every peer, and the local node in the peer list. The default slot count is 1,024. User-selectable
async/strong replication and automatic failover are not supported.

Standalone mode rejects leftover cluster flags or peers so a misspelled or partially disabled
cluster cannot silently boot with different behavior.
