import { useMemo, useRef, useState, type ChangeEvent } from "react"
import { BarChart, type BarRow } from "@/components/charts/BarChart"
import {
  familyColors,
  familyNotes,
  parseBenchmarkInput,
  type BenchmarkRow,
} from "@/lib/benchmark"

type Metric = "nsPerOp" | "bytesPerOp" | "allocsPerOp"

const metrics: { id: Metric; label: string; unit: string; format: (v: number) => string }[] = [
  { id: "nsPerOp", label: "ns / op", unit: "ns", format: (v) => formatNs(v) },
  { id: "bytesPerOp", label: "B / op", unit: "B", format: (v) => formatBytes(v) },
  { id: "allocsPerOp", label: "allocs / op", unit: "allocs", format: (v) => v.toFixed(0) },
]

export function BenchmarksPage() {
  const [rows, setRows] = useState<BenchmarkRow[]>([])
  const [source, setSource] = useState<string>("")
  const [error, setError] = useState<string | null>(null)
  const [metric, setMetric] = useState<Metric>("nsPerOp")
  const fileRef = useRef<HTMLInputElement | null>(null)

  function load(raw: string, label: string) {
    try {
      const parsed = parseBenchmarkInput(raw)
      if (parsed.length === 0) {
        setError("No benchmark rows found in the input.")
        return
      }
      setRows(parsed)
      setSource(label)
      setError(null)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(`Could not parse input: ${msg}`)
    }
  }

  function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    file.text().then((text) => load(text, file.name))
  }

  const families = useMemo(() => groupByFamily(rows), [rows])
  const activeMetric = metrics.find((m) => m.id === metric)!
  const allBars = useMemo(() => rowsToBars(rows, metric), [rows, metric])

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-white">Benchmarks</h1>
        <p className="text-sm text-[#9ca3af]">
          Load the output of <span className="font-mono">scripts/benchmark.sh</span> (or any{" "}
          <span className="font-mono">go test -bench -benchmem</span> run) and compare how the
          data structures behave.
        </p>
      </header>

      <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            className="rounded-md bg-emerald-500/20 px-3 py-1.5 text-sm text-emerald-300 hover:bg-emerald-500/30"
          >
            Load file…
          </button>
          <input
            ref={fileRef}
            type="file"
            accept=".json,.txt,.log,application/json,text/plain"
            onChange={onFile}
            className="hidden"
          />
          <span className="text-xs text-[#6b7280]">
            Accepts the raw text output or a JSON array; both work.
          </span>
          {source ? (
            <span className="ml-auto text-xs text-[#9ca3af]">
              loaded <span className="font-mono text-[#e6edf3]">{source}</span>
            </span>
          ) : null}
        </div>

        <details className="mt-3">
          <summary className="cursor-pointer text-xs text-[#9ca3af] hover:text-white">
            …or paste the output here
          </summary>
          <PasteBox onLoad={(text) => load(text, "pasted input")} />
        </details>

        {error ? <p className="mt-3 text-sm text-red-400">{error}</p> : null}
      </section>

      {rows.length === 0 ? (
        <EmptyState />
      ) : (
        <>
          <section className="flex flex-wrap items-center gap-2">
            {metrics.map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => setMetric(m.id)}
                className={[
                  "rounded-md border px-3 py-1.5 text-sm transition-colors",
                  metric === m.id
                    ? "border-emerald-500/60 bg-emerald-500/10 text-emerald-300"
                    : "border-[#1f2937] bg-[#0b0f17] text-[#9ca3af] hover:text-white",
                ].join(" ")}
              >
                {m.label}
              </button>
            ))}
            <span className="ml-2 text-xs text-[#6b7280]">
              showing {activeMetric.label} for {rows.length} benchmark{rows.length === 1 ? "" : "s"}
            </span>
          </section>

          <section className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4">
            <h2 className="mb-3 text-sm text-[#9ca3af]">All benchmarks</h2>
            <BarChart data={allBars} format={activeMetric.format} yLabel={activeMetric.unit} />
            <Legend />
          </section>

          <section className="grid gap-4 lg:grid-cols-2">
            {families.map(({ family, rows: famRows }) => (
              <div
                key={family}
                className="rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4"
              >
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="text-sm font-semibold text-white capitalize">{family}</h3>
                  <span className="text-xs text-[#6b7280]">{famRows.length} benchmark(s)</span>
                </div>
                {familyNotes[family] ? (
                  <p className="mb-2 text-xs text-[#9ca3af]">{familyNotes[family]}</p>
                ) : null}
                <BarChart
                  data={rowsToBars(famRows, metric)}
                  format={activeMetric.format}
                  height={220}
                />
              </div>
            ))}
          </section>
        </>
      )}
    </div>
  )
}

function PasteBox({ onLoad }: { onLoad: (text: string) => void }) {
  const [text, setText] = useState("")
  return (
    <div className="mt-2 flex flex-col gap-2">
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={6}
        spellCheck={false}
        placeholder="BenchmarkSET-28&#9;2926543&#9;393.3 ns/op&#9;184 B/op&#9;7 allocs/op"
        className="w-full rounded-md border border-[#1f2937] bg-[#0d1117] p-2 font-mono text-xs text-[#e6edf3] focus:border-emerald-500 focus:outline-none"
      />
      <button
        type="button"
        onClick={() => onLoad(text)}
        disabled={text.trim() === ""}
        className="self-start rounded-md bg-emerald-500/20 px-3 py-1 text-sm text-emerald-300 hover:bg-emerald-500/30 disabled:opacity-50"
      >
        Parse
      </button>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-[#1f2937] bg-[#0b0f17] p-6 text-sm text-[#9ca3af]">
      <p className="text-white">No benchmark data loaded yet.</p>
      <p className="mt-2">Generate a fresh set with:</p>
      <pre className="mt-2 overflow-x-auto rounded bg-[#0d1117] p-3 font-mono text-xs text-[#e6edf3]">
        ./scripts/benchmark.sh
      </pre>
      <p className="mt-2">
        Then load <span className="font-mono">results/engine_bench.txt</span> or paste the output
        above.
      </p>
    </div>
  )
}

function Legend() {
  return (
    <div className="mt-3 flex flex-wrap gap-3 text-xs">
      {Object.entries(familyColors).map(([family, color]) => (
        <span key={family} className="flex items-center gap-1.5 text-[#9ca3af]">
          <span className="h-2 w-2 rounded-full" style={{ background: color }} />
          {family}
        </span>
      ))}
    </div>
  )
}

function rowsToBars(rows: BenchmarkRow[], metric: Metric): BarRow[] {
  const out: BarRow[] = []
  for (const r of rows) {
    const value = r[metric]
    if (value === undefined || !Number.isFinite(value)) continue
    out.push({
      name: r.name.replace(/^Benchmark/, ""),
      value,
      color: familyColors[r.family] ?? familyColors.other,
    })
  }
  return out
}

function groupByFamily(rows: BenchmarkRow[]): { family: string; rows: BenchmarkRow[] }[] {
  const map = new Map<string, BenchmarkRow[]>()
  for (const r of rows) {
    const list = map.get(r.family) ?? []
    list.push(r)
    map.set(r.family, list)
  }
  const order = ["strings", "lists", "zsets", "hashes", "other"]
  return order
    .filter((f) => map.has(f))
    .map((family) => ({ family, rows: map.get(family)! }))
}

function formatNs(v: number): string {
  if (v < 1000) return `${v.toFixed(1)} ns`
  if (v < 1_000_000) return `${(v / 1000).toFixed(2)} µs`
  return `${(v / 1_000_000).toFixed(2)} ms`
}

function formatBytes(v: number): string {
  if (v < 1024) return `${v.toFixed(0)} B`
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`
  return `${(v / (1024 * 1024)).toFixed(1)} MB`
}
