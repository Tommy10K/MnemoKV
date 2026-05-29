import { Callout, H2, P, UL } from "../components"

export function Chapter11() {
  return (
    <>
      <P>
        Failover is the act of replacing a dead leader with a live follower so writes can
        continue. It sounds simple until you realize the dead leader might not actually be dead —
        it might be slow, partitioned, or about to recover. Doing failover safely requires a
        mechanism that prevents two nodes from acting as leader at the same time.
      </P>

      <H2>The term</H2>
      <P>
        Every leadership change increments a monotonic <em>term</em> number. Every write the
        engine accepts is tagged with the term that was current at the time. A follower will
        reject any write tagged with a term older than the one it has seen. This is called
        stale-leader fencing: an old leader can still try to write, but no follower will accept
        the work, so the data never propagates.
      </P>

      <H2>The election flow</H2>
      <UL>
        <li>The election monitor watches the membership table for unavailable leaders.</li>
        <li>When one is detected, it picks a candidate follower for each affected slot.</li>
        <li>The control plane advances the term and broadcasts the new leader.</li>
        <li>Other nodes update their leader table; subsequent commands for that slot route to the new leader.</li>
      </UL>

      <H2>What automatic failover guarantees</H2>
      <UL>
        <li>At most one acknowledged leader per slot at any term.</li>
        <li>Writes accepted by an old leader after the term has advanced are dropped, not silently double-applied.</li>
        <li>Reads during the gap may fail temporarily, then succeed against the new leader.</li>
      </UL>

      <H2>What it does <em>not</em> guarantee</H2>
      <UL>
        <li>Zero downtime. There is always a short window where the slot has no leader.</li>
        <li>No data loss in async replication mode. The few writes still in the leader's outbound queue are gone.</li>
      </UL>

      <Callout>
        Run a 3-node cluster, watch the cluster page in the Use section, then kill the leader.
        You will see the term advance, a follower take over, and any in-flight stale writes
        rejected. That visible feedback loop is the point of building the failover demo in the
        first place.
      </Callout>
    </>
  )
}
