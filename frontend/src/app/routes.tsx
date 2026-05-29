import { Navigate, Route, Routes } from "react-router-dom"
import { MainLayout } from "@/components/layout/MainLayout"
import { LearnLayout } from "@/components/layout/LearnLayout"
import { UseLayout } from "@/components/layout/UseLayout"
import { HomePage } from "@/pages/home/HomePage"
import { LearnLanding } from "@/pages/learn/LearnLanding"
import { ChapterPage } from "@/pages/learn/ChapterPage"
import { chapters } from "@/pages/learn/chapters"
import { ConfigPage } from "@/pages/use/ConfigPage"
import { DashboardPage } from "@/pages/use/DashboardPage"
import { CommandConsolePage } from "@/pages/use/CommandConsolePage"

export function AppRoutes() {
  return (
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
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
