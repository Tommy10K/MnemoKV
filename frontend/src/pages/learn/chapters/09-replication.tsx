import { Callout, Code, H2, P, UL } from "../components"

export function Chapter09() {
  return (
    <>
      <P>
        Replication keeps additional copies of writes on other nodes. A complete replicated
        system must define ownership, ordering, acknowledgements, duplicate handling, and what a
        successful client reply guarantees during failures.
      </P>

      <H2>MnemoKV replication contract</H2>
      <UL>
        <li>Every slot has exactly one leader and one assigned replica.</li>
        <li>The leader sends the next ordered record to that replica before local mutation.</li>
        <li>The client receives success only after the replica acknowledges application.</li>
        <li>A missing leader, missing replica, or replica gap rejects the write.</li>
      </UL>

      <H2>Replication records</H2>
      <P>
        Each <Code>REPLICATE</Code> request carries source node, slot, term, sequence, and command
        payload. The follower accepts only its current leader and term, applies the next sequence,
        treats an exact duplicate idempotently, and rejects gaps or stale records.
      </P>

      <Callout>
        Acknowledgement means the write is present in memory on the leader and its assigned
        replica. Disk durability still depends on the configured snapshot interval.
      </Callout>
    </>
  )
}
