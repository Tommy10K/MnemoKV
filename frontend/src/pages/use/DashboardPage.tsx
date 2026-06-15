import { useEffect, useState } from "react"
import { getEngineState } from "@/api/client"
import type { EngineStateResponse } from "@/api/types"
import { TimeSeriesChart } from "@/components/charts/TimeSeriesChart"
import { useNodeEvents } from "@/hooks/useNodeEvents"
import { useNodeStatus } from "@/hooks/useNodeStatus"
import { useAppStore } from "@/store/appStore"

export function DashboardPage() {
  const baseUrl = useAppStore((s) => s.apiBaseUrl)
  const health = useNodeStatus()
  const { status, latest, memory, throughput } = useNodeEvents()
  const [engine, setEngine] = useState<EngineStateResponse | null>(null)

  useEffect(() => {
    let cancelled = false
    const ctrl = new AbortController()
    getEngineState(ctrl.signal)
      .then((data) => {
        if (!cancelled) setEngine(data)
      })
      .catch(() => {
        if (!cancelled) setEngine(null)
      })
    return () => {
      cancelled = true
      ctrl.abort()
    }
  }, [baseUrl, latest?.timestamp])

  const offline = health === null
  const evictionCount = latest?.counters?.["eviction.count"] ?? 0
  const cmdTotal = latest?.counters?.["cmd.total"] ?? 0
  const opsPerSec = throughput.length ? throughput[throughput.length - 1].ops : 0

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-white">Dashboard</h1>
          <p className="text-sm text-[#9ca3af]">
            Live view of the node at <span className="font-mono">{baseUrl}</span>
          </p>
        </div>
        <StatusBadge status={offline ? "disconnected" : status} />
      </header>

      {offline ? (
        <OfflineNotice baseUrl={baseUrl} />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard label="Node">
              <div className="font-mono text-lg text-white">{health.nodeId}</div>
              <div className="text-xs text-[#9ca3af]">{health.mode}</div>
            </StatCard>

            <StatCard label="Memory">
              <div className="text-lg text-white">
                {formatBytes(latest?.usedBytes ?? engine?.usedBytes ?? 0)}
              </div>
              <div className="text-xs text-[#9ca3af]">
                {(latest?.memoryLimit ?? engine?.memoryLimit ?? 0) > 0
                  ? `of ${formatBytes(latest?.memoryLimit ?? engine?.memoryLimit ?? 0)} (${(
                      ((latest?.usedBytes ?? 0) /
                        (latest?.memoryLimit ?? engine?.memoryLimit ?? 1)) *
                      100
                    ).toFixed(1)}%)`
                  : "no limit configured"}
              </div>
            </StatCard>

            <StatCard label="Throughput">
              <div className="text-lg text-white">{opsPerSec.toFixed(1)} ops/s</div>
              <div className="text-xs text-[#9ca3af]">
                {cmdTotal.toLocaleString()} commands total
              </div>
            </StatCard>

            <StatCard label="Eviction">
              <div className="text-lg text-white">
                {latest?.policy ?? engine?.evictionPolicy ?? "—"}
              </div>
              <div className="text-xs text-[#9ca3af]">
                {evictionCount.toLocaleString()} evicted
              </div>
            </StatCard>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <ChartCard title="Memory used">
              <TimeSeriesChart data={memory} dataKey="used" format={formatBytes} />
            </ChartCard>
            <ChartCard title="Commands per second">
              <TimeSeriesChart
                data={throughput}
                dataKey="ops"
                color="#60a5fa"
                format={(v) => v.toFixed(1)}
              />
            </ChartCard>
          </div>
        </>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: "connecting" | "connected" | "disconnected" | "stale" }) {
  const map: Record<typeof status, { label: string; dot: string; text: string }> = {
    connecting: { label: "connecting", dot: "bg-yellow-400", text: "text-yellow-300" },
    connected: { label: "live", dot: "bg-emerald-400", text: "text-emerald-300" },
    stale: { label: "stale", dot: "bg-yellow-400", text: "text-yellow-300" },
    disconnected: { label: "offline", dot: "bg-red-500", text: "text-red-300" },
  }
  const m = map[status]
  return (
    <div className={`flex items-center gap-2 text-sm ${m.text}`}>
      <span className={`h-2 w-2 rounded-full ${m.dot}`} />
      {m.label}
    </div>
  )
}

function StatCard({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="text-xs uppercase tracking-wide text-[#6b7280]">{label}</div>
      <div className="mt-1">{children}</div>
    </div>
  )
}

function ChartCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="mb-2 text-sm text-[#9ca3af]">{title}</div>
      {children}
    </div>
  )
}

function OfflineNotice({ baseUrl }: { baseUrl: string }) {
  return (
    <div className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-6 text-sm text-[#9ca3af]">
      <p className="text-white">Backend not reachable at {baseUrl}.</p>
      <p className="mt-2">Start a node and try again, for example:</p>
      <pre className="mt-2 overflow-x-auto rounded bg-[#0d1117] p-3 font-mono text-xs text-[#e6edf3]">
        go run ./cmd/node -config configs/standalone.yaml
      </pre>
    </div>
  )
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ["KB", "MB", "GB", "TB"]
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`
}
