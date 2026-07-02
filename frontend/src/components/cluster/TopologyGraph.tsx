import { useMemo } from "react"
import {
  Background,
  Handle,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
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

type ClusterNodeData = {
  peer: PeerStatus
  isSelf: boolean
  bg: string
  border: string
}

const nodeTypes = { clusterNode: ClusterNode }

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
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.12 }}
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

function ClusterNode({ data }: NodeProps<Node<ClusterNodeData>>) {
  const { peer, isSelf, bg, border } = data
  return (
    <div
      className="relative w-[140px] rounded-md px-3 py-2 text-left shadow-sm"
      style={{ background: bg, border: `1.5px solid ${border}`, color: "#e6edf3" }}
    >
      <Handle type="target" position={Position.Top} className="!opacity-0" style={centerHandleStyle} />
      <Handle type="source" position={Position.Bottom} className="!opacity-0" style={centerHandleStyle} />
      <div className="font-mono text-sm text-white">
        {peer.id}
        {isSelf ? <span className="ml-1 text-emerald-300">(self)</span> : null}
      </div>
      <div className="mt-1 font-mono text-[10px] text-[#9ca3af]">{peer.address}</div>
      <div className="mt-1 text-[10px] uppercase tracking-wide text-[#d1d5db]">{peer.state}</div>
    </div>
  )
}

const centerHandleStyle = {
  left: "50%",
  top: "50%",
  transform: "translate(-50%, -50%)",
  pointerEvents: "none",
  width: 1,
  height: 1,
  border: "none",
} as const

function buildGraph(peers: PeerStatus[], selfId: string): { nodes: Node[]; edges: Edge[] } {
  if (peers.length === 0) return { nodes: [], edges: [] }

  const radiusX = peers.length === 1 ? 0 : 235
  const radiusY = peers.length === 1 ? 0 : 165
  const cx = 320
  const cy = 210

  const nodes: Node[] = peers.map((p, i) => {
    const angle = (i / peers.length) * Math.PI * 2 - Math.PI / 2
    const x = cx + radiusX * Math.cos(angle) - 70
    const y = cy + radiusY * Math.sin(angle) - 38
    const style = stateStyle[p.state] ?? { bg: "#1f2937", border: "#374151" }
    const isSelf = p.id === selfId
    return {
      id: p.id,
      type: "clusterNode",
      position: { x, y },
      data: {
        peer: p,
        isSelf,
        bg: style.bg,
        border: style.border,
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
        type: "straight",
        focusable: false,
        interactionWidth: 4,
        style: {
          stroke: stale ? "#4b5563" : "#6b7280",
          strokeWidth: stale ? 1 : isOuterRingEdge(i, j, peers.length) ? 2.5 : 1.1,
          strokeDasharray: stale ? "4 4" : undefined,
          opacity: stale ? 0.45 : isOuterRingEdge(i, j, peers.length) ? 0.85 : 0.5,
        },
      })
    }
  }
  return { nodes, edges }
}

function isOuterRingEdge(left: number, right: number, count: number): boolean {
  return right === left + 1 || (left === 0 && right === count - 1)
}
