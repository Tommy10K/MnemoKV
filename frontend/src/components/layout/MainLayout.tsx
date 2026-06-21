import { useEffect, useRef } from "react"
import { NavLink, Outlet, useLocation } from "react-router-dom"

const navItems = [
  { to: "/", label: "Home", end: true },
  { to: "/learn", label: "Learn" },
  { to: "/use", label: "Use" },
]

export function MainLayout() {
  const location = useLocation()
  const mainRef = useRef<HTMLElement | null>(null)
  const previousPath = useRef(location.pathname)

  useEffect(() => {
    if (previousPath.current !== location.pathname) {
      previousPath.current = location.pathname
      mainRef.current?.focus()
    }
  }, [location.pathname])

  return (
    <div className="flex min-h-screen flex-col bg-[#0d1117] text-[#e6edf3]">
      <a href="#main-content" className="skip-link">Skip to main content</a>
      <header className="border-b border-[#1f2937] bg-[#0b0f17]">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-4 py-3 sm:px-6">
          <NavLink to="/" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <span className="inline-block size-2 rounded-full bg-emerald-400" />
            MnemoKV
          </NavLink>

          <nav aria-label="Primary" className="flex gap-1">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  [
                    "rounded-md px-3 py-1.5 text-sm transition-colors",
                    isActive
                      ? "bg-[#1f2937] text-white"
                      : "text-[#9ca3af] hover:bg-[#161b22] hover:text-white",
                  ].join(" ")
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
        </div>
      </header>

      <main ref={mainRef} id="main-content" tabIndex={-1} className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 outline-none sm:px-6 sm:py-8">
        <Outlet />
      </main>

      <footer className="border-t border-[#1f2937] py-4 text-center text-xs text-[#8b949e]">
        MnemoKV — educational in-memory distributed key-value store
      </footer>
    </div>
  )
}
