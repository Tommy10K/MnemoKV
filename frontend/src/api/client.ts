import { useAppStore } from "@/store/appStore"
import type {
  ClusterStateResponse,
  CommandResult,
  EngineStateResponse,
  HealthResponse,
  MetricsSummary,
  ControllerStateResponse,
} from "./types"
import {
  parseClusterStateResponse,
  parseCommandResult,
  parseEngineStateResponse,
  parseEvictionPolicyResponse,
  parseHealthResponse,
  parseMetricsSummary,
  parseControllerStateResponse,
} from "./validate"

function base(): string {
  return useAppStore.getState().apiBaseUrl
}

async function getJSON<T>(
  path: string,
  parse: (value: unknown) => T,
  signal?: AbortSignal,
): Promise<T> {
  const res = await fetch(base() + path, { signal })
  if (!res.ok) {
    throw new Error(`${path} returned ${res.status}`)
  }
  return parse(await res.json())
}

export function getHealth(signal?: AbortSignal) {
  return getJSON<HealthResponse>("/health", parseHealthResponse, signal)
}

export function getEngineState(signal?: AbortSignal) {
  return getJSON<EngineStateResponse>("/engine/state", parseEngineStateResponse, signal)
}

export function getMetricsSummary(signal?: AbortSignal) {
  return getJSON<MetricsSummary>("/metrics/summary", parseMetricsSummary, signal)
}

export function getClusterState(signal?: AbortSignal) {
  return getJSON<ClusterStateResponse>("/cluster/state", parseClusterStateResponse, signal)
}

export function getControllerState(signal?: AbortSignal) {
  return getJSON<ControllerStateResponse>("/controller/state", parseControllerStateResponse, signal)
}

export async function runCommand(args: string[]): Promise<CommandResult> {
  const res = await fetch(base() + "/commands", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ args }),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `commands returned ${res.status}`)
  }
  return parseCommandResult(await res.json())
}

export async function setEvictionPolicy(policy: string): Promise<{ policy: string }> {
  const res = await fetch(base() + "/engine/eviction-policy", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ policy }),
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `eviction-policy returned ${res.status}`)
  }
  return parseEvictionPolicyResponse(await res.json())
}
