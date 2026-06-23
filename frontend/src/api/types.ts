export type HealthResponse = {
  status: string
  nodeId: string
  mode: string
  dataState?: string
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
  dataState?: string
  recovery?: RecoveryStatus
}

export type RecoveryState =
  | "healthy"
  | "failure_suspected"
  | "degraded"
  | "promoting"
  | "repairing"
  | "rebalancing"
  | "unavailable"
  | "potential_data_loss"
  | "starting"

export type RecoveryPlanStatus = {
  id: string
  kind: string
  reason: string
  completedSteps: number
  totalSteps: number
}

export type RecoverySlotStatus = {
  slot: number
  classification: string
  formerLeaderId: string
  formerReplicaId?: string
  failures?: string[]
  readsAvailable: boolean
  writesAvailable: boolean
  rejectedCommands?: string[]
  message: string
}

export type RecoveryStatus = {
  state: RecoveryState
  controlIndex: number
  failedNodes?: string[]
  suspectedNodes?: string[]
  oneCopySlots?: RecoverySlotStatus[]
  unavailableSlots?: RecoverySlotStatus[]
  activePlan?: RecoveryPlanStatus
  latestCommittedOperation?: string
  warning?: string
  returningNodeDataPolicy?: string
}

export type ControllerStateResponse = {
  nodeId: string
  raftRole: string
  raftLeaderId?: string
  raftTerm: number
  isLeader: boolean
  controlIndex: number
  currentView: {
    clusterId: string
    metadataVersion: number
    observedAt?: string
    status: string
    nodes: Array<{ id: string; reachable: boolean; suspected: boolean; eligible: boolean; returning: boolean; leaderSlots: number; replicaSlots: number }>
    slots: Array<{ number: number; leaderId: string; replicaId?: string; term: number; replicaReady: boolean }>
  }
  recovery: RecoveryStatus
  lastRebalance?: { id: string; kind: string; epoch: number; controlIndex: number }
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
  recovery?: RecoveryStatus
}

export type CommandResult =
  | { type: "string"; value: string }
  | { type: "error"; value: string }
  | { type: "integer"; value: number }
  | { type: "bulk"; value: string }
  | { type: "nil"; value: null }
  | { type: "array"; value: CommandResult[] }
