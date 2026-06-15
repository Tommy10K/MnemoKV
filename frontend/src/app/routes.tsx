import { lazy, Suspense } from "react"
import { Navigate, Route, Routes } from "react-router-dom"
import { MainLayout } from "@/components/layout/MainLayout"
import { LearnLayout } from "@/components/layout/LearnLayout"
import { UseLayout } from "@/components/layout/UseLayout"
import { HomePage } from "@/pages/home/HomePage"
import { LearnLanding } from "@/pages/learn/LearnLanding"
import { ChapterPage } from "@/pages/learn/ChapterPage"
import { chapters } from "@/pages/learn/chapters"

const ConfigPage = lazy(() =>
  import("@/pages/use/ConfigPage").then((module) => ({ default: module.ConfigPage })),
)
const DashboardPage = lazy(() =>
  import("@/pages/use/DashboardPage").then((module) => ({ default: module.DashboardPage })),
)
const CommandConsolePage = lazy(() =>
  import("@/pages/use/CommandConsolePage").then((module) => ({
    default: module.CommandConsolePage,
  })),
)
const WorkloadsPage = lazy(() =>
  import("@/pages/use/WorkloadsPage").then((module) => ({ default: module.WorkloadsPage })),
)
const ClusterPage = lazy(() =>
  import("@/pages/use/ClusterPage").then((module) => ({ default: module.ClusterPage })),
)
const BenchmarksPage = lazy(() =>
  import("@/pages/use/BenchmarksPage").then((module) => ({ default: module.BenchmarksPage })),
)
const EvictionLabPage = lazy(() =>
  import("@/pages/use/EvictionLabPage").then((module) => ({ default: module.EvictionLabPage })),
)

export function AppRoutes() {
  return (
    <Suspense fallback={<RouteFallback />}>
      <Routes>
        <Route element={<MainLayout />}>
          <Route index element={<HomePage />} />

          <Route path="learn" element={<LearnLayout />}>
            <Route index element={<LearnLanding />} />
            {chapters.map((chapter) => (
              <Route
                key={chapter.slug}
                path={chapter.slug}
                element={<ChapterPage chapter={chapter} />}
              />
            ))}
          </Route>

          <Route path="use" element={<UseLayout />}>
            <Route index element={<ConfigPage />} />
            <Route path="dashboard" element={<DashboardPage />} />
            <Route path="console" element={<CommandConsolePage />} />
            <Route path="workloads" element={<WorkloadsPage />} />
            <Route path="cluster" element={<ClusterPage />} />
            <Route path="benchmarks" element={<BenchmarksPage />} />
            <Route path="eviction" element={<EvictionLabPage />} />
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </Suspense>
  )
}

function RouteFallback() {
  return <div className="py-12 text-center text-sm text-[#9ca3af]">Loading page...</div>
}
