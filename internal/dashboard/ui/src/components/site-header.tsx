import { Boxes } from "lucide-react"

import { SidebarTrigger } from "@/components/ui/sidebar"
import { StatusDot, ToneBadge } from "@/components/status"
import { useLive } from "@/lib/live"
import { dbEngineRunning } from "@/lib/dbstatus"

export default function SiteHeader() {
  const { status } = useLive()

  return (
    <header className="flex h-(--header-height) shrink-0 items-center border-b border-border-soft bg-background/80 transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center gap-3 px-5 lg:px-7">
        <SidebarTrigger className="-ml-1.5 text-text-secondary hover:text-text-primary" />
        <span className="hidden text-[10px] font-medium tracking-[0.12em] text-text-muted uppercase sm:inline">
          Control room
        </span>

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
          {status?.services ? (
            <ToneBadge tone="muted" render={<a href="/services" />}>
              <Boxes /> Active services
            </ToneBadge>
          ) : null}
          <span className="hidden text-[11px] whitespace-nowrap text-text-muted tabular-nums sm:inline">
            v{status?.version ?? "…"} · up {status?.uptime ?? "—"}
          </span>
          <StatusDot
            status={status ? "running" : "error"}
            aria-label={status ? "Dashboard connected" : "Dashboard disconnected"}
          />
        </div>
      </div>
    </header>
  )
}
