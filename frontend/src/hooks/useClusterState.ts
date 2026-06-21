import { useEffect, useRef, useState } from "react"
import { getClusterState } from "@/api/client"
import type { ClusterStateResponse } from "@/api/types"
import { useAppStore } from "@/store/appStore"

export type MetadataChange = { at: number; version: number }

// Poll the reporting node and retain recent authoritative metadata-version
// changes so manual topology operations are visible in the UI.
export function useClusterState(intervalMs = 1500) {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const [state, setState] = useState<ClusterStateResponse | null>(null)
  const [reachable, setReachable] = useState(true)
  const [metadataHistory, setMetadataHistory] = useState<MetadataChange[]>([])
  const lastVersionRef = useRef<number | null>(null)

  useEffect(() => {
    let cancelled = false
    const ctrl = new AbortController()
    lastVersionRef.current = null
    let first = true

    async function poll() {
      try {
        const data = await getClusterState(ctrl.signal)
        if (cancelled) return
        if (first) {
          first = false
          setMetadataHistory([])
        }
        setReachable(true)
        setState(data)
        const version = data.metadataVersion ?? 0
        if (lastVersionRef.current === null) {
          lastVersionRef.current = version
          if (version > 0) setMetadataHistory([{ at: Date.now(), version }])
        } else if (version !== lastVersionRef.current) {
          lastVersionRef.current = version
          setMetadataHistory((prev) => [...prev, { at: Date.now(), version }].slice(-20))
        }
      } catch {
        if (!cancelled) setReachable(false)
      }
    }

    poll()
    const id = setInterval(poll, intervalMs)
    return () => {
      cancelled = true
      ctrl.abort()
      clearInterval(id)
    }
  }, [baseUrl, intervalMs])

  return { state, reachable, metadataHistory }
}
