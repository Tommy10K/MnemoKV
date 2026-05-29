import { useMemo, useState } from "react"
import { Link } from "react-router-dom"

type Profile = {
  id: "strings" | "lists" | "zset" | "mixed"
  label: string
  summary: string
  mix: string[]
}

const profiles: Profile[] = [
  {
    id: "strings",
    label: "Strings",
    summary: "Set/get heavy traffic over plain string keys.",
    mix: ["60% SET", "35% GET", "5% INCR"],
  },
  {
    id: "lists",
    label: "Lists",
    summary: "Queue-style push/pop traffic against doubly linked lists.",
    mix: ["50% RPUSH", "30% LPOP", "20% LLEN"],
  },
  {
    id: "zset",
    label: "Sorted sets",
    summary: "Leaderboard-style writes and range queries on skip lists.",
    mix: ["60% ZADD", "30% ZRANGE", "10% ZCARD"],
  },
  {
    id: "mixed",
    label: "Mixed",
    summary: "All three data types blended together.",
    mix: ["strings + lists + zset"],
  },
]

export function WorkloadsPage() {
  const [profileId, setProfileId] = useState<Profile["id"]>("mixed")
  const [concurrency, setConcurrency] = useState(8)
  const [durationSec, setDurationSec] = useState(30)
  const [keySpan, setKeySpan] = useState(1000)
  const [addr, setAddr] = useState("127.0.0.1:6380")
  const [copied, setCopied] = useState(false)

  const profile = profiles.find((p) => p.id === profileId)!

  const command = useMemo(
    () =>
      [
        "go run ./cmd/workload",
        `-addr ${addr}`,
        `-profile ${profile.id}`,
        `-concurrency ${concurrency}`,
        `-duration ${durationSec}s`,
        `-keyspan ${keySpan}`,
      ].join(" "),
    [addr, profile.id, concurrency, durationSec, keySpan],
  )

  async function copy() {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // clipboard not available, no-op
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Workloads</h1>
        <p className="text-sm text-[#9ca3af]">
          Drive synthetic traffic at the node. Pick a profile, set the parameters, then
          run the generated command in another terminal and watch the{" "}
          <Link to="/use/dashboard" className="text-emerald-400 hover:underline">
            dashboard
          </Link>
          .
        </p>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {profiles.map((p) => (
          <button
            key={p.id}
            type="button"
            onClick={() => setProfileId(p.id)}
            className={[
              "rounded-lg border p-4 text-left transition-colors",
              p.id === profileId
                ? "border-emerald-500/60 bg-emerald-500/10"
                : "border-[#1f2937] bg-[#0b0f17] hover:border-[#374151]",
            ].join(" ")}
          >
            <div className="text-sm font-semibold text-white">{p.label}</div>
            <div className="mt-1 text-xs text-[#9ca3af]">{p.summary}</div>
            <ul className="mt-2 space-y-0.5 text-xs text-[#6b7280]">
              {p.mix.map((m) => (
                <li key={m} className="font-mono">
                  {m}
                </li>
              ))}
            </ul>
          </button>
        ))}
      </section>

      <section className="grid gap-4 rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4 sm:grid-cols-2 lg:grid-cols-4">
        <Field label="Server address">
          <input
            value={addr}
            onChange={(e) => setAddr(e.target.value)}
            className="w-full rounded-md border border-[#1f2937] bg-[#0d1117] px-2 py-1 font-mono text-sm text-[#e6edf3] focus:border-emerald-500 focus:outline-none"
          />
        </Field>
        <Field label="Concurrency">
          <NumberInput value={concurrency} min={1} max={1024} onChange={setConcurrency} />
        </Field>
        <Field label="Duration (s)">
          <NumberInput value={durationSec} min={1} max={3600} onChange={setDurationSec} />
        </Field>
        <Field label="Key span">
          <NumberInput value={keySpan} min={1} max={10_000_000} onChange={setKeySpan} />
        </Field>
      </section>

      <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
        <div className="mb-2 flex items-center justify-between">
          <div className="text-sm text-[#9ca3af]">Run this in your shell</div>
          <button
            type="button"
            onClick={copy}
            className="rounded-md bg-emerald-500/20 px-3 py-1 text-sm text-emerald-300 hover:bg-emerald-500/30"
          >
            {copied ? "copied" : "copy"}
          </button>
        </div>
        <pre className="overflow-x-auto rounded bg-[#0d1117] p-3 font-mono text-xs text-[#e6edf3]">
          {command}
        </pre>
        <p className="mt-3 text-xs text-[#6b7280]">
          The workload generator opens its own RESP connections to{" "}
          <span className="font-mono">{addr}</span>. Start a node first with{" "}
          <span className="font-mono">go run ./cmd/node configs/standalone.yaml</span>.
        </p>
      </section>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-1 text-xs uppercase tracking-wide text-[#6b7280]">
      {label}
      {children}
    </label>
  )
}

function NumberInput({
  value,
  min,
  max,
  onChange,
}: {
  value: number
  min: number
  max: number
  onChange: (n: number) => void
}) {
  return (
    <input
      type="number"
      value={value}
      min={min}
      max={max}
      onChange={(e) => {
        const n = Number(e.target.value)
        if (!Number.isFinite(n)) return
        onChange(Math.min(max, Math.max(min, Math.floor(n))))
      }}
      className="w-full rounded-md border border-[#1f2937] bg-[#0d1117] px-2 py-1 font-mono text-sm text-[#e6edf3] focus:border-emerald-500 focus:outline-none"
    />
  )
}
