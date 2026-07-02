import { useEffect, useState } from "react"
import { getEngineState, setEvictionPolicy } from "@/api/client"
import type { EngineStateResponse } from "@/api/types"
import { TimeSeriesChart } from "@/components/charts/TimeSeriesChart"
import { useNodeEvents } from "@/hooks/useNodeEvents"
import { useNodeStatus } from "@/hooks/useNodeStatus"
import { useAppStore } from "@/store/appStore"

const policies = [
  {
    id: "noeviction",
    label: "noeviction",
    note: "Never evicts. Rejects memory-growing writes that would exceed the hard limit.",
  },
  { id: "fifo", label: "FIFO", note: "Oldest insert wins the boot. Predictable and cheap." },
  { id: "lru", label: "LRU", note: "Least recently used. Strong for temporal locality." },
  { id: "lfu", label: "LFU", note: "Least frequently used. Strong for stable hot keys." },
  { id: "random", label: "Random", note: "Sample and drop. Surprisingly competitive at scale." },
]

type Switch = {
  at: number
  from: string
  to: string
}

export function EvictionLabPage() {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const { health, error: healthError } = useNodeStatus()
  const { status, latest, memory, error: eventsError } = useNodeEvents()
  const [engine, setEngine] = useState<EngineStateResponse | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [history, setHistory] = useState<Switch[]>([])

  useEffect(() => {
    let cancelled = false
    const ctrl = new AbortController()
    function refresh() {
      getEngineState(ctrl.signal)
        .then((data) => {
          if (!cancelled) setEngine(data)
        })
        .catch(() => {
          if (!cancelled) setEngine(null)
        })
    }
    refresh()
    const id = window.setInterval(refresh, 2000)
    return () => {
      cancelled = true
      ctrl.abort()
      window.clearInterval(id)
    }
  }, [baseUrl])

  const offline = health === null
  const current = latest?.policy ?? engine?.evictionPolicy ?? "—"
  const evictionCount = latest?.counters?.["eviction.count"] ?? 0
  const rejectedWrites = latest?.rejectedWrites ?? engine?.rejectedWrites ?? 0
  const usedBytes = latest?.usedBytes ?? engine?.usedBytes ?? 0
  const memoryLimit = latest?.memoryLimit ?? engine?.memoryLimit ?? 0
  const availableBytes = latest?.availableBytes ?? engine?.availableBytes ?? 0
  const usageRatio = memoryLimit > 0 ? usedBytes / memoryLimit : 0

  async function switchTo(id: string) {
    if (id === current || busy) return
    setBusy(id)
    setError(null)
    try {
      const res = await setEvictionPolicy(id)
      setHistory((h) =>
        [{ at: Date.now(), from: current, to: res.policy }, ...h].slice(0, 20),
      )
      setEngine((e) => (e ? { ...e, evictionPolicy: res.policy } : e))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Eviction lab</h1>
        <p className="text-sm text-[#9ca3af]">
          Swap the active eviction policy on a running node and watch how the store responds.
        </p>
      </header>

      {offline ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-4 text-sm text-amber-200">
          {healthError?.includes("unexpected response") ? (
            healthError
          ) : (
            <>Node at <span className="font-mono">{baseUrl}</span> is unreachable. Start a node or change the API target above.</>
          )}
        </div>
      ) : (
        <>
          {eventsError ? <p role="alert" className="rounded-md border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-200">{eventsError}</p> : null}
          <section className="grid gap-4 sm:grid-cols-3">
            <Card label="Current policy">
              <div className="font-mono text-2xl uppercase text-emerald-300">{current}</div>
              <div className="mt-1 text-xs text-[#9ca3af]">
                stream is {status === "connected" ? "live" : status}
              </div>
            </Card>
            <Card label="Memory used">
              <div className="text-2xl text-white">{formatBytes(usedBytes)}</div>
              <div className="mt-1 text-xs text-[#9ca3af]">
                {memoryLimit > 0
                  ? `${(usageRatio * 100).toFixed(1)}% of ${formatBytes(memoryLimit)}`
                  : "no memory limit configured"}
              </div>
            </Card>
            <Card label="Evicted so far">
              <div className="text-2xl text-white">{evictionCount.toLocaleString()}</div>
              <div className="mt-1 text-xs text-[#9ca3af]">since this node started</div>
            </Card>
          </section>

          <section className="grid gap-4 sm:grid-cols-2">
            <Card label={memoryLimit > 0 ? "Limit remaining" : "Configured limit"}>
              <div className="text-2xl text-white">
                {memoryLimit > 0 ? formatBytes(availableBytes) : "No hard limit"}
              </div>
              <div className="mt-1 text-xs text-[#9ca3af]">
                {memoryLimit > 0
                  ? "before the next memory-growing write must evict or fail"
                  : "memoryLimitBytes is 0, so eviction will not be triggered by max memory"}
              </div>
            </Card>
            <Card label="Rejected writes">
              <div className="text-2xl text-white">{rejectedWrites.toLocaleString()}</div>
              <div className="mt-1 text-xs text-[#9ca3af]">OOM-style hard-limit rejections</div>
            </Card>
          </section>

          <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
            <h2 className="text-sm font-semibold text-white">Pick a policy</h2>
            <p className="mt-1 text-xs text-[#9ca3af]">
              Switches happen instantly — the next time the engine needs to free space it uses
              the new policy.
            </p>
            <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {policies.map((p) => {
                const active = p.id === current
                return (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => switchTo(p.id)}
                    disabled={active || busy !== null}
                    aria-pressed={active}
                    className={[
                      "rounded-md border p-3 text-left transition-colors",
                      active
                        ? "border-emerald-500 bg-emerald-500/10"
                        : "border-[#1f2937] bg-[#0d1117] hover:border-emerald-500/40",
                      busy === p.id ? "opacity-60" : "",
                    ].join(" ")}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-sm uppercase text-white">{p.label}</span>
                      {active ? (
                        <span className="text-[10px] uppercase tracking-wide text-emerald-300">
                          active
                        </span>
                      ) : busy === p.id ? (
                        <span className="text-[10px] uppercase tracking-wide text-[#9ca3af]">
                          switching…
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-1 text-xs text-[#9ca3af]">{p.note}</p>
                  </button>
                )
              })}
            </div>
            {error ? <p className="mt-3 text-sm text-red-400">{error}</p> : null}
          </section>

          <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
            <h2 className="mb-3 text-sm font-semibold text-white">Memory over time</h2>
            <TimeSeriesChart ariaLabel="Memory usage while testing eviction policies" data={memory} dataKey="used" format={formatBytes} />
          </section>

          <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
            <h2 className="mb-3 text-sm font-semibold text-white">Switches this session</h2>
            {history.length === 0 ? (
              <p className="text-xs text-[#8b949e]">No switches yet — pick a policy above.</p>
            ) : (
              <ol className="space-y-1 text-xs">
                {history.map((s, i) => (
                  <li
                    key={i}
                    className="flex items-center gap-3 rounded border border-[#1f2937] bg-[#0d1117] px-2 py-1"
                  >
                    <span className="font-mono text-[#9ca3af]">
                      {new Date(s.at).toLocaleTimeString()}
                    </span>
                    <span className="font-mono uppercase text-[#8b949e]">{s.from}</span>
                    <span className="text-[#8b949e]">→</span>
                    <span className="font-mono uppercase text-emerald-300">{s.to}</span>
                  </li>
                ))}
              </ol>
            )}
          </section>

          <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4 text-xs text-[#9ca3af]">
            {memoryLimit > 0 ? (
              <p>
                Generate load with the Workloads tab to exercise the configured{" "}
                {formatBytes(memoryLimit)} hard limit. Memory-growing writes reserve space before
                they commit; with noeviction they fail instead of deleting existing keys.
              </p>
            ) : (
              <p className="text-amber-300">
                Eviction cannot occur because this node has no memory limit. Generate a config
                with a small positive limit, restart the node with -config, then return here.
              </p>
            )}
          </section>
        </>
      )}
    </div>
  )
}

function Card({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="text-xs uppercase tracking-wide text-[#8b949e]">{label}</div>
      <div className="mt-2">{children}</div>
    </div>
  )
}

function formatBytes(v: number): string {
  if (v < 1024) return `${v.toFixed(0)} B`
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`
  if (v < 1024 * 1024 * 1024) return `${(v / (1024 * 1024)).toFixed(1)} MB`
  return `${(v / (1024 * 1024 * 1024)).toFixed(2)} GB`
}
