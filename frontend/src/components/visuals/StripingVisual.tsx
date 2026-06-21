import { useEffect, useState } from "react"

// fnv1a is a small non-cryptographic hash that mirrors what the engine does in
// spirit — we only need to demonstrate that different keys land in different
// stripes, not match any particular implementation.
function fnv1a(s: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return h >>> 0
}

const sampleKeys = [
  "user:42",
  "session:abc",
  "cart:7",
  "rate:1.2.3.4",
  "lock:job-9",
  "stats:hourly",
  "queue:tasks",
  "leaderboard",
  "config:flags",
  "trace:req-555",
]

type Props = {
  stripeCount?: number
}

export function StripingVisual({ stripeCount = 8 }: Props) {
  const [step, setStep] = useState(0)
  const [running, setRunning] = useState(true)

  useEffect(() => {
    if (!running) return
    const id = window.setInterval(() => {
      setStep((s) => (s + 1) % sampleKeys.length)
    }, 900)
    return () => window.clearInterval(id)
  }, [running])

  const key = sampleKeys[step]
  const target = fnv1a(key) % stripeCount

  return (
    <figure className="rounded-md border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="mb-3 flex items-center gap-3 text-xs text-[#9ca3af]">
        <span className="font-mono text-[#e6edf3]">hash("{key}")</span>
        <span>% {stripeCount}</span>
        <span>=</span>
        <span className="font-mono text-emerald-300">stripe {target}</span>
        <button
          type="button"
          onClick={() => setRunning((r) => !r)}
          className="ml-auto rounded border border-[#1f2937] px-2 py-0.5 text-[11px] text-[#9ca3af] hover:text-white"
        >
          {running ? "pause" : "play"}
        </button>
        <button
          type="button"
          onClick={() => setStep((s) => (s + 1) % sampleKeys.length)}
          className="rounded border border-[#1f2937] px-2 py-0.5 text-[11px] text-[#9ca3af] hover:text-white"
        >
          next
        </button>
      </div>

      <svg viewBox="0 0 480 220" className="w-full" role="img" aria-label="Lock striping diagram">
        {/* incoming key bubble */}
        <g>
          <rect x="10" y="98" width="120" height="28" rx="6" fill="#161b22" stroke="#1f2937" />
          <text x="70" y="116" fill="#e6edf3" fontSize="12" textAnchor="middle" fontFamily="monospace">
            {key}
          </text>
        </g>

        {/* routing arrow */}
        <path
          d={`M130 112 L220 ${24 + target * (180 / (stripeCount - 1))}`}
          stroke="#10b981"
          strokeWidth="1.5"
          fill="none"
          markerEnd="url(#arrow)"
        />
        <defs>
          <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto">
            <path d="M0 0 L10 5 L0 10 Z" fill="#10b981" />
          </marker>
        </defs>

        {/* stripes */}
        {Array.from({ length: stripeCount }).map((_, i) => {
          const y = 14 + i * (180 / (stripeCount - 1))
          const active = i === target
          return (
            <g key={i}>
              <rect
                x="225"
                y={y}
                width="220"
                height="18"
                rx="4"
                fill={active ? "#10b98122" : "#0d1117"}
                stroke={active ? "#10b981" : "#1f2937"}
              />
              <text x="232" y={y + 13} fill="#9ca3af" fontSize="11" fontFamily="monospace">
                stripe {i}
              </text>
              <circle cx="430" cy={y + 9} r="4" fill={active ? "#10b981" : "#374151"} />
            </g>
          )
        })}

        <text x="225" y="212" fill="#8b949e" fontSize="10">
          one lock per stripe — independent stripes run in parallel
        </text>
      </svg>
    </figure>
  )
}
