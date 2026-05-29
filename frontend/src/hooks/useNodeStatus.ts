import { useEffect, useState } from "react"
import { getHealth } from "@/api/client"
import type { HealthResponse } from "@/api/types"
import { useAppStore } from "@/store/appStore"

// useNodeStatus polls /health every few seconds. It returns null while
// the backend is unreachable so the UI can show a clear offline state
// instead of stale info.
export function useNodeStatus(intervalMs = 5000) {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const [health, setHealth] = useState<HealthResponse | null>(null)

  useEffect(() => {
    let cancelled = false
    const controller = new AbortController()

    async function poll() {
      try {
        const data = await getHealth(controller.signal)
        if (!cancelled) setHealth(data)
      } catch {
        if (!cancelled) setHealth(null)
      }
    }

    poll()
    const id = setInterval(poll, intervalMs)
    return () => {
      cancelled = true
      controller.abort()
      clearInterval(id)
    }
  }, [baseUrl, intervalMs])

  return health
}
