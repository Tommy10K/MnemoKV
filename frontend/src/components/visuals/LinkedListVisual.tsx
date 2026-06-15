import { useState } from "react"

export function LinkedListVisual() {
  const [items, setItems] = useState<string[]>(["b", "c", "d"])
  const [counter, setCounter] = useState(0)

  function next(): string {
    const next = String.fromCharCode("a".charCodeAt(0) + ((counter + 4) % 26))
    setCounter((c) => c + 1)
    return next
  }

  return (
    <figure className="rounded-md border border-[#1f2937] bg-[#0b0f17] p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
        <Button onClick={() => setItems((xs) => [next(), ...xs])}>LPUSH</Button>
        <Button onClick={() => setItems((xs) => [...xs, next()])}>RPUSH</Button>
        <Button onClick={() => setItems((xs) => xs.slice(1))} disabled={items.length === 0}>
          LPOP
        </Button>
        <Button onClick={() => setItems((xs) => xs.slice(0, -1))} disabled={items.length === 0}>
          RPOP
        </Button>
        <span className="ml-auto text-[#6b7280]">length = {items.length}</span>
      </div>

      <div className="overflow-x-auto pb-2">
        <div className="flex min-w-fit items-center gap-1 px-1">
          <Endpoint label="HEAD" />
          {items.length === 0 ? (
            <div className="rounded border border-dashed border-[#1f2937] px-3 py-2 text-xs text-[#6b7280]">
              empty list
            </div>
          ) : (
            items.map((v, i) => (
              <Node key={`${v}-${i}`} value={v} first={i === 0} last={i === items.length - 1} />
            ))
          )}
          <Endpoint label="TAIL" />
        </div>
      </div>

      <p className="mt-3 text-xs text-[#9ca3af]">
        Each node carries pointers to its neighbours, so push and pop at either end is O(1) — no
        copying, no shifting.
      </p>
    </figure>
  )
}

function Node({ value, first, last }: { value: string; first: boolean; last: boolean }) {
  return (
    <div className="flex items-center">
      <Arrow active={!first} />
      <div className="rounded-md border border-emerald-500/50 bg-emerald-500/10 px-3 py-2 font-mono text-sm text-emerald-200">
        {value}
      </div>
      <Arrow active={!last} />
    </div>
  )
}

function Arrow({ active }: { active: boolean }) {
  return (
    <span className={active ? "px-1 text-[#10b981]" : "px-1 text-transparent"}>↔</span>
  )
}

function Endpoint({ label }: { label: string }) {
  return (
    <span className="rounded border border-[#1f2937] bg-[#0d1117] px-2 py-1 text-[10px] uppercase tracking-wide text-[#6b7280]">
      {label}
    </span>
  )
}

function Button({
  onClick,
  disabled,
  children,
}: {
  onClick: () => void
  disabled?: boolean
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md border border-[#1f2937] bg-[#0d1117] px-2.5 py-1 font-mono text-[11px] text-[#e6edf3] hover:border-emerald-500/50 hover:text-emerald-300 disabled:opacity-40 disabled:hover:border-[#1f2937] disabled:hover:text-[#e6edf3]"
    >
      {children}
    </button>
  )
}
