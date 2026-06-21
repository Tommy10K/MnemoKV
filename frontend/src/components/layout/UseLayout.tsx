import { useState } from "react"
import { NavLink, Outlet } from "react-router-dom"
import { useNodeStatus } from "@/hooks/useNodeStatus"
import { useAppStore } from "@/store/appStore"

const tabs = [
  { to: "/use", label: "Configure", end: true },
  { to: "/use/dashboard", label: "Dashboard" },
  { to: "/use/console", label: "Console" },
  { to: "/use/workloads", label: "Workloads" },
  { to: "/use/cluster", label: "Cluster" },
  { to: "/use/eviction", label: "Eviction Lab" },
  { to: "/use/benchmarks", label: "Benchmarks" },
]

export function UseLayout() {
  const apiBaseUrl = useAppStore((state) => state.apiBaseUrl)
  const setApiBaseUrl = useAppStore((state) => state.setApiBaseUrl)
  const { health, error: healthError } = useNodeStatus()

  return (
    <div className="flex flex-col gap-6">
      <section className="flex flex-col gap-3 rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div className="text-xs uppercase tracking-wide text-[#8b949e]">API target</div>
          <div className="mt-1 text-sm text-[#9ca3af]">
            {health ? (
              <>
                Connected to <span className="font-mono text-white">{health.nodeId}</span>{" "}
                <span className="text-emerald-300">({health.mode})</span>
              </>
            ) : (
              <span className="text-amber-300">
                {healthError?.includes("unexpected response")
                  ? healthError
                  : "No node is responding at the selected address."}
              </span>
            )}
          </div>
        </div>
        <ApiTargetForm key={apiBaseUrl} value={apiBaseUrl} onApply={setApiBaseUrl} />
      </section>

      <nav aria-label="Database tools" className="flex flex-wrap gap-1 border-b border-[#1f2937] pb-2">
        {tabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              [
                "rounded-md px-3 py-1.5 text-sm transition-colors",
                isActive
                  ? "bg-[#1f2937] text-white"
                  : "text-[#9ca3af] hover:bg-[#161b22] hover:text-white",
              ].join(" ")
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </nav>

      <Outlet />
    </div>
  )
}

function ApiTargetForm({ value, onApply }: { value: string; onApply: (url: string) => void }) {
  const [draftUrl, setDraftUrl] = useState(value)
  const [error, setError] = useState<string | null>(null)

  function applyTarget() {
    try {
      const parsed = new URL(draftUrl)
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        throw new Error("unsupported protocol")
      }
      onApply(parsed.toString())
      setError(null)
    } catch {
      setError("Enter a complete HTTP URL, for example http://127.0.0.1:7380")
    }
  }

  return (
    <form
      className="flex w-full max-w-xl flex-col gap-2"
      onSubmit={(event) => {
        event.preventDefault()
        applyTarget()
      }}
    >
      <div className="flex flex-col gap-2 sm:flex-row">
        <label className="sr-only" htmlFor="api-base-url">
          API base URL
        </label>
        <input
          id="api-base-url"
          type="url"
          value={draftUrl}
          onChange={(event) => setDraftUrl(event.target.value)}
          aria-invalid={error !== null}
          aria-describedby={error ? "api-base-url-error" : undefined}
          className="min-w-0 flex-1 rounded-md border border-[#1f2937] bg-[#0d1117] px-3 py-1.5 font-mono text-sm text-white outline-none focus:border-emerald-500/60"
        />
        <button
          type="submit"
          className="rounded-md bg-emerald-500/90 px-4 py-1.5 text-sm font-medium text-black hover:bg-emerald-400"
        >
          Connect
        </button>
      </div>
      {error ? <p id="api-base-url-error" role="alert" className="text-xs text-red-400">{error}</p> : null}
    </form>
  )
}
