import { useEffect, useState } from "react"

type Op =
  | { kind: "set"; key: string }
  | { kind: "get"; key: string }

// A small but representative access pattern. It is small enough to follow by
// eye while the four policies produce visibly different decisions.
const pattern: Op[] = [
  { kind: "set", key: "A" },
  { kind: "set", key: "B" },
  { kind: "set", key: "C" },
  { kind: "set", key: "D" },
  { kind: "get", key: "A" },
  { kind: "get", key: "A" },
  { kind: "get", key: "B" },
  { kind: "set", key: "E" },
  { kind: "get", key: "C" },
  { kind: "set", key: "F" },
  { kind: "get", key: "A" },
  { kind: "set", key: "G" },
]

const capacity = 4
const policies = ["FIFO", "LRU", "LFU", "Random"] as const
type Policy = (typeof policies)[number]

type Entry = {
  key: string
  insertedAt: number
  lastUsed: number
  hits: number
}

type SimState = {
  contents: Entry[]
  evicted?: string
  hit?: boolean
}

function step(prev: SimState, op: Op, t: number, policy: Policy): SimState {
  const contents = prev.contents.map((e) => ({ ...e }))

  if (op.kind === "get") {
    const found = contents.find((e) => e.key === op.key)
    if (found) {
      found.hits++
      found.lastUsed = t
      return { contents, hit: true }
    }
    return { contents, hit: false }
  }

  // set
  const existing = contents.find((e) => e.key === op.key)
  if (existing) {
    existing.lastUsed = t
    existing.hits++
    return { contents }
  }

  let evicted: string | undefined
  if (contents.length >= capacity) {
    const victim = pickVictim(contents, policy, t)
    evicted = victim.key
    contents.splice(contents.indexOf(victim), 1)
  }
  contents.push({ key: op.key, insertedAt: t, lastUsed: t, hits: 1 })
  return { contents, evicted }
}

function pickVictim(entries: Entry[], policy: Policy, t: number): Entry {
  switch (policy) {
    case "FIFO":
      return entries.slice().sort((a, b) => a.insertedAt - b.insertedAt)[0]
    case "LRU":
      return entries.slice().sort((a, b) => a.lastUsed - b.lastUsed)[0]
    case "LFU":
      return entries.slice().sort((a, b) => a.hits - b.hits || a.lastUsed - b.lastUsed)[0]
    case "Random":
      // deterministic "random" so the demo is reproducible across renders
      return entries[t % entries.length]
  }
}

function runUpTo(policy: Policy, upTo: number): SimState {
  let state: SimState = { contents: [] }
  for (let i = 0; i <= upTo && i < pattern.length; i++) {
    state = step(state, pattern[i], i, policy)
  }
  return state
}

export function EvictionVisual() {
  const [cursor, setCursor] = useState(0)
  const [running, setRunning] = useState(false)

  useEffect(() => {
    if (!running) return
    const id = window.setInterval(() => {
      setCursor((c) => {
        if (c >= pattern.length - 1) {
          setRunning(false)
          return c
        }
        return c + 1
      })
    }, 800)
    return () => window.clearInterval(id)
  }, [running])

  return (
    <figure className="rounded-md border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
        <span className="text-[#9ca3af]">capacity = {capacity}</span>
        <button
          type="button"
          onClick={() => {
            setCursor(0)
            setRunning(true)
          }}
          className="rounded border border-[#1f2937] px-2 py-1 text-[#e6edf3] hover:border-emerald-500/50 hover:text-emerald-300"
        >
          replay
        </button>
        <button
          type="button"
          onClick={() => setCursor((c) => Math.max(0, c - 1))}
          className="rounded border border-[#1f2937] px-2 py-1 text-[#e6edf3] hover:border-emerald-500/50 hover:text-emerald-300"
        >
          ← step
        </button>
        <button
          type="button"
          onClick={() => setCursor((c) => Math.min(pattern.length - 1, c + 1))}
          className="rounded border border-[#1f2937] px-2 py-1 text-[#e6edf3] hover:border-emerald-500/50 hover:text-emerald-300"
        >
          step →
        </button>
        <span className="ml-auto text-[#6b7280]">
          step {cursor + 1} / {pattern.length}
        </span>
      </div>

      <div className="mb-3 flex flex-wrap gap-1 text-[11px]">
        {pattern.map((op, i) => (
          <button
            key={i}
            type="button"
            onClick={() => setCursor(i)}
            className={[
              "rounded border px-2 py-1 font-mono",
              i === cursor
                ? "border-emerald-500 bg-emerald-500/10 text-emerald-200"
                : i < cursor
                  ? "border-[#1f2937] bg-[#0d1117] text-[#9ca3af]"
                  : "border-[#1f2937] bg-[#0d1117] text-[#4b5563]",
            ].join(" ")}
          >
            {op.kind === "set" ? "SET " : "GET "}
            {op.key}
          </button>
        ))}
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {policies.map((p) => {
          const state = runUpTo(p, cursor)
          return (
            <div key={p} className="rounded-md border border-[#1f2937] bg-[#0d1117] p-3">
              <div className="mb-2 flex items-center justify-between">
                <span className="font-semibold text-white">{p}</span>
                {state.evicted ? (
                  <span className="rounded bg-red-500/20 px-2 py-0.5 text-[10px] font-mono text-red-300">
                    evicted {state.evicted}
                  </span>
                ) : state.hit === false ? (
                  <span className="rounded bg-amber-500/20 px-2 py-0.5 text-[10px] text-amber-300">
                    miss
                  </span>
                ) : state.hit ? (
                  <span className="rounded bg-emerald-500/20 px-2 py-0.5 text-[10px] text-emerald-300">
                    hit
                  </span>
                ) : null}
              </div>
              <div className="flex gap-1">
                {Array.from({ length: capacity }).map((_, slot) => {
                  const entry = state.contents[slot]
                  return (
                    <div
                      key={slot}
                      className={[
                        "flex h-10 w-10 items-center justify-center rounded border font-mono text-sm",
                        entry
                          ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-200"
                          : "border-dashed border-[#1f2937] text-[#374151]",
                      ].join(" ")}
                    >
                      {entry?.key ?? "·"}
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })}
      </div>

      <p className="mt-3 text-xs text-[#9ca3af]">
        Same access pattern, four policies. FIFO evicts the oldest insert; LRU evicts the
        least-recently used; LFU evicts the least-frequently used; Random just picks a slot.
      </p>
    </figure>
  )
}
