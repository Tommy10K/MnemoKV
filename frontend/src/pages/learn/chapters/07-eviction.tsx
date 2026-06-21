import { EvictionVisual } from "@/components/visuals/EvictionVisual"
import { Callout, H2, P, UL } from "../components"

export function Chapter07() {
  return (
    <>
      <P>
        Eviction is what happens when the store hits its memory limit and a new write needs space.
        Something already in memory has to go. The choice of which key to evict is the
        <em> eviction policy</em>, and different policies behave very differently under the same
        workload.
      </P>

      <H2>How MnemoKV measures memory</H2>
      <UL>
        <li>Every write reserves bytes against a counter.</li>
        <li>Every delete releases them.</li>
        <li>Memory-growing writes must reserve enough capacity before they become visible.</li>
        <li>Reads never trigger eviction.</li>
        <li>The accounting is approximate — it tracks the dominant cost of each value, not every byte of overhead.</li>
      </UL>

      <H2>The selectable policies</H2>
      <UL>
        <li>
          <strong>FIFO.</strong> Evict the oldest insertion. Cheap, predictable, but ignores
          access patterns entirely.
        </li>
        <li>
          <strong>Random.</strong> Take candidates from a random store sample without scoring a
          "worst" item.
        </li>
        <li>
          <strong>LRU (Least Recently Used).</strong> Evict the key not touched for the longest
          time. Great when there is temporal locality.
        </li>
        <li>
          <strong>LFU (Least Frequently Used).</strong> Evict the key with the lowest access
          count. Great for stable hot keys, bad when popularity shifts.
        </li>
        <li>
          <strong>Noeviction.</strong> Never chooses a victim. When a hard memory limit is set, it
          rejects memory-growing writes instead of deleting existing keys.
        </li>
      </UL>

      <EvictionVisual />

      <H2>Why sampling</H2>
      <P>
        Maintaining a perfectly accurate LRU or LFU list across millions of keys is expensive.
        MnemoKV's FIFO, LRU, and LFU policies score a small sample of candidates rather than
        maintaining a perfect global order. Random uses candidates from the sample directly.
        Sampling reduces bookkeeping cost at the expense of approximate choices.
      </P>

      <Callout>
        The Eviction Lab can switch policy on a live node. FIFO, LRU, LFU, and Random evict before
        a memory-growing write commits; noeviction keeps existing keys and rejects the write.
      </Callout>
    </>
  )
}
