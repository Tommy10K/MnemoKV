import { Callout, Code, H2, P, UL } from "../components"

export function Chapter08() {
  return (
    <>
      <P>
        Sharding spreads keys across nodes so one machine does not hold the entire dataset. A
        common design uses consistent hashing: nodes and keys are placed on a ring, and each key
        is assigned to the next node clockwise. Adding or removing a node then moves only part of
        the keyspace instead of remapping nearly every key.
      </P>

      <H2>General routing model</H2>
      <UL>
        <li>Hash the command key and resolve one authoritative owner.</li>
        <li>Execute locally when the receiving node is the owner.</li>
        <li>Otherwise proxy or redirect the command to the owner.</li>
        <li>Use the same ownership decision for writes, reads, replication, and status reporting.</li>
      </UL>

      <H2>What MnemoKV implements today</H2>
      <UL>
        <li>A consistent-hash ring can calculate an owner for a key.</li>
        <li>The cluster API exposes configured peers and one node's membership observations.</li>
        <li>Live RESP and HTTP commands still execute against the node that received them.</li>
        <li>The ring therefore describes intended ownership but does not enforce it.</li>
      </UL>

      <Callout>
        Cluster mode is experimental. Connecting to two nodes and writing the same key can create
        separate local values because command routing is not wired into the request path. Treat
        the Cluster page as an observability view, not proof that keys are sharded.
      </Callout>

      <P>
        A useful verification exercise is to connect to each node with <Code>redis-cli</Code>,
        write the same key, and observe the limitation before authoritative routing is added.
      </P>
    </>
  )
}
