import { Link, useLocation } from "react-router-dom"
import { Boxes, ChevronRight } from "lucide-react"

import { Separator } from "@/components/ui/separator"
import {
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { StatusDot, ToneBadge } from "@/components/status"
import { useLive } from "@/lib/live"
import { dbEngineRunning } from "@/lib/dbstatus"
import { NAV_ITEMS } from "@/components/app-sidebar"

// extractSiteName returns the site name from a /sites/:name path, or "".
function extractSiteName(pathname: string): string {
  const m = pathname.match(/^\/sites\/([^/]+)$/)
  return m ? decodeURIComponent(m[1]) : ""
}

export default function SiteHeader() {
  const { status } = useLive()
  const location = useLocation()
  const current = NAV_ITEMS.find((n) => n.url === location.pathname)
  const siteName = extractSiteName(location.pathname)

  return (
    <header className="flex h-(--header-height) shrink-0 items-center gap-2 border-b transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center gap-2 px-4 lg:gap-3 lg:px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator
          orientation="vertical"
          className="mx-1 h-4 data-vertical:self-auto"
        />
        <h1 className="text-base font-medium">
          {siteName ? (
            <span className="inline-flex items-center gap-1.5">
              <Link to="/sites" className="text-muted-foreground hover:text-foreground transition-colors">
                Sites
              </Link>
              <ChevronRight className="text-muted-foreground/50 size-3.5" />
              <span>{siteName}</span>
            </span>
          ) : current ? (
            <span className="inline-flex items-center gap-2">
              <current.icon className="size-4" />
              {current.title}
            </span>
          ) : (
            "Sabdopalon"
          )}
        </h1>

        <div className="ml-auto flex items-center gap-2">
          {status?.database ? (
            <ToneBadge tone={dbEngineRunning(status) ? "success" : "warning"}>
              DB {status.database}
              {status.database !== "sqlite" ? (dbEngineRunning(status) ? " ✓" : " ✗") : ""}
            </ToneBadge>
          ) : (
            <ToneBadge tone="muted">DB —</ToneBadge>
          )}
          {(status?.php_version || status?.php) && (
            <ToneBadge tone="muted">PHP {status?.php_version ?? status?.php}</ToneBadge>
          )}
          {status?.services && (
            /* A link, not a status: outline weight so it sits level with the
               badges beside it (it used to be the one solid, double-icon chip
               in the row), and the variant already styles anchor hover. The
               external-link icon went with it — this opens an in-app page. */
            <ToneBadge tone="muted" render={<a href="/services" />}>
              <Boxes /> Active services
            </ToneBadge>
          )}
          <StatusDot
            status={status ? "running" : "error"}
            aria-label={status ? "Dashboard connected" : "Dashboard disconnected"}
          />
        </div>
      </div>
    </header>
  )
}
