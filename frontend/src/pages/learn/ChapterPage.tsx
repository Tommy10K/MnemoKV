import { Link } from "react-router-dom"
import type { Chapter } from "./chapters"
import { chapters } from "./chapters"

type ChapterPageProps = {
  chapter: Chapter
}

export function ChapterPage({ chapter }: ChapterPageProps) {
  const index = chapters.findIndex((c) => c.slug === chapter.slug)
  const prev = index > 0 ? chapters[index - 1] : null
  const next = index < chapters.length - 1 ? chapters[index + 1] : null
  const Body = chapter.body

  return (
    <article className="flex flex-col gap-8">
      <header className="flex flex-col gap-2 border-b border-[#1f2937] pb-6">
        <span className="text-xs uppercase tracking-wider text-[#8b949e]">
          Chapter {String(index + 1).padStart(2, "0")}
        </span>
        <h1 className="text-3xl font-semibold tracking-tight text-white">{chapter.title}</h1>
        <p className="text-[#9ca3af]">{chapter.summary}</p>
      </header>

      <div className="prose-invert flex flex-col gap-5 text-[15px] leading-relaxed text-[#d1d5db]">
        <Body />
      </div>

      <nav className="mt-4 flex items-center justify-between border-t border-[#1f2937] pt-6 text-sm">
        {prev ? (
          <Link
            to={`/learn/${prev.slug}`}
            className="flex flex-col text-[#9ca3af] hover:text-white"
          >
            <span className="text-xs uppercase tracking-wider text-[#8b949e]">← Previous</span>
            <span>{prev.title}</span>
          </Link>
        ) : (
          <span />
        )}
        {next ? (
          <Link
            to={`/learn/${next.slug}`}
            className="flex flex-col text-right text-[#9ca3af] hover:text-white"
          >
            <span className="text-xs uppercase tracking-wider text-[#8b949e]">Next →</span>
            <span>{next.title}</span>
          </Link>
        ) : (
          <span />
        )}
      </nav>
    </article>
  )
}
