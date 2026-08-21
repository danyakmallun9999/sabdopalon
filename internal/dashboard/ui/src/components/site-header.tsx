import { useLocation } from "react-router-dom"
import { ExternalLink, Mail } from "lucide-react"

import { Separator } from "@/components/ui/separator"
import {
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Badge } from "@/components/ui/badge"
import type { Status } from "@/lib/api"
import { NAV_ITEMS } from "@/components/app-sidebar"

export default function SiteHeader({ status }: { status: Status | null }) {
  const location = useLocation()
  const current = NAV_ITEMS.find((n) => n.url === location.pathname)

  return (
    <header className="flex h-(--header-height) shrink-0 items-center gap-2 border-b transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center gap-2 px-4 lg:gap-3 lg:px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator
          orientation="vertical"
          className="mx-1 h-4 data-vertical:self-auto"
        />
        <h1 className="text-base font-medium">
          {current ? (
            <span className="inline-flex items-center gap-2">
              <current.icon className="size-4" />
              {current.title}
            </span>
          ) : (
            "Sabdopalon"
          )}
        </h1>

        <div className="ml-auto flex items-center gap-2">
          <Badge variant="outline" className="text-muted-foreground">
            DB {status?.database ?? "—"}
          </Badge>
          {status?.php && (
            <Badge variant="outline" className="text-muted-foreground">
              PHP {status.php}
            </Badge>
          )}
          {status?.mailpit && (
            <Badge variant="secondary" render={<a href="http://localhost:8025" target="_blank" rel="noreferrer" />}>
              <Mail /> Mailpit <ExternalLink />
            </Badge>
          )}
          <span
            aria-label={status ? "connected" : "disconnected"}
            className={`size-2 rounded-full ${status ? "bg-emerald-500" : "bg-destructive"}`}
          />
        </div>
      </div>
    </header>
  )
}
