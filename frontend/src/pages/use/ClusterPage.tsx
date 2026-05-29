import { useMemo } from "react"
import type { PeerStatus } from "@/api/types"
import { TopologyGraph } from "@/components/cluster/TopologyGraph"
import { useClusterState, type TermChange } from "@/hooks/useClusterState"

export function ClusterPage() {
  const { state, reachable, termHistory } = useClusterState()

  if (!reachable || state === null) {
    return (
      <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-6 text-sm text-[#9ca3af]">
        <p className="text-white">Cluster state is not available.</p>
        <p className="mt-2">Make sure a node is running and reachable.</p>
      </div>
    )
  }

  if (!state.enabled) {
    return <StandaloneView nodeId={state.nodeId} />
  }

  return <ClusterView state={state} termHistory={termHistory} />
}

function StandaloneView({ nodeId }: { nodeId: string }) {
  return (
    <div className="flex flex-col gap-4">
      <header>
        <h1 className="text-2xl font-semibold text-white">Cluster</h1>
        <p className="text-sm text-[#9ca3af]">
          This node is running in standalone mode. There is no cluster to visualize.
        </p>
      </header>

      <div className="flex w-fit items-center gap-3 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm">
        <span className="h-2 w-2 rounded-full bg-emerald-400" />
        <span className="font-mono text-white">{nodeId}</span>
        <span className="text-[#9ca3af]">standalone</span>
      </div>

      <p className="text-xs text-[#6b7280]">
        Generate a clustered config from the <span className="font-mono">Configure</span> tab and
        start three nodes to see the topology here.
      </p>
    </div>
  )
}

type ClusterViewProps = {
  state: {
    nodeId: string
    enabled: boolean
    writeMode: string
    autoFailover: boolean
    term?: number
    peers: string[]
    membership?: PeerStatus[]
  }
  termHistory: TermChange[]
}

function ClusterView({ state, termHistory }: ClusterViewProps) {
  const peers = useMemo<PeerStatus[]>(
    () => fillMissing(state.membership ?? [], state.peers, state.nodeId),
    [state.membership, state.peers, state.nodeId],
  )

  const counts = peers.reduce<Record<string, number>>((acc, p) => {
    acc[p.state] = (acc[p.state] ?? 0) + 1
    return acc
  }, {})

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Cluster</h1>
        <p className="text-sm text-[#9ca3af]">
          Live view of cluster membership. Colors reflect gossip state; the dashed edges connect
          to nodes the gossip layer has lost contact with.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Term">{state.term ?? 0}</StatCard>
        <StatCard label="Write mode">{state.writeMode || "—"}</StatCard>
        <StatCard label="Auto failover">{state.autoFailover ? "on" : "off"}</StatCard>
        <StatCard label="Healthy peers">
          {(counts.healthy ?? 0)}/{peers.length}
        </StatCard>
      </div>

      <TopologyGraph peers={peers} selfId={state.nodeId} />

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
          <h2 className="mb-3 text-sm uppercase tracking-wide text-[#6b7280]">Membership</h2>
          <table className="w-full text-left text-sm">
            <thead className="text-xs uppercase tracking-wide text-[#6b7280]">
              <tr>
                <th className="pb-2">Node</th>
                <th className="pb-2">Address</th>
                <th className="pb-2">State</th>
              </tr>
            </thead>
            <tbody className="font-mono text-[#e6edf3]">
              {peers.map((p) => (
                <tr key={p.id} className="border-t border-[#1f2937]">
                  <td className="py-2">
                    {p.id}
                    {p.id === state.nodeId ? (
                      <span className="ml-2 font-sans text-xs text-emerald-300">self</span>
                    ) : null}
                  </td>
                  <td className="py-2">{p.address}</td>
                  <td className="py-2">
                    <StateDot state={p.state} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
          <h2 className="mb-3 text-sm uppercase tracking-wide text-[#6b7280]">Failover timeline</h2>
          {termHistory.length === 0 ? (
            <p className="text-sm text-[#6b7280]">
              No term changes recorded yet. Term {state.term ?? 0} is the starting term.
            </p>
          ) : (
            <ol className="space-y-2 text-sm">
              {termHistory
                .slice()
                .reverse()
                .map((entry, i) => (
                  <li key={`${entry.at}-${i}`} className="flex items-baseline gap-3">
                    <span className="text-xs text-[#6b7280]">
                      {new Date(entry.at).toLocaleTimeString()}
                    </span>
                    <span className="font-mono text-[#e6edf3]">term → {entry.term}</span>
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
  return (
    <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="text-xs uppercase tracking-wide text-[#6b7280]">{label}</div>
      <div className="mt-1 text-lg text-white">{children}</div>
    </div>
  )
}

function StateDot({ state }: { state: string }) {
  const colors: Record<string, string> = {
    healthy: "bg-emerald-400 text-emerald-300",
    recovering: "bg-sky-400 text-sky-300",
    suspect: "bg-yellow-400 text-yellow-300",
    unavailable: "bg-red-500 text-red-300",
  }
  const cls = colors[state] ?? "bg-gray-400 text-gray-300"
  const [dot, text] = cls.split(" ")
  return (
    <span className={`inline-flex items-center gap-2 text-xs ${text}`}>
      <span className={`h-2 w-2 rounded-full ${dot}`} />
      {state}
    </span>
  )
}

// fillMissing makes sure every configured peer shows up in the table even
// when the membership view has not yet learned about it. Anything missing
// is reported as recovering so the UI does not silently drop nodes.
function fillMissing(membership: PeerStatus[], peers: string[], selfId: string): PeerStatus[] {
  const byId = new Map(membership.map((m) => [m.id, m]))
  const out: PeerStatus[] = []
  const seen = new Set<string>()
  for (const id of peers) {
    if (byId.has(id)) {
      out.push(byId.get(id)!)
    } else {
      out.push({ id, address: "—", state: id === selfId ? "healthy" : "recovering" })
    }
    seen.add(id)
  }
  for (const m of membership) {
    if (!seen.has(m.id)) out.push(m)
  }
  if (out.length === 0) {
    out.push({ id: selfId, address: "—", state: "healthy" })
  }
  return out
}
