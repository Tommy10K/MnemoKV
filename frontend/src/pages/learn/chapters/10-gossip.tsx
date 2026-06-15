import { Callout, H2, P, UL } from "../components"

export function Chapter10() {
  return (
    <>
      <P>
        Gossip protocols spread membership information through partial, repeated peer exchanges.
        Each node contacts only part of the cluster, and observations converge over time without
        one central coordinator.
      </P>

      <H2>General membership states</H2>
      <UL>
        <li><strong>Healthy:</strong> the node responded recently.</li>
        <li><strong>Suspect:</strong> recent checks failed, but failure is not yet confirmed.</li>
        <li><strong>Unavailable:</strong> the failure threshold was crossed.</li>
        <li><strong>Unknown:</strong> the reporting node has no observation to support a stronger claim.</li>
      </UL>

      <H2>What MnemoKV implements today</H2>
      <UL>
        <li>Each node directly pings every configured peer.</li>
        <li>The resulting membership table is that node's local observation.</li>
        <li>There is no random peer exchange or cluster-wide convergence protocol.</li>
        <li>A missing observation does not prove that a node is recovering.</li>
      </UL>

      <Callout>
        The Cluster page labels its source node and shows configured peers without inventing a
        recovery state. Read the table as heartbeat-based failure observation, not agreed global
        membership.
      </Callout>
    </>
  )
}
