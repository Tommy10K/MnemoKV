import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter03() {
  return (
    <>
      <P>
        Strings are the simplest value type and the foundation of every key-value store. A string
        in MnemoKV is just a byte slice stored against a key in a hash map. All the interesting
        operations (<Code>SET</Code>, <Code>GET</Code>, <Code>INCR</Code>, <Code>DEL</Code>) are
        O(1) on average because the underlying lookup is a hash.
      </P>

      <H2>How a string lives in memory</H2>
      <Pre>{`key "foo"  →  Entry { type: String, value: []byte("bar"), expireAt: 0 }`}</Pre>
      <P>
        The entry carries metadata (type, expiration, last-access timestamp) alongside the value.
        That metadata is what makes <Code>EXPIRE</Code>, <Code>TTL</Code>, and the various
        eviction policies possible. Reads update access metadata while holding the key's stripe
        lock, so that metadata has a real hot-path cost.
      </P>

      <H2>The commands implemented</H2>
      <UL>
        <li>
          <Code>SET key value</Code> — replace or insert
        </li>
        <li>
          <Code>GET key</Code> — return value or nil
        </li>
        <li>
          <Code>INCR key</Code> — parse as integer, add 1, store back
        </li>
        <li>
          <Code>DEL key</Code> — remove and free memory
        </li>
        <li>
          <Code>EXISTS key</Code> — boolean check
        </li>
        <li>
          <Code>EXPIRE key seconds</Code> / <Code>TTL key</Code> — set or read expiration
        </li>
      </UL>

      <H2>Why INCR is interesting</H2>
      <P>
        <Code>INCR</Code> looks trivial but it must be atomic. Two concurrent
        <Code>INCR</Code>s on the same counter cannot both read the same starting value. MnemoKV
        guarantees this by locking the stripe that owns the key for the duration of the
        read-modify-write — see the lock striping chapter.
      </P>

      <Callout>
        Strings are the baseline. They are expected to be cheaper than the ordered or linked data
        structures, but the actual difference should be measured for the current workload.
      </Callout>
    </>
  )
}
