import { Callout, Code, H2, P, UL } from "../components"

export function Chapter01() {
  return (
    <>
      <P>
        An in-memory database keeps its entire working set in RAM rather than on disk. Reads and
        writes never wait for a spinning platter or even an SSD — they touch CPU cache and main
        memory directly. That makes them extremely fast, but it also means losing power can lose
        data, and the dataset is bounded by how much RAM you can afford.
      </P>

      <H2>Why RAM is faster</H2>
      <UL>
        <li>RAM access latency is ~100 nanoseconds; SSD reads are tens of microseconds.</li>
        <li>No filesystem layer, no block cache miss, no I/O scheduling.</li>
        <li>Operations can be lock-free or use cheap in-process synchronization.</li>
      </UL>

      <H2>The trade-offs you accept</H2>
      <UL>
        <li>
          <strong>Volatility.</strong> If the process dies, the data is gone unless you replicate
          or snapshot to disk.
        </li>
        <li>
          <strong>Size limit.</strong> A single node can hold tens of GB. Beyond that you shard.
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
