import { useMemo, useState } from "react"

type Node = {
  value: number
  level: number // highest level this node participates in (0-indexed)
}

const initial: Node[] = [
  { value: 3, level: 0 },
  { value: 7, level: 2 },
  { value: 12, level: 1 },
  { value: 19, level: 0 },
  { value: 23, level: 3 },
  { value: 31, level: 1 },
  { value: 42, level: 0 },
  { value: 55, level: 2 },
]

const totalLevels = 4

export function SkipListVisual() {
  const [target, setTarget] = useState<number>(31)

  // path: which (level, index) cells the search visits.
  const path = useMemo(() => searchPath(initial, target), [target])

  return (
    <figure className="rounded-md border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
        <span className="text-[#9ca3af]">Search for</span>
        {initial.map((n) => (
          <button
            key={n.value}
            type="button"
            onClick={() => setTarget(n.value)}
            className={[
              "rounded border px-2 py-0.5 font-mono",
              target === n.value
                ? "border-emerald-500 bg-emerald-500/10 text-emerald-200"
                : "border-[#1f2937] bg-[#0d1117] text-[#9ca3af] hover:text-white",
            ].join(" ")}
          >
            {n.value}
          </button>
        ))}
      </div>

      <div className="space-y-1 overflow-x-auto pb-2">
        {Array.from({ length: totalLevels })
          .map((_, idx) => totalLevels - 1 - idx)
          .map((level) => (
            <Row
              key={level}
              level={level}
              nodes={initial}
              visited={path.filter((p) => p.level === level).map((p) => p.index)}
              found={path.length > 0 && path[path.length - 1].value === target ? target : undefined}
            />
          ))}
      </div>

      <p className="mt-3 text-xs text-[#9ca3af]">
        Higher levels skip over many nodes at once; lower levels are dense. Each insertion picks a
        level at random, so searches are O(log n) on average.
      </p>
    </figure>
  )
}

function Row({
  level,
  nodes,
  visited,
  found,
}: {
  level: number
  nodes: Node[]
  visited: number[]
  found: number | undefined
}) {
  return (
    <div className="flex items-center gap-1">
      <span className="w-12 shrink-0 text-right font-mono text-[10px] text-[#6b7280]">
        L{level}
      </span>
      <span className="rounded border border-[#1f2937] bg-[#0d1117] px-2 py-1 font-mono text-[11px] text-[#6b7280]">
        head
      </span>
      {nodes.map((n, i) => {
        const present = n.level >= level
        const wasVisited = visited.includes(i)
        const isFound = found === n.value
        return present ? (
          <span key={i} className="flex items-center">
            <span className={wasVisited ? "px-1 text-emerald-400" : "px-1 text-[#1f2937]"}>→</span>
            <span
              className={[
                "rounded border px-2 py-1 font-mono text-[11px]",
                isFound
                  ? "border-emerald-500 bg-emerald-500/20 text-emerald-200"
                  : wasVisited
                    ? "border-emerald-500/50 bg-emerald-500/10 text-emerald-200"
                    : "border-[#1f2937] bg-[#0d1117] text-[#9ca3af]",
              ].join(" ")}
            >
              {n.value}
            </span>
          </span>
        ) : (
          <span key={i} className="flex items-center">
            <span className="px-1 text-[#1f2937]">·</span>
            <span className="rounded border border-dashed border-[#1f2937] px-2 py-1 font-mono text-[11px] text-[#1f2937]">
              {n.value}
            </span>
          </span>
        )
      })}
    </div>
  )
}

function searchPath(nodes: Node[], target: number): { level: number; index: number; value: number }[] {
  const trail: { level: number; index: number; value: number }[] = []
  let level = totalLevels - 1
  let i = -1 // -1 means "head"
  while (level >= 0) {
    // walk forward at current level while next node exists and is <= target
    let j = i + 1
    while (j < nodes.length) {
      if (nodes[j].level < level) {
        j++
        continue
      }
      if (nodes[j].value > target) break
      i = j
      trail.push({ level, index: i, value: nodes[i].value })
      if (nodes[i].value === target) return trail
      j++
    }
    level--
  }
  return trail
}
