import { useEffect, useRef, useState } from "react"
import { connectEvents } from "@/api/events"
import type { NodeEvent } from "@/api/types"
import { useAppStore } from "@/store/appStore"

export type ConnectionStatus = "connecting" | "connected" | "disconnected" | "stale"

export type MemoryPoint = { t: number; used: number; ratio: number }
export type ThroughputPoint = { t: number; ops: number }

const MAX_POINTS = 60
const STALE_AFTER_MS = 3000

// useNodeEvents subscribes to the backend SSE stream and exposes the latest
// event, a rolling 60-point window of memory and throughput samples, and
// the current connection status. Throughput is computed from cmd.total
// deltas between consecutive events.
export function useNodeEvents() {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const [status, setStatus] = useState<ConnectionStatus>("connecting")
  const [latest, setLatest] = useState<NodeEvent | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [memory, setMemory] = useState<MemoryPoint[]>([])
  const [throughput, setThroughput] = useState<ThroughputPoint[]>([])
  const prevRef = useRef<{ ts: number; total: number } | null>(null)

  useEffect(() => {
    let staleTimer: ReturnType<typeof setTimeout> | null = null
    prevRef.current = null

    const resetStaleTimer = () => {
      if (staleTimer) clearTimeout(staleTimer)
      staleTimer = setTimeout(() => setStatus("stale"), STALE_AFTER_MS)
    }

    const close = connectEvents(baseUrl, {
      onOpen: () => {
        setStatus("connected")
        setMemory([])
        setThroughput([])
        setLatest(null)
        setError(null)
        resetStaleTimer()
      },
      onError: () => {
        setStatus("disconnected")
      },
      onInvalid: (cause) => {
        setStatus("stale")
        setError(cause.message)
      },
      onEvent: (event) => {
        setStatus("connected")
        setError(null)
        resetStaleTimer()
        setLatest(event)

        const ratio = event.memoryLimit > 0 ? event.usedBytes / event.memoryLimit : 0
        setMemory((prev) =>
          appendPoint(prev, { t: event.timestamp, used: event.usedBytes, ratio }),
        )

        const total = event.counters?.["cmd.total"] ?? 0
        const prev = prevRef.current
        if (prev) {
          const dt = event.timestamp - prev.ts
          const ops = dt > 0 ? Math.max(0, (total - prev.total) / dt) : 0
          setThroughput((points) => appendPoint(points, { t: event.timestamp, ops }))
        }
        prevRef.current = { ts: event.timestamp, total }
      },
    })

    return () => {
      if (staleTimer) clearTimeout(staleTimer)
      close()
    }
  }, [baseUrl])

  return { status, latest, memory, throughput, error }
}

function appendPoint<T>(points: T[], next: T): T[] {
  const out = points.length >= MAX_POINTS ? points.slice(1) : points.slice()
  out.push(next)
  return out
}
