import { Suspense, lazy, useEffect, useState } from "react"
import { Navigate, Route, Routes, useLocation } from "react-router-dom"

import AppSidebar from "@/components/app-sidebar"
import { AppTitlebar, TITLEBAR_H, isTauri } from "@/components/app-titlebar"
import SiteHeader from "@/components/site-header"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "@/components/ui/sonner"
import { LiveProvider } from "@/lib/live"
import { cn } from "@/lib/utils"
import type { SetupStatus } from "@/lib/api"

// Route-level code splitting: each page (xterm, recharts, dnd-kit…) loads on
// demand instead of one 1.2 MB monolith at boot.
const DatabasePage = lazy(() => import("@/pages/database"))
const DashboardPage = lazy(() => import("@/pages/dashboard"))
const LogsPage = lazy(() => import("@/pages/logs"))
const PackagesPage = lazy(() => import("@/pages/packages"))
const ServicesPage = lazy(() => import("@/pages/services"))
const SetupPage = lazy(() => import("@/pages/setup"))
const SitesPage = lazy(() => import("@/pages/sites"))
const SslPage = lazy(() => import("@/pages/ssl"))
const TerminalPage = lazy(() => import("@/pages/terminal"))

function PageSkeleton() {
  return (
    <div className="flex flex-col gap-4 p-6">
      <Skeleton className="h-8 w-64" />
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-28 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-72 rounded-xl" />
    </div>
  )
}

export function App() {
  const [setup, setSetup] = useState<SetupStatus | null>(null)
  const [setupLoaded, setSetupLoaded] = useState(false)

  // Full-bleed pages manage their own height (app-frame layout): content
  // spans exactly from the header to the bottom of the viewport and scrolls
  // internally — no window-level gap under the fold.
  const location = useLocation()
  const fullBleed = location.pathname === "/sites" || location.pathname === "/terminal"

  // One-shot bootstrap check: unbuilt installs redirect to the setup wizard.
  useEffect(() => {
    fetch("/api/setup/status")
      .then((r) => r.json())
      .then((s: SetupStatus) => {
        setSetup(s)
        setSetupLoaded(true)
      })
      .catch(() => setSetupLoaded(true))
  }, [])

  if (!setupLoaded) return null

  // Inside the Tauri desktop shell the native title bar is replaced by our
  // own (design-system) bar; everything below shifts by --tb-h.
  const tauri = isTauri()
  const frameStyle = tauri ? ({ "--tb-h": TITLEBAR_H } as React.CSSProperties) : undefined

  // First-run gate: the wizard takes over the WHOLE window — no sidebar, no
  // header, no dashboard chrome. The user cannot reach the dashboard until
  // setup completes (backend marker decides).
  if (setup !== null && !setup.bootstrapped) {
    return (
      <div className={cn("flex min-h-dvh flex-col", tauri && "tb-frame")} style={frameStyle}>
        <AppTitlebar />
        <div className="flex flex-1 flex-col">
          <Suspense fallback={null}>
            <SetupPage />
          </Suspense>
        </div>
      </div>
    )
  }

  return (
    <div className={cn("flex min-h-dvh flex-col", tauri && "tb-frame")} style={frameStyle}>
      <AppTitlebar />
      <LiveProvider>
      <SidebarProvider
        className={cn(tauri && "min-h-[calc(100svh-var(--tb-h,0px))] flex-1")}
        style={
          {
            "--sidebar-width": "calc(var(--spacing) * 64)",
            "--header-height": "calc(var(--spacing) * 12)",
          } as React.CSSProperties
        }
      >
        <AppSidebar />
        <SidebarInset>
          <SiteHeader />
          <div className="flex flex-1 flex-col">
            <div className="@container/main flex flex-1 flex-col gap-2">
              <div
                className={
                  fullBleed
                    ? "flex h-[calc(100dvh-var(--tb-h,0px)-var(--header-height))] flex-col overflow-hidden pt-2"
                    : "flex flex-col gap-4 py-4 md:gap-6 md:py-6"
                }
              >
                <Suspense fallback={<PageSkeleton />}>
                  <Routes>
                    {/* The gate above owns the un-bootstrapped case; a manual
                        /setup visit after setup simply returns to the dashboard. */}
                    <Route path="/setup" element={<Navigate to="/dashboard" replace />} />
                    <>
                      <Route path="/" element={<DashboardPage />} />
                      <Route path="/dashboard" element={<DashboardPage />} />
                      <Route path="/sites" element={<SitesPage />} />
                      <Route path="/database" element={<DatabasePage />} />
                      <Route path="/packages" element={<PackagesPage />} />
                      <Route path="/services" element={<ServicesPage />} />
                      <Route path="/ssl" element={<SslPage />} />
                      <Route path="/settings" element={<SettingsLazy />} />
                      <Route path="/logs" element={<LogsPage />} />
                      <Route path="/terminal" element={<TerminalPage />} />
                    </>
                  </Routes>
                </Suspense>
              </div>
            </div>
          </div>
        </SidebarInset>
        <Toaster position="bottom-right" richColors closeButton />
      </SidebarProvider>
      </LiveProvider>
    </div>
  )
}

const SettingsLazy = lazy(() => import("@/pages/settings"))

export default App
