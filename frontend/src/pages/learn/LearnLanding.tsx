import { Link } from "react-router-dom"
import { chapters } from "./chapters"

export function LearnLanding() {
  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-2">
        <h1 className="text-3xl font-semibold tracking-tight text-white">Learn</h1>
        <p className="max-w-2xl text-[#9ca3af]">
          Twelve short chapters that walk through the theory behind MnemoKV. Each chapter explains
          one idea and connects it to how the implementation actually works. Read them in order or
          jump to a topic you care about.
        </p>
      </header>

      <ol className="flex flex-col gap-3">
        {chapters.map((chapter, index) => (
          <li key={chapter.slug}>
            <Link
              to={`/learn/${chapter.slug}`}
              className="flex items-start gap-4 rounded-md border border-[#1f2937] bg-[#0b0f17] p-4 transition-colors hover:border-emerald-500/40 hover:bg-[#111722]"
            >
              <span className="mono shrink-0 text-sm text-[#6b7280]">
                {String(index + 1).padStart(2, "0")}
              </span>
              <div className="flex flex-col gap-1">
                <span className="font-medium text-white">{chapter.title}</span>
                <span className="text-sm text-[#9ca3af]">{chapter.summary}</span>
              </div>
            </Link>
          </li>
        ))}
      </ol>
    </div>
  )
}
