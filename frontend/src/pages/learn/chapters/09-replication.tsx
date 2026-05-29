import { Callout, Code, H2, P, UL } from "../components"

export function Chapter09() {
  return (
    <>
      <P>
        Replication is how a cluster keeps more than one copy of every key. When the leader for a
        slot accepts a write, that write must reach the followers — otherwise a leader crash
        loses data. MnemoKV supports two replication modes that make different trade-offs
        between latency and durability.
      </P>

      <H2>Async mode</H2>
      <UL>
        <li>The leader applies the write locally and returns success immediately.</li>
        <li>The write is queued for delivery to followers in the background.</li>
        <li>Latency is the same as a single-node write.</li>
        <li>If the leader dies before the queue drains, those writes are lost.</li>
      </UL>

      <H2>Strong mode</H2>
      <UL>
        <li>The leader waits until a configured number of followers acknowledge.</li>
        <li>The client sees higher latency — every write costs at least one extra round trip.</li>
        <li>Acknowledged writes are durable across leader crashes.</li>
      </UL>

      <H2>How followers apply writes</H2>
      <P>
        Each replicated write is encoded as a <Code>REPLICATE</Code> command containing the
        original command's arguments. The follower's engine executes it exactly like a local
        write, except a flag suppresses re-replication. The replication queue is per-target so a
        slow follower cannot block a fast one.
      </P>

      <H2>What replication does <em>not</em> give you</H2>
      <UL>
        <li>Cross-cluster consistency. Each slot has one leader; replication is leader-to-follower.</li>
        <li>Conflict resolution. There are no concurrent writers competing for the same key.</li>
        <li>Disk durability. Both modes keep data in memory; the trade-off is purely about how many copies exist.</li>
      </UL>

      <Callout>
        Replication is what makes failover safe. When a leader dies, the cluster promotes a
        follower that has the most recent state. The next chapter covers how gossip notices the
        leader is gone in the first place.
      </Callout>
    </>
  )
}
