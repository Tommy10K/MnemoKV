import { Callout, Code, H2, P, UL } from "../components"

export function Chapter11() {
  return (
    <>
      <P>
        MnemoKV uses explicit manual failover. Operators promote the assigned replica, choose a
        replacement replica, and run a full-slot synchronization before writes resume.
      </P>

      <H2>Manual workflow</H2>
      <UL>
        <li>Promote the current assigned replica; this increments the slot term.</li>
        <li>Assign a new replica; the slot remains unavailable for writes.</li>
        <li>Synchronize the complete slot snapshot and mark the replica ready.</li>
        <li>Metadata is broadcast and persisted in node snapshots.</li>
      </UL>

      <H2>Safety boundaries</H2>
      <UL>
        <li>There is no automatic election or consensus protocol.</li>
        <li>Old leaders are fenced by newer metadata versions and slot terms.</li>
        <li>A stale node fetches newer metadata before serving cluster traffic.</li>
        <li>Writes stop whenever the current leader or ready replica is unavailable.</li>
      </UL>

      <Callout>
        Manual operations are available through the cluster admin API and <Code>adminctl</Code>.
        Promotion alone is intentionally insufficient: assign and synchronize a replacement
        replica before expecting writes to succeed.
      </Callout>
    </>
  )
}
