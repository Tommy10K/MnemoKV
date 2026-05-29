import { Callout, H2, P, UL } from "../components"

export function Chapter10() {
  return (
    <>
      <P>
        Gossip is how every node finds out what every other node is doing without needing a
        central coordinator. Each node periodically picks a few peers at random and exchanges
        small status updates. The information spreads exponentially fast, even though no single
        message reaches everyone.
      </P>

      <H2>What a gossip message carries</H2>
      <UL>
        <li>The sender's view of every peer: alive, suspect, or unavailable.</li>
        <li>The sender's term and other coarse cluster metadata.</li>
        <li>A timestamp so receivers can detect stale information.</li>
      </UL>

      <H2>State transitions</H2>
      <UL>
        <li>
          <strong>Healthy.</strong> Recently heard from. The default state.
        </li>
        <li>
          <strong>Suspect.</strong> Silent for longer than the suspect threshold. Other nodes
          start counting it against the failure detector.
        </li>
        <li>
          <strong>Unavailable.</strong> Crossed the failure threshold. The cluster considers it
          gone; if it owned slots, failover begins.
        </li>
        <li>
          <strong>Recovering.</strong> Came back after being gone. Catches up before serving
          traffic.
        </li>
      </UL>

      <H2>Why gossip beats heartbeats-to-one-leader</H2>
      <UL>
        <li>No central bottleneck — every node observes every other.</li>
        <li>Failure detection is decentralized; a partition cannot hide a dead node.</li>
        <li>Bandwidth scales as O(log N) per node, not O(N).</li>
      </UL>

      <Callout>
        Gossip alone does not make decisions. It produces a shared view of who is up. The
        control plane reads that view and triggers election when a slot leader goes missing —
        the topic of the next chapter.
      </Callout>
    </>
  )
}
