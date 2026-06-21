# 6. Snapshots And Recovery

## Durability Model

MnemoKV uses periodic full snapshots, not a write-ahead log. A successful command is durable in
memory only. A crash can lose every write after the newest snapshot, including an acknowledged
cluster write. This limitation must remain explicit in UI and documentation.

## Shared Logical Model

[`internal/snapshot/model.go`](../../internal/snapshot/model.go) defines one versioned `Model` used
by both formats. It contains node identity, optional cluster identity and metadata, creation time,
entries, and a SHA-256 checksum.

Each entry stores its key, value type, type-specific bytes, approximate size, and absolute
expiration timestamp. Cluster snapshots additionally contain peers and every slot's role, leader,
replica, term, sequences, and readiness.

`Model.Seal` canonicalizes and checksums the model. `Model.Verify` validates metadata and compares
the checksum. The checksum detects accidental or deliberate file changes; it is not authentication.

## Codecs

[`internal/snapshot/codec.go`](../../internal/snapshot/codec.go) implements:

- JSON: human-readable model encoding.
- Binary: a fixed header, length, and Go `gob` payload.

Both decode into the same model and run the same verification. The binary decoder rejects wrong
headers, length mismatches, and trailing data.

## Engine Value Encoding

[`internal/engine/snapshot.go`](../../internal/engine/snapshot.go) bridges engine values and the
logical model.

- Strings copy their raw bytes.
- Lists use count plus length-prefixed element bytes in logical order.
- Sorted sets use count plus float64 score bits and length-prefixed members in score order.

Snapshot capture takes `admissionMu`, then read-locks every stripe in stable order. The result is a
key-sorted, unexpired dataset that does not update LRU/LFU metadata.

Restore decodes and validates all entries before replacing the dataset. It rejects duplicate keys,
bad type encodings, incorrect size accounting, and snapshots larger than the current memory limit.
Expired entries are skipped. Whole-store replacement locks every stripe before swapping maps and
the memory counter.

## Persistence Manager

[`internal/persistence/manager.go`](../../internal/persistence/manager.go) adds filesystem and
lifecycle behavior:

1. Capture engine entries and cluster metadata.
2. Seal the model.
3. Write to a temporary file in the target directory.
4. flush and `Sync` the file;
5. rename it to the final checksummed filename;
6. retain only the newest configured number of valid snapshots.

Invalid files are not counted as valid retention candidates. On startup, restore scans candidates
newest first and falls back past corrupt newer files. It also rejects a valid snapshot belonging to
another node.

The same manager supports periodic snapshots and manual `Snapshot()` calls. The HTTP trigger is
`POST /admin/snapshot`; `adminctl snapshot` calls that endpoint.

## Cluster Restore Ordering

The persistence manager receives metadata provider/restorer callbacks from the cluster manager.
During startup it restores cluster metadata before engine entries. The cluster manager then asks
healthy peers for a newer metadata version before listeners serve normal traffic. Version and term
checks fence stale ownership.

Older snapshot models that lack authoritative leader/replica IDs can still restore their data, but
ownership is rebuilt from configuration and refreshed from peers rather than guessed.

## Files And Tests To Read

1. [`snapshot/model.go`](../../internal/snapshot/model.go)
2. [`snapshot/codec.go`](../../internal/snapshot/codec.go)
3. [`engine/snapshot.go`](../../internal/engine/snapshot.go)
4. [`persistence/manager.go`](../../internal/persistence/manager.go)
5. [`persistence/manager_test.go`](../../internal/persistence/manager_test.go)

Run [`scripts/demo-persistence.ps1`](../../scripts/demo-persistence.ps1) to create data, snapshot,
terminate the node, restart it, and verify strings, lists, and sorted sets in both formats.
