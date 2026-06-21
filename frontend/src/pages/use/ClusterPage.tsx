import { useMemo } from "react"
import type { ClusterStateResponse, PeerStatus } from "@/api/types"
import { TopologyGraph } from "@/components/cluster/TopologyGraph"
import { useClusterState, type MetadataChange } from "@/hooks/useClusterState"
import { useAppStore } from "@/store/appStore"

export function ClusterPage() {
  const baseUrl = useAppStore((state) => state.apiBaseUrl)
  const { state, reachable, metadataHistory, error } = useClusterState()

  if (!reachable || state === null) {
    return (
      <div role={error?.includes("unexpected response") ? "alert" : undefined} className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-6 text-sm text-[#9ca3af]">
        <p className="text-white">{error?.includes("unexpected response") ? error : "Cluster state is not available."}</p>
        <p className="mt-2">Make sure a node is running and reachable.</p>
      </div>
    )
  }

  if (!state.enabled) return <StandaloneView nodeId={state.nodeId} />
  return <ClusterView state={state} metadataHistory={metadataHistory} baseUrl={baseUrl} />
}

function StandaloneView({ nodeId }: { nodeId: string }) {
  return (
    <div className="flex flex-col gap-4">
      <header>
        <h1 className="text-2xl font-semibold text-white">Cluster</h1>
        <p className="text-sm text-[#9ca3af]">This node is running in standalone mode.</p>
      </header>
      <div className="flex w-fit items-center gap-3 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm">
        <span className="h-2 w-2 rounded-full bg-emerald-400" />
        <span className="font-mono text-white">{nodeId}</span>
        <span className="text-[#9ca3af]">standalone</span>
      </div>
    </div>
  )
}

function ClusterView({
  state,
  metadataHistory,
  baseUrl,
}: {
  state: ClusterStateResponse
  metadataHistory: MetadataChange[]
  baseUrl: string
}) {
  const peers = useMemo<PeerStatus[]>(
    () => fillMissing(state.membership ?? [], state.peers, state.nodeId),
    [state.membership, state.peers, state.nodeId],
  )
  const counts = peers.reduce<Record<string, number>>((acc, peer) => {
    acc[peer.state] = (acc[peer.state] ?? 0) + 1
    return acc
  }, {})
  const slots = state.slots ?? []
  const localLeaderSlots = slots.filter((slot) => slot.localRole === "leader").length
  const localReplicaSlots = slots.filter((slot) => slot.localRole === "replica").length
  const unreadyReplicas = slots.filter((slot) => !slot.replicaReady).length

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Cluster</h1>
        <p className="text-sm text-[#9ca3af]">
          Authoritative slot metadata observed by <span className="font-mono text-white">{state.nodeId}</span>{" "}
          at <span className="font-mono">{baseUrl}</span>. Failover and replica repair are manual.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Metadata version">{state.metadataVersion ?? 0}</StatCard>
        <StatCard label="Local slots">{localLeaderSlots} leader / {localReplicaSlots} replica</StatCard>
        <StatCard label="Replica readiness">{slots.length - unreadyReplicas}/{slots.length}</StatCard>
        <StatCard label="Observed healthy">{counts.healthy ?? 0}/{peers.length}</StatCard>
      </div>

      <TopologyGraph peers={peers} selfId={state.nodeId} />

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
          <h2 className="mb-3 text-sm uppercase tracking-wide text-[#8b949e]">Slot assignments</h2>
          <div className="max-h-80 overflow-auto" tabIndex={0} aria-label="Scrollable slot assignments">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">Cluster slot leaders, replicas, terms, and sequences</caption>
              <thead className="text-xs uppercase tracking-wide text-[#8b949e]">
                <tr><th className="pb-2">Slot</th><th className="pb-2">Leader</th><th className="pb-2">Replica</th><th className="pb-2">Term / seq</th></tr>
              </thead>
              <tbody className="font-mono text-[#e6edf3]">
                {slots.map((slot) => (
                  <tr key={slot.number} className="border-t border-[#1f2937]">
                    <td className="py-1.5">{slot.number}</td>
                    <td>{slot.leaderId}</td>
                    <td className={slot.replicaReady ? "" : "text-amber-300"}>{slot.replicaId || "unassigned"}</td>
                    <td>{slot.term} / {slot.lastSequence}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
          <h2 className="mb-3 text-sm uppercase tracking-wide text-[#8b949e]">Metadata changes</h2>
          <p className="mb-3 text-xs text-[#8b949e]">
            {state.clusterId} · {state.slotCount} slots · {state.routingMode} routing · {state.failoverMode} failover
          </p>
          {metadataHistory.length === 0 ? (
            <p className="text-sm text-[#8b949e]">No metadata changes observed.</p>
          ) : (
            <ol className="space-y-2 text-sm">
              {metadataHistory.slice().reverse().map((entry, i) => (
                <li key={`${entry.at}-${i}`} className="flex items-baseline gap-3">
                  <span className="text-xs text-[#8b949e]">{new Date(entry.at).toLocaleTimeString()}</span>
                  <span className="font-mono text-[#e6edf3]">version → {entry.version}</span>
                </li>
              ))}
            </ol>
          )}
        </section>
      </div>
    </div>
  )
}

function StatCard({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4"><div className="text-xs uppercase tracking-wide text-[#8b949e]">{label}</div><div className="mt-1 text-lg text-white">{children}</div></div>
}

function fillMissing(membership: PeerStatus[], peers: string[], selfId: string): PeerStatus[] {
  const byId = new Map(membership.map((member) => [member.id, member]))
  return peers.map((id) => byId.get(id) ?? { id, address: "—", state: id === selfId ? "healthy" : "unknown" })
}
