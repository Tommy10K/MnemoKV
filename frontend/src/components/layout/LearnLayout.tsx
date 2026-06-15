import { NavLink, Outlet } from "react-router-dom"
import { chapters } from "@/pages/learn/chapters"

export function LearnLayout() {
  return (
    <div className="grid grid-cols-1 gap-8 md:grid-cols-[240px_1fr]">
      <aside className="md:sticky md:top-6 md:self-start">
        <NavLink
          to="/learn"
          end
          className={({ isActive }) =>
            [
              "mb-3 block rounded-md px-3 py-2 text-sm font-medium",
              isActive ? "bg-[#1f2937] text-white" : "text-[#9ca3af] hover:text-white",
            ].join(" ")
          }
        >
          Overview
        </NavLink>

        <ol className="space-y-1">
          {chapters.map((chapter, index) => (
            <li key={chapter.slug}>
              <NavLink
                to={`/learn/${chapter.slug}`}
                className={({ isActive }) =>
                  [
                    "block rounded-md px-3 py-2 text-sm leading-snug transition-colors",
                    isActive
                      ? "bg-[#1f2937] text-white"
                      : "text-[#9ca3af] hover:bg-[#161b22] hover:text-white",
                  ].join(" ")
                }
              >
                <span className="mr-2 text-[#6b7280]">{String(index + 1).padStart(2, "0")}</span>
                {chapter.title}
              </NavLink>
            </li>
          ))}
        </ol>
      </aside>

      <section className="min-w-0">
        <Outlet />
      </section>
    </div>
  )
}
