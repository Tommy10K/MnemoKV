import { SkipListVisual } from "@/components/visuals/SkipListVisual"
import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter05() {
  return (
    <>
      <P>
        A sorted set keeps members in score order while still letting you look up any member by
        name in O(1). MnemoKV implements this with two structures that mirror each other: a hash
        map from member to score, and a skip list ordered by (score, member).
      </P>

      <H2>The skip list</H2>
      <P>
        A skip list is a sorted linked list with extra "express lane" pointers at random levels.
        Search starts at the top level and drops down whenever the next pointer would overshoot.
        Insert, delete, and search are all O(log n) on average — same complexity as a balanced
        tree, but the code is much simpler.
      </P>
      <Pre>{`L3:  head ─────────────────────────────→ NIL
L2:  head ─────────→ 5 ─────────→ 12 ──→ NIL
L1:  head ──→ 2 ──→ 5 ──→ 9 ──→ 12 ──→ NIL
L0:  head → 2 → 5 → 7 → 9 → 11 → 12 → 15 → NIL`}</Pre>

      <SkipListVisual />

      <H2>Why two structures</H2>
      <UL>
        <li>
          The hash map answers <Code>ZSCORE member</Code> in O(1).
        </li>
        <li>
          The current skip list answers <Code>ZRANGE start stop</Code> in O(n + m), because it
          walks from the head to the requested rank before returning m members.
        </li>
        <li>
          A single <Code>ZADD</Code> updates both structures while the key is protected by its
          store stripe and sorted-set synchronization.
        </li>
      </UL>

      <H2>The commands implemented</H2>
      <UL>
        <li>
          <Code>ZADD key score member</Code> — insert or update
        </li>
        <li>
          <Code>ZRANGE key start stop</Code> — return members in score order
        </li>
        <li>
          <Code>ZCARD key</Code> / <Code>ZSCORE key member</Code> — inspect size or one score
        </li>
      </UL>

      <Callout>
        Sorted sets are the algorithmically interesting type. When the benchmarks show zsets
        running slower than strings, maintaining order is one likely contributor. Benchmark
        results also include locking, allocation, key distribution, and range-walk costs.
      </Callout>
    </>
  )
}
