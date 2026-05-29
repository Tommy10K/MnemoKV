import { useRef, useState, type KeyboardEvent } from "react"
import { runCommand } from "@/api/client"
import type { CommandResult } from "@/api/types"

type Entry = {
  input: string
  result?: CommandResult
  error?: string
}

const MAX_HISTORY = 50

export function CommandConsolePage() {
  const [input, setInput] = useState("")
  const [entries, setEntries] = useState<Entry[]>([])
  const [busy, setBusy] = useState(false)
  const historyRef = useRef<{ items: string[]; index: number }>({ items: [], index: -1 })
  const outputRef = useRef<HTMLDivElement | null>(null)

  async function submit() {
    const text = input.trim()
    if (!text || busy) return

    const args = parseArgs(text)
    if (args.length === 0) return

    const hist = historyRef.current
    hist.items = [text, ...hist.items.filter((x) => x !== text)].slice(0, MAX_HISTORY)
    hist.index = -1

    setInput("")
    setBusy(true)
    const pending: Entry = { input: text }
    setEntries((prev) => trim([...prev, pending]))

    try {
      const result = await runCommand(args)
      setEntries((prev) => replaceLast(prev, { input: text, result }))
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setEntries((prev) => replaceLast(prev, { input: text, error: message }))
    } finally {
      setBusy(false)
      requestAnimationFrame(() => {
        outputRef.current?.scrollTo({ top: outputRef.current.scrollHeight })
      })
    }
  }

  function onKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.preventDefault()
      void submit()
      return
    }
    const hist = historyRef.current
    if (e.key === "ArrowUp" && hist.items.length > 0) {
      e.preventDefault()
      hist.index = Math.min(hist.index + 1, hist.items.length - 1)
      setInput(hist.items[hist.index])
    } else if (e.key === "ArrowDown") {
      e.preventDefault()
      if (hist.index <= 0) {
        hist.index = -1
        setInput("")
      } else {
        hist.index -= 1
        setInput(hist.items[hist.index])
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <header>
        <h1 className="text-2xl font-semibold text-white">Command console</h1>
        <p className="text-sm text-[#9ca3af]">
          Send RESP commands straight to the running node. Try{" "}
          <span className="font-mono text-[#e6edf3]">SET foo bar</span> then{" "}
          <span className="font-mono text-[#e6edf3]">GET foo</span>.
        </p>
      </header>

      <div
        ref={outputRef}
        className="h-[420px] overflow-y-auto rounded-lg border border-[#1f2937] bg-[#0b0f17] p-4 font-mono text-sm"
      >
        {entries.length === 0 ? (
          <div className="text-[#6b7280]">No commands yet.</div>
        ) : (
          entries.map((entry, i) => <EntryRow key={i} entry={entry} />)
        )}
      </div>

      <div className="flex items-center gap-2 rounded-lg border border-[#1f2937] bg-[#0b0f17] px-3 py-2">
        <span className="select-none font-mono text-emerald-400">{">"}</span>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={onKey}
          spellCheck={false}
          autoFocus
          placeholder="SET foo bar"
          className="flex-1 bg-transparent font-mono text-sm text-[#e6edf3] outline-none placeholder:text-[#4b5563]"
        />
        <button
          type="button"
          onClick={submit}
          disabled={busy || input.trim() === ""}
          className="rounded-md bg-emerald-500/20 px-3 py-1 text-sm text-emerald-300 hover:bg-emerald-500/30 disabled:cursor-not-allowed disabled:opacity-50"
        >
          send
        </button>
      </div>
    </div>
  )
}

function EntryRow({ entry }: { entry: Entry }) {
  return (
    <div className="mb-3 last:mb-0">
      <div className="text-[#e6edf3]">
        <span className="text-emerald-400">{"> "}</span>
        {entry.input}
      </div>
      {entry.error !== undefined ? (
        <div className="pl-4 text-red-400">network error: {entry.error}</div>
      ) : entry.result === undefined ? (
        <div className="pl-4 text-[#6b7280]">…</div>
      ) : (
        <div className="pl-4">{renderResult(entry.result)}</div>
      )}
    </div>
  )
}

function renderResult(r: CommandResult): React.ReactNode {
  switch (r.type) {
    case "string":
      return <span className="text-emerald-300">{r.value}</span>
    case "error":
      return <span className="text-red-400">(error) {r.value}</span>
    case "integer":
      return <span className="text-sky-300">(integer) {r.value}</span>
    case "bulk":
      return <span className="text-[#e6edf3]">"{r.value}"</span>
    case "nil":
      return <span className="text-[#6b7280]">(nil)</span>
    case "array":
      if (r.value.length === 0) return <span className="text-[#6b7280]">(empty array)</span>
      return (
        <ol className="list-decimal pl-5">
          {r.value.map((item, i) => (
            <li key={i}>{renderResult(item)}</li>
          ))}
        </ol>
      )
  }
}

// parseArgs splits an input line into argv-style tokens. It supports double
// and single quoted strings so values with spaces work, e.g.
// `SET greeting "hello world"`.
function parseArgs(line: string): string[] {
  const out: string[] = []
  let cur = ""
  let quote: '"' | "'" | null = null
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (quote) {
      if (ch === quote) {
        quote = null
      } else if (ch === "\\" && i + 1 < line.length) {
        cur += line[i + 1]
        i++
      } else {
        cur += ch
      }
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      continue
    }
    if (ch === " " || ch === "\t") {
      if (cur !== "") {
        out.push(cur)
        cur = ""
      }
      continue
    }
    cur += ch
  }
  if (cur !== "") out.push(cur)
  return out
}

function trim(entries: Entry[]): Entry[] {
  const MAX = 200
  return entries.length > MAX ? entries.slice(entries.length - MAX) : entries
}

function replaceLast(entries: Entry[], next: Entry): Entry[] {
  const out = entries.slice()
  out[out.length - 1] = next
  return out
}
