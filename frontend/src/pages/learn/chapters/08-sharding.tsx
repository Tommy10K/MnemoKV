import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter08() {
  return (
    <>
      <P>
        Once you outgrow a single machine, you need a way to spread keys across many nodes. The
        naive answer — <Code>hash(key) % N</Code> — is terrible: when N changes, almost every
        key moves. Consistent hashing solves this by mapping nodes and keys onto the same
        circular space.
      </P>

      <H2>The hash ring</H2>
      <Pre>{`     node-2
        ↑
   ┌────●────┐
   │         │
node-3       node-1
   │         │
   └────●────┘
        ↑
     node-1`}</Pre>
      <P>
        Each node owns the arc of the ring immediately before its position. A key's owner is the
        first node found by walking clockwise from <Code>hash(key)</Code>. Adding or removing a
        node only reshuffles the keys whose arc just changed hands — typically <Code>1/N</Code>
        of the dataset.
      </P>

      <H2>Virtual nodes</H2>
      <P>
        A real node usually appears on the ring many times (virtual nodes). This smooths out
        ownership so that no single node ends up with a disproportionately large arc, and
        adding a new node draws traffic from several existing nodes instead of just one
        neighbor.
      </P>

      <H2>Local bypass</H2>
      <UL>
        <li>A client can connect to any node and issue any command.</li>
        <li>If the key belongs locally, the engine handles it in-process.</li>
        <li>If it belongs to a peer, the node proxies the command over an internal connection.</li>
        <li>The result looks identical to the client either way.</li>
      </UL>

      <Callout>
        MnemoKV's ring is built at startup from the configured peer list. The router uses it on
        every command. When you watch the cluster page (later phase), you can see which node owns
        which slice of the keyspace.
      </Callout>
    </>
  )
}
