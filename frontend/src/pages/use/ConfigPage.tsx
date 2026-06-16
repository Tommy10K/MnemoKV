import { useMemo, useState } from "react"
import { Link } from "react-router-dom"
import {
  configToYaml,
  defaultClusterPeers,
  defaultStandalone,
  validate,
  type EvictionPolicy,
  type Mode,
  type NodeConfig,
  type Peer,
  type WriteSafety,
} from "@/lib/config"
import { downloadFile } from "@/lib/download"
import { useAppStore } from "@/store/appStore"

const evictionPolicies: EvictionPolicy[] = ["noeviction", "fifo", "lru", "lfu", "random"]
const writeModes: WriteSafety[] = ["async", "strong"]

export function ConfigPage() {
  const setApiBaseUrl = useAppStore((state) => state.setApiBaseUrl)
  const [config, setConfig] = useState<NodeConfig>(defaultStandalone)
  const errors = useMemo(() => validate(config), [config])
  const yaml = useMemo(() => configToYaml(config), [config])

  function update<K extends keyof NodeConfig>(key: K, value: NodeConfig[K]) {
    setConfig((c) => ({ ...c, [key]: value }))
  }

  function switchMode(mode: Mode) {
    if (mode === "clustered" && config.peers.length === 0) {
      setConfig((c) => ({
        ...c,
        mode,
        shardingEnabled: true,
        replicationEnabled: true,
        autoFailover: false,
        peers: defaultClusterPeers(3),
      }))
      return
    }
    update("mode", mode)
  }

  function updatePeer(index: number, patch: Partial<Peer>) {
    setConfig((c) => ({
      ...c,
      peers: c.peers.map((p, i) => (i === index ? { ...p, ...patch } : p)),
    }))
  }

  function addPeer() {
    setConfig((c) => {
      const nextIndex = c.peers.length + 1
      return {
        ...c,
        peers: [...c.peers, { id: `node-${nextIndex}`, address: `127.0.0.1:${6380 + nextIndex}` }],
      }
    })
  }

  function removePeer(index: number) {
    setConfig((c) => ({ ...c, peers: c.peers.filter((_, i) => i !== index) }))
  }

  function reset() {
    setConfig(defaultStandalone())
  }

  function download() {
    downloadFile(`${config.id}.yaml`, yaml, "text/yaml")
  }

  const errorByField = new Map(errors.map((e) => [e.field, e.message]))

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_minmax(0,440px)]">
      <form className="flex flex-col gap-6" onSubmit={(e) => e.preventDefault()}>
        <Section title="Node">
          <Field label="Node id" error={errorByField.get("id")}>
            <TextInput value={config.id} onChange={(v) => update("id", v)} />
          </Field>
          <Field label="Mode">
            <Segmented
              options={["standalone", "clustered"]}
              value={config.mode}
              onChange={(v) => switchMode(v as Mode)}
            />
          </Field>
          <Field label="Data directory">
            <TextInput value={config.dataDir} onChange={(v) => update("dataDir", v)} />
          </Field>
        </Section>

        <Section title="Network">
          <Field label="Bind address">
            <TextInput value={config.bindAddr} onChange={(v) => update("bindAddr", v)} />
          </Field>
          <Field label="RESP port" error={errorByField.get("port")}>
            <NumberInput value={config.port} onChange={(v) => update("port", v)} />
          </Field>
          <Field label="API bind address">
            <TextInput value={config.apiBindAddr} onChange={(v) => update("apiBindAddr", v)} />
          </Field>
          <Field label="API port" error={errorByField.get("apiPort")}>
            <NumberInput value={config.apiPort} onChange={(v) => update("apiPort", v)} />
          </Field>
        </Section>

        <Section title="Engine">
          <Field label="Stripe count" error={errorByField.get("stripeCount")}>
            <NumberInput value={config.stripeCount} onChange={(v) => update("stripeCount", v)} />
          </Field>
          <Field
            label="Memory limit (bytes)"
            hint="0 disables eviction"
            error={errorByField.get("memoryLimitBytes")}
          >
            <NumberInput
              value={config.memoryLimitBytes}
              onChange={(v) => update("memoryLimitBytes", v)}
            />
          </Field>
          <Field label="Eviction policy">
            <Segmented
              options={evictionPolicies}
              value={config.evictionPolicy}
              onChange={(v) => update("evictionPolicy", v as EvictionPolicy)}
            />
          </Field>
        </Section>

        {config.mode === "clustered" && (
          <Section title="Cluster">
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-200">
              Cluster mode is an experimental prototype. Live commands are not routed by key
              ownership, replication is fan-out rather than consensus, and automatic failover
              does not provide a one-leader safety guarantee.
            </div>
            <Field label="Sharding enabled">
              <Toggle
                checked={config.shardingEnabled}
                onChange={(v) => update("shardingEnabled", v)}
              />
            </Field>
            <Field label="Replication enabled">
              <Toggle
                checked={config.replicationEnabled}
                onChange={(v) => update("replicationEnabled", v)}
              />
            </Field>
            <Field label="Auto-failover" error={errorByField.get("autoFailover")}>
              <Toggle
                checked={config.autoFailover}
                onChange={(v) => update("autoFailover", v)}
              />
            </Field>
            <Field label="Write safety">
              <Segmented
                options={writeModes}
                value={config.writeSafetyMode}
                onChange={(v) => update("writeSafetyMode", v as WriteSafety)}
              />
            </Field>
            <p className="text-xs text-[#9ca3af]">
              The backend setting named <span className="font-mono">strong</span> currently means
              synchronous fan-out to configured peers. It is not a quorum or durable commit mode.
            </p>

            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium text-[#e6edf3]">Peers</label>
                <button
                  type="button"
                  onClick={addPeer}
                  className="rounded-md border border-[#1f2937] px-2 py-1 text-xs text-[#9ca3af] hover:border-emerald-500/40 hover:text-white"
                >
                  + Add peer
                </button>
              </div>
              {errorByField.get("peers") && (
                <p className="text-xs text-red-400">{errorByField.get("peers")}</p>
              )}
              <div className="flex flex-col gap-2">
                {config.peers.map((peer, i) => (
                  <div key={i} className="flex gap-2">
                    <TextInput
                      value={peer.id}
                      onChange={(v) => updatePeer(i, { id: v })}
                      placeholder="id"
                    />
                    <TextInput
                      value={peer.address}
                      onChange={(v) => updatePeer(i, { address: v })}
                      placeholder="host:port"
                    />
                    <button
                      type="button"
                      onClick={() => removePeer(i)}
                      className="shrink-0 rounded-md border border-[#1f2937] px-2 text-sm text-[#9ca3af] hover:border-red-500/40 hover:text-red-400"
                      title="Remove"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </Section>
        )}
      </form>

      <aside className="flex flex-col gap-3 lg:sticky lg:top-6 lg:self-start">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-[#6b7280]">
            {config.id}.yaml
          </h2>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={reset}
              className="rounded-md border border-[#1f2937] px-3 py-1 text-xs text-[#9ca3af] hover:border-[#374151] hover:text-white"
            >
              Reset
            </button>
            <button
              type="button"
              onClick={download}
              disabled={errors.length > 0}
              className="rounded-md bg-emerald-500/90 px-3 py-1 text-xs font-medium text-black hover:bg-emerald-400 disabled:cursor-not-allowed disabled:bg-[#1f2937] disabled:text-[#6b7280]"
            >
              Download
            </button>
          </div>
        </div>

        <pre className="mono max-h-[70vh] overflow-auto rounded-md border border-[#1f2937] bg-[#0b0f17] p-4 text-[12.5px] leading-relaxed text-[#d1d5db]">
          {yaml}
        </pre>

        {errors.length > 0 && (
          <ul className="rounded-md border border-red-500/30 bg-red-500/5 p-3 text-xs text-red-300">
            {errors.map((e, i) => (
              <li key={i}>• {e.message}</li>
            ))}
          </ul>
        )}

        <p className="text-xs text-[#6b7280]">
          This page only generates a file; it cannot reconfigure a running node. Save it, then run:{" "}
          <code className="mono rounded bg-[#161b22] px-1.5 py-0.5">
            ./bin/mnemokv-node -config {config.id}.yaml
          </code>
        </p>
        <Link
          to="/use/dashboard"
          onClick={() => setApiBaseUrl(apiUrlFor(config.apiBindAddr, config.apiPort))}
          className="rounded-md border border-emerald-500/40 px-3 py-2 text-center text-xs text-emerald-300 hover:bg-emerald-500/10"
        >
          Open dashboard for {apiUrlFor(config.apiBindAddr, config.apiPort)}
        </Link>
      </aside>
    </div>
  )
}

function apiUrlFor(bindAddr: string, port: number): string {
  const host = bindAddr === "0.0.0.0" ? "127.0.0.1" : bindAddr === "::" ? "[::1]" : bindAddr
  return `http://${host}:${port}`
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-lg border border-[#1f2937] bg-[#0b0f17] p-5">
      <h3 className="text-sm font-semibold uppercase tracking-wider text-[#6b7280]">{title}</h3>
      <div className="flex flex-col gap-3">{children}</div>
    </section>
  )
}

function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string
  children: React.ReactNode
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="flex items-baseline justify-between">
        <span className="text-sm font-medium text-[#e6edf3]">{label}</span>
        {hint && <span className="text-xs text-[#6b7280]">{hint}</span>}
      </span>
      {children}
      {error && <span className="text-xs text-red-400">{error}</span>}
    </label>
  )
}

const inputClass =
  "w-full rounded-md border border-[#1f2937] bg-[#0d1117] px-3 py-1.5 text-sm text-white outline-none focus:border-emerald-500/60"

function TextInput({
  value,
  onChange,
  placeholder,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <input
      type="text"
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className={inputClass}
    />
  )
}

function NumberInput({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  return (
    <input
      type="number"
      value={value}
      onChange={(e) => onChange(Number(e.target.value))}
      className={inputClass}
    />
  )
}

function Segmented<T extends string>({
  options,
  value,
  onChange,
}: {
  options: readonly T[]
  value: T
  onChange: (v: T) => void
}) {
  return (
    <div className="inline-flex flex-wrap gap-1 rounded-md border border-[#1f2937] bg-[#0d1117] p-1">
      {options.map((opt) => (
        <button
          key={opt}
          type="button"
          onClick={() => onChange(opt)}
          className={[
            "rounded px-2.5 py-1 text-xs transition-colors",
            value === opt
              ? "bg-emerald-500/90 text-black"
              : "text-[#9ca3af] hover:bg-[#161b22] hover:text-white",
          ].join(" ")}
        >
          {opt}
        </button>
      ))}
    </div>
  )
}

function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className={[
        "relative inline-flex h-6 w-11 items-center rounded-full transition-colors",
        checked ? "bg-emerald-500" : "bg-[#1f2937]",
      ].join(" ")}
    >
      <span
        className={[
          "inline-block size-5 transform rounded-full bg-white transition-transform",
          checked ? "translate-x-5" : "translate-x-0.5",
        ].join(" ")}
      />
    </button>
  )
}
