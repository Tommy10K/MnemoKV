import { useMemo, useState } from "react"
import { Link } from "react-router-dom"
import {
  configToYaml,
  defaultClusterPeers,
  defaultStandalone,
  peersForAutomaticMode,
  validate,
  type EvictionPolicy,
  type FailoverMode,
  type Mode,
  type NodeConfig,
  type Peer,
} from "@/lib/config"
import { downloadFile } from "@/lib/download"
import { useAppStore } from "@/store/appStore"

const evictionPolicies: EvictionPolicy[] = ["noeviction", "fifo", "lru", "lfu", "random"]
const failoverModes: FailoverMode[] = ["manual", "automatic"]

export function ConfigPage() {
  const setApiBaseUrl = useAppStore((state) => state.setApiBaseUrl)
  const [config, setConfig] = useState<NodeConfig>(defaultStandalone)
  const errors = useMemo(() => validate(config), [config])
  const yaml = useMemo(() => configToYaml(config), [config])

  function update<K extends keyof NodeConfig>(key: K, value: NodeConfig[K]) {
    setConfig((c) => ({ ...c, [key]: value }))
  }

  function switchMode(mode: Mode) {
    if (mode === "standalone") {
      setConfig((c) => ({
        ...c,
        mode,
        dataDir: "./data",
        port: 6380,
        apiPort: 7380,
        clusterId: "demo-cluster",
        shardingEnabled: false,
        replicationEnabled: false,
        failoverMode: "manual",
        peers: [],
      }))
      return
    }

    if (mode === "clustered" && config.peers.length === 0) {
      setConfig((c) => ({
        ...c,
        mode,
        dataDir: "./data/node-1",
        port: 6381,
        apiPort: 7381,
        clusterId: "demo-cluster",
        shardingEnabled: true,
        replicationEnabled: true,
        slotCount: 1024,
        peers: defaultClusterPeers(3),
      }))
      return
    }
    update("mode", mode)
  }

  function switchFailoverMode(failoverMode: FailoverMode) {
    setConfig((c) => {
      if (failoverMode === "automatic") {
        const peers = peersForAutomaticMode(c.peers.length > 0 ? c.peers : defaultClusterPeers(5))
        const bootstrapNodeId = peers.some((peer) => peer.id === c.controllerBootstrapNodeId)
          ? c.controllerBootstrapNodeId
          : peers[0]?.id ?? "node-1"

        return {
          ...c,
          failoverMode,
          dataDir:
            c.dataDir === "./data" || c.dataDir === "./data/node-1"
              ? `./data/auto/${c.id}`
              : c.dataDir,
          clusterId: c.clusterId === "demo-cluster" ? "demo-auto-cluster" : c.clusterId,
          replicationEnabled: true,
          peers,
          controllerControlPort: c.controllerControlPort || 7481,
          controllerRaftDir:
            c.controllerRaftDir === "./data/node-1/controller" || !c.controllerRaftDir.trim()
              ? `./data/auto/${c.id}/controller`
              : c.controllerRaftDir,
          controllerBootstrapNodeId: bootstrapNodeId,
          controlPlaneRequestSigningSecret:
            c.controlPlaneRequestSigningSecret.trim() || "mnemokv-local-demo-controller-secret",
        }
      }

      return {
        ...c,
        failoverMode,
        dataDir: c.dataDir === `./data/auto/${c.id}` ? `./data/${c.id}` : c.dataDir,
        clusterId: c.clusterId === "demo-auto-cluster" ? "demo-cluster" : c.clusterId,
      }
    })
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
        peers: [
          ...c.peers,
          {
            id: `node-${nextIndex}`,
            address: `127.0.0.1:${6380 + nextIndex}`,
            apiAddress: `127.0.0.1:${7380 + nextIndex}`,
            controlAddress: `127.0.0.1:${7480 + nextIndex}`,
          },
        ],
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
  const peerGridClass =
    config.failoverMode === "automatic"
      ? "grid gap-2 sm:grid-cols-[1fr_1.25fr_1.25fr_1.25fr_auto]"
      : "grid gap-2 sm:grid-cols-[1fr_1.4fr_1.4fr_auto]"

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Configure a node</h1>
        <p className="text-sm text-[#9ca3af]">Generate a validated standalone or cluster YAML configuration.</p>
      </header>
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
            <div className="rounded-md border border-sky-500/40 bg-sky-500/10 p-3 text-xs text-sky-200">
              Cluster mode uses fixed slots, proxy routing, and one synchronous replica per shard.
              Failover is selected at startup: manual keeps operator-driven repair, while automatic
              starts the embedded controller. Every node must use the same cluster id, peer list,
              and failover mode.
            </div>
            <Field label="Cluster id" error={errorByField.get("clusterId")}>
              <TextInput value={config.clusterId} onChange={(v) => update("clusterId", v)} />
            </Field>
            <Field label="Slot count" error={errorByField.get("slotCount")}>
              <NumberInput value={config.slotCount} onChange={(v) => update("slotCount", v)} />
            </Field>
            <Field label="Failover mode">
              <Segmented
                options={failoverModes}
                value={config.failoverMode}
                onChange={(v) => switchFailoverMode(v)}
              />
            </Field>
            <p className="text-xs text-[#9ca3af]">
              Sharding and replication are required in cluster mode. Writes are acknowledged only
              after the assigned replica applies the next ordered record. Changing failover mode
              requires updating every node config and restarting the cluster.
            </p>

            {config.failoverMode === "automatic" && (
              <div className="flex flex-col gap-3 border-t border-[#1f2937] pt-3">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-[#8b949e]">
                  Automatic controller
                </h3>
                <div className="grid gap-3 md:grid-cols-2">
                  <Field
                    label="Controller port"
                    error={errorByField.get("controllerControlPort")}
                  >
                    <NumberInput
                      value={config.controllerControlPort}
                      onChange={(v) => update("controllerControlPort", v)}
                    />
                  </Field>
                  <Field
                    label="Bootstrap node id"
                    error={errorByField.get("controllerBootstrapNodeId")}
                  >
                    <TextInput
                      value={config.controllerBootstrapNodeId}
                      onChange={(v) => update("controllerBootstrapNodeId", v)}
                    />
                  </Field>
                  <Field label="Raft directory" error={errorByField.get("controllerRaftDir")}>
                    <TextInput
                      value={config.controllerRaftDir}
                      onChange={(v) => update("controllerRaftDir", v)}
                    />
                  </Field>
                  <Field
                    label="Signing secret"
                    error={errorByField.get("controlPlaneRequestSigningSecret")}
                  >
                    <TextInput
                      value={config.controlPlaneRequestSigningSecret}
                      onChange={(v) => update("controlPlaneRequestSigningSecret", v)}
                    />
                  </Field>
                  <Field label="Observe interval (ms)" error={errorByField.get("controllerTiming")}>
                    <NumberInput
                      value={config.controllerObserveIntervalMs}
                      onChange={(v) => update("controllerObserveIntervalMs", v)}
                    />
                  </Field>
                  <Field label="Failure timeout (ms)" error={errorByField.get("controllerTiming")}>
                    <NumberInput
                      value={config.controllerFailureTimeoutMs}
                      onChange={(v) => update("controllerFailureTimeoutMs", v)}
                    />
                  </Field>
                  <Field label="Consecutive failures" error={errorByField.get("controllerTiming")}>
                    <NumberInput
                      value={config.controllerConsecutiveFailures}
                      onChange={(v) => update("controllerConsecutiveFailures", v)}
                    />
                  </Field>
                  <Field
                    label="Rebalance skew"
                    error={errorByField.get("controllerRebalance")}
                  >
                    <NumberInput
                      value={config.controllerRebalanceSkewThreshold}
                      onChange={(v) => update("controllerRebalanceSkewThreshold", v)}
                    />
                  </Field>
                  <Field
                    label="Migration rate limit"
                    error={errorByField.get("controllerRebalance")}
                  >
                    <NumberInput
                      value={config.controllerMigrationRateLimit}
                      onChange={(v) => update("controllerMigrationRateLimit", v)}
                    />
                  </Field>
                </div>
              </div>
            )}

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
                <div className={`${peerGridClass} text-xs text-[#8b949e]`}>
                  <span>Node id</span>
                  <span>RESP address (client/data port)</span>
                  <span>HTTP API address (dashboard/admin port)</span>
                  {config.failoverMode === "automatic" && <span>Controller address (Raft port)</span>}
                  <span className="sr-only">Remove</span>
                </div>
                {config.peers.map((peer, i) => (
                  <div key={i} className={peerGridClass}>
                    <TextInput
                      value={peer.id}
                      onChange={(v) => updatePeer(i, { id: v })}
                      placeholder="id"
                      ariaLabel={`Peer ${i + 1} node id`}
                    />
                    <TextInput
                      value={peer.address}
                      onChange={(v) => updatePeer(i, { address: v })}
                      placeholder="host:port"
                      ariaLabel={`Peer ${i + 1} RESP address`}
                    />
                    <TextInput
                      value={peer.apiAddress}
                      onChange={(v) => updatePeer(i, { apiAddress: v })}
                      placeholder="API host:port"
                      ariaLabel={`Peer ${i + 1} API address`}
                    />
                    {config.failoverMode === "automatic" && (
                      <TextInput
                        value={peer.controlAddress}
                        onChange={(v) => updatePeer(i, { controlAddress: v })}
                        placeholder="controller host:port"
                        ariaLabel={`Peer ${i + 1} controller address`}
                      />
                    )}
                    <button
                      type="button"
                      onClick={() => removePeer(i)}
                      className="shrink-0 rounded-md border border-[#1f2937] px-2 text-sm text-[#9ca3af] hover:border-red-500/40 hover:text-red-400"
                      title="Remove"
                      aria-label={`Remove peer ${peer.id || i + 1}`}
                    >
                      x
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
          <h2 className="text-sm font-semibold uppercase tracking-wider text-[#8b949e]">
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
              className="rounded-md bg-emerald-500/90 px-3 py-1 text-xs font-medium text-black hover:bg-emerald-400 disabled:cursor-not-allowed disabled:bg-[#1f2937] disabled:text-[#8b949e]"
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
              <li key={i}>- {e.message}</li>
            ))}
          </ul>
        )}

        <p className="text-xs text-[#8b949e]">
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
      <h2 className="text-sm font-semibold uppercase tracking-wider text-[#8b949e]">{title}</h2>
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
        {hint && <span className="text-xs text-[#8b949e]">{hint}</span>}
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
  ariaLabel,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  ariaLabel?: string
}) {
  return (
    <input
      type="text"
      value={value}
      placeholder={placeholder}
      aria-label={ariaLabel}
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
          aria-pressed={value === opt}
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
