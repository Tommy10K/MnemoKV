import type { ReactNode } from "react"

export function H2({ children }: { children: ReactNode }) {
  return <h2 className="mt-2 text-xl font-semibold text-white">{children}</h2>
}

export function P({ children }: { children: ReactNode }) {
  return <p>{children}</p>
}

export function UL({ children }: { children: ReactNode }) {
  return <ul className="list-disc space-y-1 pl-6 text-[#d1d5db]">{children}</ul>
}

export function Code({ children }: { children: ReactNode }) {
  return (
    <code className="mono rounded bg-[#161b22] px-1.5 py-0.5 text-[13px] text-[#e6edf3]">
      {children}
    </code>
  )
}

export function Pre({ children }: { children: ReactNode }) {
  return (
    <pre className="mono overflow-x-auto rounded-md border border-[#1f2937] bg-[#0b0f17] p-4 text-[13px] leading-relaxed text-[#d1d5db]">
      {children}
    </pre>
  )
}

export function Callout({ children }: { children: ReactNode }) {
  return (
    <aside className="rounded-md border border-emerald-500/30 bg-emerald-500/5 p-4 text-[14px] text-emerald-100/90">
      {children}
    </aside>
  )
}
