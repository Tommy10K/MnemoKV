import { useAppStore } from "@/store/appStore"
import type {
  ClusterStateResponse,
  CommandResult,
  EngineStateResponse,
  HealthResponse,
  MetricsSummary,
} from "./types"

function base(): string {
  return useAppStore.getState().apiBaseUrl
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(base() + path, { signal })
  if (!res.ok) {
    throw new Error(`${path} returned ${res.status}`)
  }
  return (await res.json()) as T
}

export function getHealth(signal?: AbortSignal) {
  return getJSON<HealthResponse>("/health", signal)
}

export function getEngineState(signal?: AbortSignal) {
  return getJSON<EngineStateResponse>("/engine/state", signal)
}

export function getMetricsSummary(signal?: AbortSignal) {
  return getJSON<MetricsSummary>("/metrics/summary", signal)
}

export function getClusterState(signal?: AbortSignal) {
  return getJSON<ClusterStateResponse>("/cluster/state", signal)
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
  return (await res.json()) as CommandResult
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
  return (await res.json()) as { policy: string }
}
