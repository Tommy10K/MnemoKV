import { useMemo } from "react"
import {
  Background,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import type { PeerStatus } from "@/api/types"

type Props = {
  peers: PeerStatus[]
  selfId: string
}

const stateStyle: Record<string, { bg: string; border: string }> = {
  healthy: { bg: "#064e3b", border: "#10b981" },
  recovering: { bg: "#1f3a5f", border: "#60a5fa" },
  suspect: { bg: "#3f2d09", border: "#f59e0b" },
  unavailable: { bg: "#3f1d1d", border: "#ef4444" },
  unknown: { bg: "#1f2937", border: "#8b949e" },
}

export function TopologyGraph({ peers, selfId }: Props) {
  const { nodes, edges } = useMemo(() => buildGraph(peers, selfId), [peers, selfId])

  return (
    <div role="img" aria-label={`Cluster topology with ${peers.length} nodes`} className="h-[320px] rounded-lg border border-[#1f2937] bg-[#0b0f17] sm:h-[420px]">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        fitView
        proOptions={{ hideAttribution: true }}
        colorMode="dark"
      >
        <Background color="#1f2937" gap={20} />
      </ReactFlow>
      <span className="sr-only">
        {peers.map((peer) => `${peer.id}${peer.id === selfId ? " (self)" : ""}: ${peer.state} at ${peer.address}`).join("; ")}
      </span>
    </div>
  )
}

function buildGraph(peers: PeerStatus[], selfId: string): { nodes: Node[]; edges: Edge[] } {
  if (peers.length === 0) return { nodes: [], edges: [] }

  const radius = peers.length === 1 ? 0 : 180
  const cx = 250
  const cy = 180

  const nodes: Node[] = peers.map((p, i) => {
    const angle = (i / peers.length) * Math.PI * 2 - Math.PI / 2
    const x = cx + radius * Math.cos(angle) - 70
    const y = cy + radius * Math.sin(angle) - 30
    const style = stateStyle[p.state] ?? { bg: "#1f2937", border: "#374151" }
    const isSelf = p.id === selfId
    return {
      id: p.id,
      position: { x, y },
      data: {
        label: (
          <div className="flex flex-col items-start gap-0.5 px-1 py-0.5 text-left">
            <div className="font-mono text-sm text-white">
              {p.id}
              {isSelf ? <span className="ml-1 text-emerald-300">(self)</span> : null}
            </div>
            <div className="font-mono text-[10px] text-[#9ca3af]">{p.address}</div>
            <div className="text-[10px] uppercase tracking-wide text-[#d1d5db]">{p.state}</div>
          </div>
        ),
      },
      style: {
        background: style.bg,
        border: `1.5px solid ${style.border}`,
        borderRadius: 8,
        padding: 6,
        width: 140,
        color: "#e6edf3",
      },
    }
  })

  const edges: Edge[] = []
  for (let i = 0; i < peers.length; i++) {
    for (let j = i + 1; j < peers.length; j++) {
      const a = peers[i]
      const b = peers[j]
      const stale =
        a.state === "unavailable" ||
        b.state === "unavailable" ||
        a.state === "unknown" ||
        b.state === "unknown"
      edges.push({
        id: `${a.id}-${b.id}`,
        source: a.id,
        target: b.id,
        style: { stroke: stale ? "#4b5563" : "#374151", strokeDasharray: stale ? "4 4" : undefined },
      })
    }
  }
  return { nodes, edges }
}
