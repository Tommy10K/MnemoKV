import { Callout, H2, P, Pre, UL } from "../components"

export function Chapter06() {
  return (
    <>
      <P>
        If every command had to take a single global lock, only one operation could run at a
        time. The CPU might have 16 cores, but the store would behave like a single-threaded
        process. Lock striping fixes this by splitting the store into N independent shards, each
        with its own lock.
      </P>

      <H2>How striping works</H2>
      <Pre>{`hash(key) % N  →  stripe index
stripe.lock()
stripe.map[key] = value
stripe.unlock()`}</Pre>
      <P>
        Two clients writing to different keys almost always land on different stripes and can
        proceed in parallel. Two clients writing the same key still serialize — that's correct.
        The contention only happens at the granularity of stripes, not the whole store.
      </P>

      <H2>Choosing N</H2>
      <UL>
        <li>Too few stripes → false contention; threads queue up on the same lock.</li>
        <li>Too many stripes → wasted memory and worse cache behavior.</li>
        <li>A common rule of thumb is 16-128 stripes for a few dozen cores.</li>
      </UL>

      <H2>What striping does <em>not</em> do</H2>
      <UL>
        <li>It does not make operations on the same key faster.</li>
        <li>It does not eliminate the need for per-value locks (lists and zsets still have their own).</li>
        <li>It does not protect against hot keys; if one key gets 90% of traffic, one stripe will saturate.</li>
      </UL>

      <Callout>
        MnemoKV configures the stripe count at startup. The default works for most workloads.
        When you watch the dashboard under load, the throughput you see is bounded by how many
        stripes can be busy at once.
      </Callout>
    </>
  )
}
