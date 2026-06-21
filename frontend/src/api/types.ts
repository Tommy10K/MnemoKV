export type HealthResponse = {
  status: string
  nodeId: string
  mode: string
}

export type EngineStateResponse = {
  usedBytes: number
  memoryLimit: number
  availableBytes: number
  usageRatio: number
  evictionPolicy: string
  rejectedWrites: number
}

export type MetricsSummary = {
  counters: Record<string, number>
}

export type PeerStatus = {
  id: string
  address: string
  state: string
}

export type ClusterStateResponse = {
  enabled: boolean
  nodeId: string
  clusterId?: string
  slotCount?: number
  metadataVersion?: number
  routingMode?: string
  failoverMode?: string
  peers: string[]
  membership?: PeerStatus[]
  slots?: SlotStatus[]
}

export type SlotStatus = {
  number: number
  leaderId: string
  replicaId?: string
  localRole: "leader" | "replica" | "none"
  term: number
  lastSequence: number
  lastAppliedSequence: number
  replicaReady: boolean
}

export type NodeEvent = {
  timestamp: number
  usedBytes: number
  memoryLimit: number
  availableBytes: number
  policy: string
  rejectedWrites?: number
  counters?: Record<string, number>
}

export type CommandResult =
  | { type: "string"; value: string }
  | { type: "error"; value: string }
  | { type: "integer"; value: number }
  | { type: "bulk"; value: string }
  | { type: "nil"; value: null }
  | { type: "array"; value: CommandResult[] }
