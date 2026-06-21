import { Callout, Code, H2, P, UL } from "../components"

export function Chapter01() {
  return (
    <>
      <P>
        An in-memory database keeps its working set in RAM rather than reading values from disk on
        every command. That usually makes data access much faster, but synchronization, parsing,
        networking, and memory allocation still contribute latency. The dataset is also bounded
        by available RAM.
      </P>

      <H2>Why RAM is faster</H2>
      <UL>
        <li>RAM access is typically much faster than storage I/O, though exact figures vary by hardware.</li>
        <li>No filesystem layer, no block cache miss, no I/O scheduling.</li>
        <li>Implementations may be lock-free or use in-process synchronization; MnemoKV uses locks.</li>
      </UL>

      <H2>The trade-offs you accept</H2>
      <UL>
        <li>
          <strong>Volatility.</strong> MnemoKV can periodically persist JSON or binary snapshots,
          but it has no write-ahead log, so writes after the latest snapshot can still be lost.
          Replication provides another in-memory copy rather than independent disk durability.
        </li>
        <li>
          <strong>Size limit.</strong> Capacity depends on the machine and workload. Sharding is one
          way to distribute data when one node is not enough.
        </li>
        <li>
          <strong>Eviction.</strong> When memory fills up, something has to leave. That decision
          becomes a first-class concern.
        </li>
      </UL>

      <H2>When you actually want one</H2>
      <UL>
        <li>Caches in front of a slower system of record</li>
        <li>Session storage, rate limiters, leaderboards, real-time counters</li>
        <li>Coordination primitives like locks and queues</li>
        <li>Hot data that must respond in single-digit milliseconds</li>
      </UL>

      <Callout>
        MnemoKV is a teaching-grade in-memory store. It speaks the same protocol as Redis
        (<Code>RESP2</Code>), implements the same families of data structures, and exposes the
        same kinds of trade-offs so you can study them directly.
      </Callout>
    </>
  )
}
