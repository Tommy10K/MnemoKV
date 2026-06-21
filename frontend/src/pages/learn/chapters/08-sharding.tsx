import { Callout, Code, H2, P, UL } from "../components"

export function Chapter08() {
  return (
    <>
      <P>
        Sharding spreads keys across nodes so one machine does not hold the entire dataset. A
        common design divides the keyspace into fixed slots and assigns each slot to an owner.
        MnemoKV uses 1,024 slots and an explicit metadata table so every request path can make the
        same routing decision.
      </P>

      <H2>General routing model</H2>
      <UL>
        <li>Hash every command key to a slot and resolve its authoritative leader.</li>
        <li>Execute locally when the receiving node is the owner.</li>
        <li>Otherwise proxy or redirect the command to the owner.</li>
        <li>Use the same ownership decision for writes, reads, replication, and status reporting.</li>
      </UL>

      <H2>What MnemoKV implements today</H2>
      <UL>
        <li>Sorted peer IDs receive deterministic contiguous slot ranges.</li>
        <li>Any RESP or HTTP gateway proxies a command to the slot leader.</li>
        <li>Multi-key commands are accepted only when every key maps to one slot.</li>
        <li>The same metadata drives routing, replication, fencing, repair, and status reporting.</li>
      </UL>

      <Callout>
        MnemoKV does not implement hash tags. A cross-slot multi-key command returns
        <Code>CROSSSLOT</Code>; issue one command per slot instead.
      </Callout>

      <P>
        Connect to each node with <Code>redis-cli</Code>, write and read the same key through
        different gateways, then inspect its slot assignment on the Cluster page.
      </P>
    </>
  )
}
