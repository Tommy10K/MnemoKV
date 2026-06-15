import { Callout, H2, P, UL } from "../components"

export function Chapter11() {
  return (
    <>
      <P>
        Safe failover replaces an unavailable leader while preventing two nodes from accepting
        conflicting writes. That normally requires agreed terms, deterministic elections,
        fencing before mutation, replica-freshness checks, and a recovery protocol.
      </P>

      <H2>Pieces present in MnemoKV</H2>
      <UL>
        <li>The control plane tracks terms and slot leaders in memory.</li>
        <li>An election monitor reacts to locally unavailable members.</li>
        <li>The UI records term changes observed from the selected node.</li>
      </UL>

      <H2>Current limitations</H2>
      <UL>
        <li>Candidate selection is not a quorum vote and does not compare replica freshness.</li>
        <li>Nodes do not have a protocol that guarantees agreement on one leader.</li>
        <li>Terms and sequences are not checked when replicated commands are applied.</li>
        <li>Async write fencing currently happens after local mutation, so rejection can be too late.</li>
        <li>There is no implemented catch-up flow for a returning node.</li>
      </UL>

      <Callout>
        Do not use the current prototype to demonstrate safe automatic failover. A valid exercise
        is to stop a node, compare the local observations reported by the remaining nodes, and
        identify where they disagree. Auto-failover should stay off in demo configs until the
        backend has an agreed election and fencing design.
      </Callout>
    </>
  )
}
