import { useEffect, useState } from "react"
import { Navigate, Route, Routes } from "react-router-dom"

import AppSidebar from "@/components/app-sidebar"
import SiteHeader from "@/components/site-header"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { Toaster } from "@/components/ui/sonner"
import api, { type SetupStatus, type Status } from "@/lib/api"

import DatabasePage from "@/pages/database"
import LogsPage from "@/pages/logs"
import PackagesPage from "@/pages/packages"
import ServicesPage from "@/pages/services"
import SettingsPage from "@/pages/settings"
import SetupPage from "@/pages/setup"
import SitesPage from "@/pages/sites"
import SslPage from "@/pages/ssl"
import TerminalPage from "@/pages/terminal"

export function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [setup, setSetup] = useState<SetupStatus | null>(null)
  const [setupLoaded, setSetupLoaded] = useState(false)

  useEffect(() => {
    const tick = () => {
      api.status().then(setStatus).catch(() => setStatus(null))
    }
    const t = setInterval(tick, 5000)
    tick()
    return () => clearInterval(t)
  }, [])

  // On first load, ask the backend whether the install is bootstrapped.
  // If not, the SPA redirects to /setup (the setup wizard).
  useEffect(() => {
    api
      .setupStatus()
      .then((s) => {
        setSetup(s)
        setSetupLoaded(true)
      })
      .catch(() => setSetupLoaded(true))
  }, [])

  if (!setupLoaded) return null

  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 64)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as React.CSSProperties
      }
    >
      <AppSidebar status={status} />
      <SidebarInset>
        <SiteHeader status={status} />
        <div className="flex flex-1 flex-col">
          <div className="@container/main flex flex-1 flex-col gap-2">
            <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
              <Routes>
                <Route
                  path="/setup"
                  element={
                    setup && !setup.bootstrapped ? (
                      <SetupPage />
                    ) : (
                      <Navigate to="/sites" replace />
                    )
                  }
                />
                {setup && !setup.bootstrapped ? (
                  <Route path="*" element={<Navigate to="/setup" replace />} />
                ) : (
                  <>
                    <Route path="/" element={<SitesPage />} />
                    <Route path="/sites" element={<SitesPage />} />
                    <Route path="/database" element={<DatabasePage />} />
                    <Route path="/packages" element={<PackagesPage />} />
                    <Route path="/services" element={<ServicesPage />} />
                    <Route path="/ssl" element={<SslPage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                    <Route path="/logs" element={<LogsPage />} />
                    <Route path="/terminal" element={<TerminalPage />} />
                  </>
                )}
              </Routes>
            </div>
          </div>
        </div>
      </SidebarInset>
      <Toaster position="bottom-right" richColors closeButton />
    </SidebarProvider>
  )
}

export default App
