import { Boxes, RefreshCw } from "lucide-react"
import { Link, useLocation } from "react-router-dom"

import { SidebarTrigger } from "@/components/ui/sidebar"
import { StatusDot } from "@/components/status"
import { useLive } from "@/lib/live"
import { NAV_ITEMS } from "@/components/app-sidebar"
import { Button } from "@/components/ui/button"
import { usePageRefreshState } from "@/components/page-refresh"
import { dbEngineRunning } from "@/lib/dbstatus"

// The trail the reference design puts in the top bar: on a site page you are
// one click from the list you came from.
function siteNameFrom(pathname: string): string {
  const m = pathname.match(/^\/sites\/([^/]+)$/)
  return m ? decodeURIComponent(m[1]) : ""
}

/**
 * The top bar is a status strip, not a toolbar: what the machine is doing
 * right now, nothing else. The pieces are neutral panels (a 1px edge on the
 * app background) so the coloured dot is the only thing that draws the eye.
 */
function Pill({
  children,
  className = "",
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <span
      className={
        "inline-flex h-[28px] items-center gap-1.5 rounded-lg border border-ring-inner bg-surface px-3 text-[13.5px] whitespace-nowrap text-text-secondary shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)] " +
        className
      }
    >
      {children}
    </span>
  )
}

export default function SiteHeader() {
  const { status } = useLive()
  const location = useLocation()
  const siteName = siteNameFrom(location.pathname)
  const page = NAV_ITEMS.find((n) => n.url === location.pathname)
  const dbRunning = status ? dbEngineRunning(status) : false
  const { canRefresh, refresh } = usePageRefreshState()

  return (
    <header className="flex h-(--header-height) shrink-0 items-center border-b border-border-soft bg-background transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center gap-3 px-7">
        <SidebarTrigger className="size-7 rounded-md border border-border text-text-secondary hover:bg-surface-raised hover:text-text-primary" />

        {siteName ? (
          <nav
            aria-label="Breadcrumb"
            className="flex items-center gap-1.5 text-[13.5px] text-text-secondary"
          >
            <Link to="/sites" className="hover:text-text-primary">
              Sites
            </Link>
            <span className="text-text-muted/50">/</span>
            <span className="font-semibold text-text-primary">{siteName}</span>
          </nav>
        ) : page ? (
          <h1 className="flex items-center gap-2 text-[15.5px] font-semibold tracking-[-0.01em] text-text-primary">
            <page.icon className="size-4 text-text-muted" />
            {page.title}
          </h1>
        ) : null}

        {canRefresh ? (
          <Button
            size="icon-sm"
            variant="outline"
            onClick={refresh}
            aria-label="Refresh"
            title="Refresh"
            className="ml-2"
          >
            <RefreshCw />
          </Button>
        ) : null}

        <div className="ml-auto flex items-center gap-3.5">
          {status?.database ? (
            <Pill>
              <StatusDot status={dbRunning ? "running" : "error"} />
              {status.database}
            </Pill>
          ) : (
            <Pill>
              <StatusDot status="error" />
              DB —
            </Pill>
          )}
          {(status?.php_version || status?.php) && (
            <Pill>PHP {status?.php_version ?? status?.php}</Pill>
          )}
          {status?.services && (
            <Pill>
              <Boxes className="size-3 text-text-muted" />
              Active services
            </Pill>
          )}
          <Pill className="text-[13.5px]">
            v{status?.version ?? "…"} · up {status?.uptime ?? "—"}
          </Pill>
          <StatusDot
            status={status ? "running" : "error"}
            aria-label={status ? "Dashboard connected" : "Dashboard disconnected"}
          />
        </div>
      </div>
    </header>
  )
}
