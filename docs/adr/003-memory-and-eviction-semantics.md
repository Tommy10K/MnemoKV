# ADR 003: Memory and Eviction Semantics

## Status

Accepted for the future engine work. The baseline milestone tracks memory bytes but does not enforce a hard cap or evict.

## Decision

- Memory accounting is *approximate*. Every entry contributes the size of its key, its value payload, and a fixed overhead constant for entry metadata. Containers (lists, zsets) sum their elements.
- Memory accounting is updated on every create, update, and delete in the store, regardless of whether eviction is enabled.
- The configured `engine.memoryLimitBytes` is treated as a **soft limit**: writes are accepted, then the eviction manager is asked to bring usage back below the limit asynchronously after the write commits. A future phase may add a hard-cap mode.
- A value of `0` for `memoryLimitBytes` means "no limit"; eviction is disabled.
- Switching the eviction policy at runtime resets the policy-specific bookkeeping (LRU list, LFU counters). The store contents are not touched.
