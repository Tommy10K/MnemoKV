import { LinkedListVisual } from "@/components/visuals/LinkedListVisual"
import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter04() {
  return (
    <>
      <P>
        A list in MnemoKV is a doubly linked list. Each node holds a value and pointers to its
        previous and next neighbors. The list value itself holds pointers to the head and tail,
        and a length counter. That layout is what makes push and pop from either end O(1).
      </P>

      <LinkedListVisual />

      <H2>Why doubly linked, not an array?</H2>
      <UL>
        <li>Arrays would need to shift elements on every <Code>LPOP</Code> — O(n).</li>
        <li>Doubly linked nodes can be unlinked from either end without touching the rest.</li>
        <li>The trade-off: random access by index is O(n), but lists are rarely indexed that way.</li>
      </UL>

      <H2>A push on the right</H2>
      <Pre>{`Before: HEAD → A ↔ B ↔ C ← TAIL
RPUSH "D"
After:  HEAD → A ↔ B ↔ C ↔ D ← TAIL`}</Pre>

      <H2>The commands implemented</H2>
      <UL>
        <li>
          <Code>LPUSH key v</Code> / <Code>RPUSH key v</Code> — prepend / append
        </li>
        <li>
          <Code>LPOP key</Code> / <Code>RPOP key</Code> — remove and return from either end
        </li>
      </UL>

      <H2>Use them as</H2>
      <UL>
        <li>
          <strong>Queues:</strong> <Code>LPUSH</Code> from producers, <Code>RPOP</Code> from
          consumers
        </li>
        <li>
          <strong>Stacks:</strong> <Code>LPUSH</Code> and <Code>LPOP</Code> on the same end
        </li>
        <li>
          <strong>Sliding logs:</strong> keep the most recent N events
        </li>
      </UL>

      <Callout>
        Lists carry their own lock. A push or pop only blocks other operations on the same list,
        not other keys. That matters when many producers are writing to the same queue at once.
      </Callout>
    </>
  )
}
