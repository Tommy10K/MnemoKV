import { useEffect, useRef, useState } from "react"
import { getClusterState } from "@/api/client"
import type { ClusterStateResponse } from "@/api/types"
import { useAppStore } from "@/store/appStore"

export type TermChange = { at: number; term: number }

// useClusterState polls /cluster/state on an interval and tracks the history
// of term changes locally so the page can show a failover timeline without
// any server-side log.
export function useClusterState(intervalMs = 1500) {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const [state, setState] = useState<ClusterStateResponse | null>(null)
  const [reachable, setReachable] = useState(true)
  const [termHistory, setTermHistory] = useState<TermChange[]>([])
  const lastTermRef = useRef<number | null>(null)

  useEffect(() => {
    let cancelled = false
    const ctrl = new AbortController()
    lastTermRef.current = null
    let first = true

    async function poll() {
      try {
        const data = await getClusterState(ctrl.signal)
        if (cancelled) return
        if (first) {
          first = false
          setTermHistory([])
        }
        setReachable(true)
        setState(data)
        const term = data.term ?? 0
        if (lastTermRef.current === null) {
          lastTermRef.current = term
          if (term > 0) {
            setTermHistory([{ at: Date.now(), term }])
          }
        } else if (term !== lastTermRef.current) {
          lastTermRef.current = term
          setTermHistory((prev) => [...prev, { at: Date.now(), term }].slice(-20))
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

  return { state, reachable, termHistory }
}
