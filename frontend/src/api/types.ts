export type HealthResponse = {
  status: string
  nodeId: string
  mode: string
}

export type EngineStateResponse = {
  usedBytes: number
  memoryLimit: number
  usageRatio: number
  evictionPolicy: string
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
  writeMode: string
  autoFailover: boolean
  term?: number
  peers: string[]
  membership?: PeerStatus[]
}

export type NodeEvent = {
  timestamp: number
  usedBytes: number
  memoryLimit: number
  policy: string
  counters?: Record<string, number>
}

export type CommandResult =
  | { type: "string"; value: string }
  | { type: "error"; value: string }
  | { type: "integer"; value: number }
  | { type: "bulk"; value: string }
  | { type: "nil"; value: null }
  | { type: "array"; value: CommandResult[] }
