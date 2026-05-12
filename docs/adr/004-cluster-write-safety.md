# ADR 004: Cluster Write Safety

## Status

Accepted as forward direction. Not exercised by the baseline milestone.

## Decision

Two write-safety modes will be supported when clustering is enabled:

- **async** — leader executes locally, acknowledges the client, then enqueues replication. A client-acknowledged write may be lost if the leader fails before the record is shipped.
- **strong** — leader waits for follower acknowledgements according to the configured quorum (or for control-plane commit) before acknowledging the client.

In both modes, follower apply paths must validate the source term/epoch and reject writes from a stale leader.

The baseline milestone exposes `cluster.writeSafetyMode` in configuration but does not exercise either path because clustering is not active yet.
