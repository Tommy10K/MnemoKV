# ADR 003: Memory and Eviction Semantics

## Status

Accepted. The engine tracks approximate dataset bytes and enforces a hard accounted memory cap when `engine.memoryLimitBytes` is positive.

## Decision

- Memory accounting is *approximate*. Every entry contributes the size of its key, its value payload, and a fixed overhead constant for entry metadata. Containers (lists, zsets) sum their elements.
- Memory accounting is updated on every create, update, and delete in the store, regardless of whether eviction is enabled.
- The configured `engine.memoryLimitBytes` is treated as a **hard accounted-dataset limit**. Memory-growing writes reserve enough capacity before they commit, so the dataset is not intentionally made visible above the configured limit.
- A value of `0` for `memoryLimitBytes` means "no limit"; eviction is disabled.
- The `noeviction` policy never deletes keys automatically. It rejects memory-growing writes that do not fit, while deletes, expirations, pops, flushes, and size-reducing updates remain available.
- FIFO, LRU, LFU, and Random evict selected victims before the incoming write commits. They share the same admission flow and differ only in victim selection.
- Switching the eviction policy at runtime changes future admission decisions. The store contents are not touched.
