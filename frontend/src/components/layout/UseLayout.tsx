import { NavLink, Outlet } from "react-router-dom"

const tabs = [
  { to: "/use", label: "Configure", end: true },
  { to: "/use/dashboard", label: "Dashboard" },
  { to: "/use/console", label: "Console" },
  { to: "/use/workloads", label: "Workloads" },
  { to: "/use/cluster", label: "Cluster", disabled: true },
  { to: "/use/benchmarks", label: "Benchmarks", disabled: true },
]

export function UseLayout() {
  return (
    <div className="flex flex-col gap-6">
      <nav className="flex flex-wrap gap-1 border-b border-[#1f2937] pb-2">
        {tabs.map((tab) =>
          tab.disabled ? (
            <span
              key={tab.to}
              className="cursor-not-allowed rounded-md px-3 py-1.5 text-sm text-[#4b5563]"
              title="Coming in a later phase"
            >
              {tab.label}
            </span>
          ) : (
            <NavLink
              key={tab.to}
              to={tab.to}
              end={tab.end}
              className={({ isActive }) =>
                [
                  "rounded-md px-3 py-1.5 text-sm transition-colors",
                  isActive
                    ? "bg-[#1f2937] text-white"
                    : "text-[#9ca3af] hover:bg-[#161b22] hover:text-white",
                ].join(" ")
              }
            >
              {tab.label}
            </NavLink>
          ),
        )}
      </nav>

      <Outlet />
    </div>
  )
}
