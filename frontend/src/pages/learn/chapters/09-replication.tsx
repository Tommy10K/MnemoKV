import { Callout, Code, H2, P, UL } from "../components"

export function Chapter09() {
  return (
    <>
      <P>
        Replication keeps additional copies of writes on other nodes. A complete replicated
        system must define ownership, ordering, acknowledgements, duplicate handling, and what a
        successful client reply guarantees during failures.
      </P>

      <H2>MnemoKV asynchronous fan-out</H2>
      <UL>
        <li>A local write is applied before replication is queued.</li>
        <li>One shared queue sends writes to configured peers sequentially.</li>
        <li>A slow peer can delay delivery to peers behind it.</li>
        <li>A process failure can lose queued writes.</li>
      </UL>

      <H2>MnemoKV synchronous fan-out</H2>
      <UL>
        <li>The node sends the command to every configured peer before local execution.</li>
        <li>This adds network latency and can fail when a peer is unavailable.</li>
        <li>It is not a quorum, consensus, rollback, or durable commit protocol.</li>
        <li>An acknowledgement must not be interpreted as a linearizable or disk-durable write.</li>
      </UL>

      <H2>Replication records</H2>
      <P>
        Internal records contain source, slot, term, sequence, and timestamp metadata, but the
        current <Code>REPLICATE</Code> wire command does not transmit or validate all of it. The
        follower applies the inner command without rejecting stale, duplicate, or out-of-order
        records.
      </P>

      <Callout>
        Use replication as a prototype demonstration of command fan-out between in-memory
        processes. It is not yet a safe foundation for automatic failover or acknowledged-write
        durability.
      </Callout>
    </>
  )
}
