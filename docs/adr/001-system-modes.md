# ADR 001: System Modes

## Status

Accepted and implemented.

## Decision

MnemoKV supports standalone startup and clustered startup. Clustered startup has two failover
submodes selected statically by `cluster.failoverMode`. Configuration outside this matrix fails
validation.

| Mode | `cluster.enabled` | `shardingEnabled` | `replicationEnabled` | Routing | Failover |
|---|---:|---:|---:|---|---|
| Standalone | false | false | false | local | n/a |
| Cluster | true | true | true | `proxy` | `manual` |
| Cluster | true | true | true | `proxy` | `automatic` |

Cluster mode requires a cluster ID, 2-5 peers, a fixed slot count, a unique RESP and API address for
every peer, and the local node in the peer list. The default slot count is 1,024. User-selectable
async/strong replication is not supported; cluster writes always use the synchronous replica
contract in ADR 004.

Manual failover mode keeps topology changes operator-driven as described in ADR 005. Automatic
recovery mode is opt-in and starts the embedded control-plane-only controller described in ADR 006.
Automatic mode additionally requires every peer entry to use `failoverMode: automatic`, every peer
to have a control address, a configured controller Raft directory, a bootstrap node ID, positive
controller timing/rebalance settings, and a shared non-empty request-signing secret.

Standalone mode rejects leftover cluster flags or peers so a misspelled or partially disabled
cluster cannot silently boot with different behavior.
