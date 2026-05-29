import { NavLink, Outlet } from "react-router-dom"

const navItems = [
  { to: "/", label: "Home", end: true },
  { to: "/learn", label: "Learn" },
  { to: "/use", label: "Use" },
]

export function MainLayout() {
  return (
    <div className="flex min-h-screen flex-col bg-[#0d1117] text-[#e6edf3]">
      <header className="border-b border-[#1f2937] bg-[#0b0f17]">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
          <NavLink to="/" className="flex items-center gap-2 text-lg font-semibold tracking-tight">
            <span className="inline-block size-2 rounded-full bg-emerald-400" />
            MnemoKV
          </NavLink>

          <nav className="flex gap-1">
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

      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <Outlet />
      </main>

      <footer className="border-t border-[#1f2937] py-4 text-center text-xs text-[#6b7280]">
        MnemoKV — educational in-memory distributed key-value store
      </footer>
    </div>
  )
}
