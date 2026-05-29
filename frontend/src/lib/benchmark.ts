export type BenchmarkRow = {
  name: string
  family: string
  nsPerOp: number
  bytesPerOp?: number
  allocsPerOp?: number
}

// parseBenchmarkInput accepts either the raw `go test -bench -benchmem` text
// output or a JSON array of benchmark records and returns a normalised set
// of rows the chart can render. Throws on unparseable input.
export function parseBenchmarkInput(raw: string): BenchmarkRow[] {
  const trimmed = raw.trim()
  if (trimmed === "") return []

  if (trimmed.startsWith("[") || trimmed.startsWith("{")) {
    return parseJSON(trimmed)
  }
  return parseText(raw)
}

function parseJSON(raw: string): BenchmarkRow[] {
  const data = JSON.parse(raw)
  const items: unknown[] = Array.isArray(data) ? data : [data]
  const out: BenchmarkRow[] = []
  for (const item of items) {
    if (!item || typeof item !== "object") continue
    const obj = item as Record<string, unknown>
    const name = pickString(obj, ["name", "Name", "benchmark"])
    if (!name) continue
    const ns = pickNumber(obj, ["nsPerOp", "ns_per_op", "ns_op", "NsPerOp", "ns/op"])
    if (ns === undefined) continue
    const bytes = pickNumber(obj, [
      "bytesPerOp",
      "bytes_per_op",
      "b_op",
      "BytesPerOp",
      "B/op",
    ])
    const allocs = pickNumber(obj, [
      "allocsPerOp",
      "allocs_per_op",
      "allocs_op",
      "AllocsPerOp",
      "allocs/op",
    ])
    out.push({
      name,
      family: familyOf(name),
      nsPerOp: ns,
      bytesPerOp: bytes,
      allocsPerOp: allocs,
    })
  }
  return out
}

// matches lines like: BenchmarkSET-28   2926543   393.3 ns/op   184 B/op   7 allocs/op
const benchLine =
  /^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+)\s+ns\/op(?:\s+([\d.]+)\s+B\/op)?(?:\s+([\d.]+)\s+allocs\/op)?/

function parseText(raw: string): BenchmarkRow[] {
  const seen = new Map<string, BenchmarkRow[]>()
  for (const line of raw.split(/\r?\n/)) {
    const m = benchLine.exec(line.trim())
    if (!m) continue
    const name = m[1]
    const row: BenchmarkRow = {
      name,
      family: familyOf(name),
      nsPerOp: Number(m[2]),
      bytesPerOp: m[3] !== undefined ? Number(m[3]) : undefined,
      allocsPerOp: m[4] !== undefined ? Number(m[4]) : undefined,
    }
    const list = seen.get(name) ?? []
    list.push(row)
    seen.set(name, list)
  }
  // average across -count=N runs so repeated lines collapse to one row
  return Array.from(seen.values()).map(average)
}

function average(rows: BenchmarkRow[]): BenchmarkRow {
  if (rows.length === 1) return rows[0]
  const n = rows.length
  return {
    name: rows[0].name,
    family: rows[0].family,
    nsPerOp: sum(rows, (r) => r.nsPerOp) / n,
    bytesPerOp: rows[0].bytesPerOp !== undefined ? sum(rows, (r) => r.bytesPerOp ?? 0) / n : undefined,
    allocsPerOp:
      rows[0].allocsPerOp !== undefined ? sum(rows, (r) => r.allocsPerOp ?? 0) / n : undefined,
  }
}

function sum(rows: BenchmarkRow[], pick: (r: BenchmarkRow) => number): number {
  let s = 0
  for (const r of rows) s += pick(r)
  return s
}

// familyOf classifies a benchmark name into the data-structure family it
// exercises. Unknown names fall into "other".
export function familyOf(rawName: string): string {
  const upper = rawName.replace(/^Benchmark/i, "").replace(/Parallel$/i, "").toUpperCase()
  if (/^(SET|GET|INCR|DECR|DEL|EXISTS|MSET|MGET|SETNX|EXPIRE|TTL|STRLEN)/.test(upper)) {
    return "strings"
  }
  if (/^(LPUSH|RPUSH|LPOP|RPOP|LRANGE|LLEN|LINDEX)/.test(upper)) {
    return "lists"
  }
  if (/^(ZADD|ZRANGE|ZSCORE|ZCARD|ZREM|ZINCRBY)/.test(upper)) {
    return "zsets"
  }
  if (/^(HSET|HGET|HDEL|HLEN)/.test(upper)) {
    return "hashes"
  }
  return "other"
}

function pickString(obj: Record<string, unknown>, keys: string[]): string | undefined {
  for (const k of keys) {
    const v = obj[k]
    if (typeof v === "string" && v !== "") return v
  }
  return undefined
}

function pickNumber(obj: Record<string, unknown>, keys: string[]): number | undefined {
  for (const k of keys) {
    const v = obj[k]
    if (typeof v === "number" && Number.isFinite(v)) return v
    if (typeof v === "string" && v !== "" && Number.isFinite(Number(v))) return Number(v)
  }
  return undefined
}

export const familyColors: Record<string, string> = {
  strings: "#10b981",
  lists: "#60a5fa",
  zsets: "#f59e0b",
  hashes: "#a78bfa",
  other: "#9ca3af",
}

export const familyNotes: Record<string, string> = {
  strings: "Hash map lookups — O(1) amortised.",
  lists: "Doubly linked lists — O(1) push and pop at either end.",
  zsets: "Skip lists — O(log n) insert and range queries.",
  hashes: "Per-key hash map — O(1) field access.",
}
