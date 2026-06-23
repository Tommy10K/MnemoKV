import type {
  ClusterStateResponse,
  CommandResult,
  EngineStateResponse,
  HealthResponse,
  MetricsSummary,
  NodeEvent,
  PeerStatus,
  RecoveryPlanStatus,
  RecoverySlotStatus,
  RecoveryState,
  RecoveryStatus,
  SlotStatus,
  ControllerStateResponse,
} from "./types"

export class ResponseValidationError extends Error {
  constructor(source: string, detail: string) {
    super(`${source} returned an unexpected response: ${detail}`)
    this.name = "ResponseValidationError"
  }
}

type ObjectValue = Record<string, unknown>

export function parseHealthResponse(value: unknown): HealthResponse {
  const obj = object(value, "/health")
  return {
    status: string(obj.status, "/health", "status"),
    nodeId: string(obj.nodeId, "/health", "nodeId"),
    mode: string(obj.mode, "/health", "mode"),
    dataState: optionalString(obj.dataState, "/health", "dataState"),
  }
}

export function parseEngineStateResponse(value: unknown): EngineStateResponse {
  const obj = object(value, "/engine/state")
  return {
    usedBytes: finiteNumber(obj.usedBytes, "/engine/state", "usedBytes"),
    memoryLimit: finiteNumber(obj.memoryLimit, "/engine/state", "memoryLimit"),
    availableBytes: finiteNumber(obj.availableBytes, "/engine/state", "availableBytes"),
    usageRatio: finiteNumber(obj.usageRatio, "/engine/state", "usageRatio"),
    evictionPolicy: string(obj.evictionPolicy, "/engine/state", "evictionPolicy"),
    rejectedWrites: finiteNumber(obj.rejectedWrites, "/engine/state", "rejectedWrites"),
  }
}

export function parseMetricsSummary(value: unknown): MetricsSummary {
  const obj = object(value, "/metrics/summary")
  return { counters: numberRecord(obj.counters, "/metrics/summary", "counters") }
}

export function parseClusterStateResponse(value: unknown): ClusterStateResponse {
  const obj = object(value, "/cluster/state")
  return {
    enabled: boolean(obj.enabled, "/cluster/state", "enabled"),
    nodeId: string(obj.nodeId, "/cluster/state", "nodeId"),
    clusterId: optionalString(obj.clusterId, "/cluster/state", "clusterId"),
    slotCount: optionalNumber(obj.slotCount, "/cluster/state", "slotCount"),
    metadataVersion: optionalNumber(obj.metadataVersion, "/cluster/state", "metadataVersion"),
    routingMode: optionalString(obj.routingMode, "/cluster/state", "routingMode"),
    failoverMode: optionalString(obj.failoverMode, "/cluster/state", "failoverMode"),
    peers: arrayOrEmpty(obj.peers, "/cluster/state", "peers").map((peer, index) =>
      string(peer, "/cluster/state", `peers[${index}]`),
    ),
    membership: optionalArray(obj.membership, "/cluster/state", "membership")?.map(parsePeer),
    slots: optionalArray(obj.slots, "/cluster/state", "slots")?.map(parseSlot),
    dataState: optionalString(obj.dataState, "/cluster/state", "dataState"),
    recovery: obj.recovery === undefined ? undefined : parseRecoveryStatus(obj.recovery, "/cluster/state.recovery"),
  }
}

export function parseControllerStateResponse(value: unknown): ControllerStateResponse {
  const source = "/controller/state"
  const obj = object(value, source)
  const last = obj.lastRebalance === undefined ? undefined : object(obj.lastRebalance, source, "lastRebalance")
  const view = object(obj.currentView, source, "currentView")
  return {
    nodeId: string(obj.nodeId, source, "nodeId"),
    raftRole: string(obj.raftRole, source, "raftRole"),
    raftLeaderId: optionalString(obj.raftLeaderId, source, "raftLeaderId"),
    raftTerm: finiteNumber(obj.raftTerm, source, "raftTerm"),
    isLeader: boolean(obj.isLeader, source, "isLeader"),
    controlIndex: finiteNumber(obj.controlIndex, source, "controlIndex"),
    currentView: {
      clusterId: string(view.clusterId, source, "currentView.clusterId"),
      metadataVersion: finiteNumber(view.metadataVersion, source, "currentView.metadataVersion"),
      observedAt: optionalString(view.observedAt, source, "currentView.observedAt"),
      status: string(view.status, source, "currentView.status"),
      nodes: array(view.nodes, source, "currentView.nodes").map((item, index) => {
        const node = object(item, source, `currentView.nodes[${index}]`)
        return {
          id: string(node.id, source, `currentView.nodes[${index}].id`),
          reachable: boolean(node.reachable, source, `currentView.nodes[${index}].reachable`),
          suspected: boolean(node.suspected, source, `currentView.nodes[${index}].suspected`),
          eligible: boolean(node.eligible, source, `currentView.nodes[${index}].eligible`),
          returning: boolean(node.returning, source, `currentView.nodes[${index}].returning`),
          leaderSlots: finiteNumber(node.leaderSlots, source, `currentView.nodes[${index}].leaderSlots`),
          replicaSlots: finiteNumber(node.replicaSlots, source, `currentView.nodes[${index}].replicaSlots`),
        }
      }),
      slots: array(view.slots, source, "currentView.slots").map((item, index) => {
        const slot = object(item, source, `currentView.slots[${index}]`)
        return {
          number: finiteNumber(slot.number, source, `currentView.slots[${index}].number`),
          leaderId: string(slot.leaderId, source, `currentView.slots[${index}].leaderId`),
          replicaId: optionalString(slot.replicaId, source, `currentView.slots[${index}].replicaId`),
          term: finiteNumber(slot.term, source, `currentView.slots[${index}].term`),
          replicaReady: boolean(slot.replicaReady, source, `currentView.slots[${index}].replicaReady`),
        }
      }),
    },
    recovery: parseRecoveryStatus(obj.recovery, `${source}.recovery`),
    lastRebalance: last === undefined ? undefined : {
      id: string(last.id, source, "lastRebalance.id"),
      kind: string(last.kind, source, "lastRebalance.kind"),
      epoch: finiteNumber(last.epoch, source, "lastRebalance.epoch"),
      controlIndex: finiteNumber(last.controlIndex, source, "lastRebalance.controlIndex"),
    },
  }
}

export function parseCommandResult(value: unknown, source = "/commands"): CommandResult {
  const obj = object(value, source)
  const type = string(obj.type, source, "type")
  switch (type) {
    case "string":
    case "error":
    case "bulk":
      return { type, value: string(obj.value, source, "value") }
    case "integer":
      return { type, value: finiteNumber(obj.value, source, "value") }
    case "nil":
      if (obj.value !== undefined && obj.value !== null) fail(source, "value must be null")
      return { type, value: null }
    case "array":
      return {
        type,
        value: array(obj.value, source, "value").map((item, index) =>
          parseCommandResult(item, `${source}.value[${index}]`),
        ),
      }
    default:
      fail(source, `unsupported command result type ${JSON.stringify(type)}`)
  }
}

export function parseEvictionPolicyResponse(value: unknown): { policy: string } {
  const obj = object(value, "/engine/eviction-policy")
  return { policy: string(obj.policy, "/engine/eviction-policy", "policy") }
}

export function parseNodeEvent(value: unknown): NodeEvent {
  const source = "/events"
  const obj = object(value, source)
  return {
    timestamp: finiteNumber(obj.timestamp, source, "timestamp"),
    usedBytes: finiteNumber(obj.usedBytes, source, "usedBytes"),
    memoryLimit: finiteNumber(obj.memoryLimit, source, "memoryLimit"),
    availableBytes: finiteNumber(obj.availableBytes, source, "availableBytes"),
    policy: string(obj.policy, source, "policy"),
    rejectedWrites: optionalNumber(obj.rejectedWrites, source, "rejectedWrites"),
    counters:
      obj.counters === undefined ? undefined : numberRecord(obj.counters, source, "counters"),
    recovery: obj.recovery === undefined ? undefined : parseRecoveryStatus(obj.recovery, `${source}.recovery`),
  }
}

function parseRecoveryStatus(value: unknown, source: string): RecoveryStatus {
  const obj = object(value, source)
  const state = string(obj.state, source, "state")
  if (!isRecoveryState(state)) fail(source, `state ${JSON.stringify(state)} is invalid`)
  return {
    state,
    controlIndex: finiteNumber(obj.controlIndex, source, "controlIndex"),
    failedNodes: optionalStringArray(obj.failedNodes, source, "failedNodes"),
    suspectedNodes: optionalStringArray(obj.suspectedNodes, source, "suspectedNodes"),
    oneCopySlots: optionalArray(obj.oneCopySlots, source, "oneCopySlots")?.map((item, index) => parseRecoverySlot(item, source, "oneCopySlots", index)),
    unavailableSlots: optionalArray(obj.unavailableSlots, source, "unavailableSlots")?.map((item, index) => parseRecoverySlot(item, source, "unavailableSlots", index)),
    activePlan: obj.activePlan === undefined ? undefined : parseRecoveryPlan(obj.activePlan, source),
    latestCommittedOperation: optionalString(obj.latestCommittedOperation, source, "latestCommittedOperation"),
    warning: optionalString(obj.warning, source, "warning"),
    returningNodeDataPolicy: optionalString(obj.returningNodeDataPolicy, source, "returningNodeDataPolicy"),
  }
}

function parseRecoveryPlan(value: unknown, source: string): RecoveryPlanStatus {
  const obj = object(value, source, "activePlan")
  return {
    id: string(obj.id, source, "activePlan.id"),
    kind: string(obj.kind, source, "activePlan.kind"),
    reason: string(obj.reason, source, "activePlan.reason"),
    completedSteps: finiteNumber(obj.completedSteps, source, "activePlan.completedSteps"),
    totalSteps: finiteNumber(obj.totalSteps, source, "activePlan.totalSteps"),
  }
}

function parseRecoverySlot(value: unknown, source: string, field: string, index: number): RecoverySlotStatus {
  const prefix = `${field}[${index}]`
  const obj = object(value, source, prefix)
  return {
    slot: finiteNumber(obj.slot, source, `${prefix}.slot`),
    classification: string(obj.classification, source, `${prefix}.classification`),
    formerLeaderId: string(obj.formerLeaderId, source, `${prefix}.formerLeaderId`),
    formerReplicaId: optionalString(obj.formerReplicaId, source, `${prefix}.formerReplicaId`),
    failures: optionalStringArray(obj.failures, source, `${prefix}.failures`),
    readsAvailable: boolean(obj.readsAvailable, source, `${prefix}.readsAvailable`),
    writesAvailable: boolean(obj.writesAvailable, source, `${prefix}.writesAvailable`),
    rejectedCommands: optionalStringArray(obj.rejectedCommands, source, `${prefix}.rejectedCommands`),
    message: string(obj.message, source, `${prefix}.message`),
  }
}

function isRecoveryState(value: string): value is RecoveryState {
  return ["healthy", "failure_suspected", "degraded", "promoting", "repairing", "rebalancing", "unavailable", "potential_data_loss", "starting"].includes(value)
}

function optionalStringArray(value: unknown, source: string, field: string): string[] | undefined {
  return optionalArray(value, source, field)?.map((item, index) => string(item, source, `${field}[${index}]`))
}

function parsePeer(value: unknown, index: number): PeerStatus {
  const source = "/cluster/state"
  const obj = object(value, source, `membership[${index}]`)
  return {
    id: string(obj.id, source, `membership[${index}].id`),
    address: string(obj.address, source, `membership[${index}].address`),
    state: string(obj.state, source, `membership[${index}].state`),
  }
}

function parseSlot(value: unknown, index: number): SlotStatus {
  const source = "/cluster/state"
  const obj = object(value, source, `slots[${index}]`)
  const localRole = string(obj.localRole, source, `slots[${index}].localRole`)
  if (localRole !== "leader" && localRole !== "replica" && localRole !== "none") {
    fail(source, `slots[${index}].localRole is invalid`)
  }
  return {
    number: finiteNumber(obj.number, source, `slots[${index}].number`),
    leaderId: string(obj.leaderId, source, `slots[${index}].leaderId`),
    replicaId: optionalString(obj.replicaId, source, `slots[${index}].replicaId`),
    localRole,
    term: finiteNumber(obj.term, source, `slots[${index}].term`),
    lastSequence: finiteNumber(obj.lastSequence, source, `slots[${index}].lastSequence`),
    lastAppliedSequence: finiteNumber(
      obj.lastAppliedSequence,
      source,
      `slots[${index}].lastAppliedSequence`,
    ),
    replicaReady: boolean(obj.replicaReady, source, `slots[${index}].replicaReady`),
  }
}

function object(value: unknown, source: string, field = "body"): ObjectValue {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    fail(source, `${field} must be an object`)
  }
  return value as ObjectValue
}

function array(value: unknown, source: string, field: string): unknown[] {
  if (!Array.isArray(value)) fail(source, `${field} must be an array`)
  return value
}

function optionalArray(value: unknown, source: string, field: string): unknown[] | undefined {
  return value === undefined || value === null ? undefined : array(value, source, field)
}

function arrayOrEmpty(value: unknown, source: string, field: string): unknown[] {
  return value === undefined || value === null ? [] : array(value, source, field)
}

function string(value: unknown, source: string, field: string): string {
  if (typeof value !== "string") fail(source, `${field} must be a string`)
  return value
}

function optionalString(value: unknown, source: string, field: string): string | undefined {
  return value === undefined ? undefined : string(value, source, field)
}

function boolean(value: unknown, source: string, field: string): boolean {
  if (typeof value !== "boolean") fail(source, `${field} must be a boolean`)
  return value
}

function finiteNumber(value: unknown, source: string, field: string): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    fail(source, `${field} must be a finite number`)
  }
  return value
}

function optionalNumber(value: unknown, source: string, field: string): number | undefined {
  return value === undefined ? undefined : finiteNumber(value, source, field)
}

function numberRecord(value: unknown, source: string, field: string): Record<string, number> {
  const obj = object(value, source, field)
  const result: Record<string, number> = {}
  for (const [key, item] of Object.entries(obj)) {
    result[key] = finiteNumber(item, source, `${field}.${key}`)
  }
  return result
}

function fail(source: string, detail: string): never {
  throw new ResponseValidationError(source, detail)
}
