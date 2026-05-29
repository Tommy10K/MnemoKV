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
        <li>When the counter exceeds the configured limit, the eviction manager runs.</li>
        <li>The accounting is approximate — it tracks the dominant cost of each value, not every byte of overhead.</li>
      </UL>

      <H2>The four policies</H2>
      <UL>
        <li>
          <strong>FIFO.</strong> Evict the oldest insertion. Cheap, predictable, but ignores
          access patterns entirely.
        </li>
        <li>
          <strong>Random.</strong> Pick a random sample and drop the worst. Surprisingly
          competitive when access is uniform.
        </li>
        <li>
          <strong>LRU (Least Recently Used).</strong> Evict the key not touched for the longest
          time. Great when there is temporal locality.
        </li>
        <li>
          <strong>LFU (Least Frequently Used).</strong> Evict the key with the lowest access
          count. Great for stable hot keys, bad when popularity shifts.
        </li>
      </UL>

      <H2>Why sampling</H2>
      <P>
        Maintaining a perfectly accurate LRU or LFU list across millions of keys is expensive.
        MnemoKV uses approximate policies: it samples a small handful of candidates per eviction
        cycle and picks the worst from the sample. Real Redis does the same. The accuracy you
        lose is negligible compared to the work you save.
      </P>

      <Callout>
        The eviction policy is the one engine setting that can be switched at runtime today. The
        Use section's Eviction Lab (later phase) lets you flip between policies on a live node
        and watch how memory and throughput respond.
      </Callout>
    </>
  )
}
