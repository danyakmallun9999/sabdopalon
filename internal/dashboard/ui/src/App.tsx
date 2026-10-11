import { Suspense, lazy, useEffect, useState } from "react"
import { Navigate, Outlet, Route, Routes } from "react-router-dom"

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
const SiteDetailPage = lazy(() => import("@/pages/site-detail"))
const SslPage = lazy(() => import("@/pages/ssl"))
const TerminalPage = lazy(() => import("@/pages/terminal"))
const SettingsLazy = lazy(() => import("@/pages/settings"))
const AboutPage = lazy(() => import("@/pages/about"))

function PageSkeleton() {
  return (
    <div className="flex flex-col gap-4 px-4 py-6 lg:px-6">
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

/** The wizard owns the whole window on first run, so this is the app's first
 *  impression. It used to be `fallback={null}`: a blank window for as long as
 *  the chunk took to arrive. */
function SetupSkeleton() {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-3 px-4 py-6 sm:px-6">
      <Skeleton className="h-9 w-56" />
      <Skeleton className="h-52 w-full rounded-xl" />
      <Skeleton className="h-16 w-full rounded-xl" />
    </div>
  )
}

/**
 * Layouts, not a list of route names.
 *
 * Which pages scroll internally used to be decided by comparing the pathname
 * against a hard-coded array in this file (`fullBleed`), so a new page meant
 * editing two places — and forgetting one silently changed how that page
 * scrolled. The distinction now lives where the routes are declared, and the
 * height comes from --app-h, which the shell defines once.
 */
function FullBleedLayout() {
  return (
    <div className="h-(--app-h) flex min-h-0 flex-col overflow-hidden pt-2">
      <Outlet />
    </div>
  )
}

function PageLayout() {
  return (
    <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
      <Outlet />
    </div>
  )
}

export function App() {
  const [setup, setSetup] = useState<SetupStatus | null>(null)
  const [setupLoaded, setSetupLoaded] = useState(false)

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
  // own (design-system) bar: it is FIXED at the top (never scrolls away) and
  // the frame is pushed down by --tb-h so no content hides underneath it.
  const tauri = isTauri()
  const frameStyle = tauri
    ? ({ "--tb-h": TITLEBAR_H, paddingTop: TITLEBAR_H } as React.CSSProperties)
    : undefined

  // First-run gate: the wizard takes over the WHOLE window — no sidebar, no
  // header, no dashboard chrome. The user cannot reach the dashboard until
  // setup completes (backend marker decides).
  if (setup !== null && !setup.bootstrapped) {
    return (
      <div className={cn("flex flex-col", tauri ? "tb-frame" : "min-h-dvh")} style={frameStyle}>
        <AppTitlebar />
        <div className="flex flex-1 flex-col">
          <Suspense fallback={<SetupSkeleton />}>
            <SetupPage />
          </Suspense>
        </div>
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col", tauri ? "tb-frame" : "min-h-dvh")} style={frameStyle}>
      <AppTitlebar />
      <LiveProvider>
      <SidebarProvider
        className={cn(tauri && "min-h-(--app-h) flex-1")}
        style={
          {
            "--sidebar-width": "calc(var(--spacing) * 64)",
            "--header-height": "calc(var(--spacing) * 12)",
            // The frame the pages live in: viewport minus the desktop title bar
            // and the app header. Defined ONCE — pages read h-(--app-h) instead
            // of each repeating a calc() chain (there used to be three, mixing
            // dvh and svh).
            "--app-h": "calc(100dvh - var(--tb-h, 0px) - var(--header-height))",
          } as React.CSSProperties
        }
      >
        <AppSidebar />
        <SidebarInset>
          <SiteHeader />
          <div className="@container/main flex min-h-0 flex-1 flex-col">
            <Suspense fallback={<PageSkeleton />}>
              <Routes>
                {/* The gate above owns the un-bootstrapped case; a manual
                    /setup visit after setup simply returns to the dashboard. */}
                <Route path="/setup" element={<Navigate to="/dashboard" replace />} />
                <Route element={<FullBleedLayout />}>
                  <Route path="/sites" element={<SitesPage />} />
                  <Route path="/sites/:name" element={<SiteDetailPage />} />
                  <Route path="/database" element={<DatabasePage />} />
                  <Route path="/terminal" element={<TerminalPage />} />
                </Route>
                <Route element={<PageLayout />}>
                  {/* The dashboard is canonical at /dashboard: "/" redirects
                      instead of rendering it, so the address bar always says
                      where you are. The desktop shell navigates to the bare
                      origin, which used to leave the window showing "/" — and
                      the URL is how anyone reports what they were looking at. */}
                  <Route path="/" element={<Navigate to="/dashboard" replace />} />
                  <Route path="/dashboard" element={<DashboardPage />} />
                  <Route path="/packages" element={<PackagesPage />} />
                  <Route path="/about" element={<AboutPage />} />
                  <Route path="/services" element={<ServicesPage />} />
                  <Route path="/ssl" element={<SslPage />} />
                  <Route path="/settings" element={<SettingsLazy />} />
                  <Route path="/logs" element={<LogsPage />} />
                </Route>
                {/* A path that is not a route — a stale bookmark, a mistyped
                    URL, a link from an older build — used to render the app
                    chrome around an empty page, which reads as "the dashboard
                    is broken". Send it to the dashboard instead of showing
                    nothing. */}
                <Route path="*" element={<Navigate to="/dashboard" replace />} />
              </Routes>
            </Suspense>
          </div>
        </SidebarInset>
        <Toaster position="bottom-right" closeButton />
      </SidebarProvider>
      </LiveProvider>
    </div>
  )
}

export default App
