import { NavLink } from "react-router-dom"

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { StatusDot } from "@/components/status"
import { NAV_GROUPS } from "@/components/nav-data"
import { useLive } from "@/lib/live-context"

// The active page is drawn the same way a button is: a 1px outer edge, a 2px
// gap in the app background, then the raised panel with its own 1px edge.
const ACTIVE_RING =
  "data-active:shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)]"

export default function AppSidebar() {
  const { status } = useLive()
  const proxyPort = status?.low_ports ? "80" : `${status?.http_port ?? "…"}`
  const httpsPort = status?.low_ports ? "443" : `${status?.https_port ?? "…"}`

  return (
    <Sidebar collapsible="offcanvas" className="border-r border-sidebar-border">
      <SidebarContent className="px-3.5 py-5">
        {NAV_GROUPS.map((group) => (
          <SidebarGroup key={group.label} className="px-0 pt-2 pb-0 first:pt-0">
            <SidebarGroupContent>
              <SidebarMenu className="gap-1.5">
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.url}>
                    <NavLink to={item.url}>
                      {({ isActive }) => (
                        <SidebarMenuButton
                          tooltip={item.title}
                          isActive={isActive}
                          render={<span />}
                          className={
                            "relative h-9 gap-2.5 rounded-lg border border-transparent px-2.5 text-[13.5px] font-normal text-text-secondary [&_svg]:size-4 [&_svg]:text-text-muted data-active:z-10 " +
                            ACTIVE_RING +
                            " data-active:border-ring-inner data-active:bg-surface-raised data-active:font-semibold data-active:text-text-primary data-active:[&_svg]:text-brand"
                          }
                        >
                          <item.icon />
                          <span>{item.title}</span>
                        </SidebarMenuButton>
                      )}
                    </NavLink>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="p-3.5">
        <div className="rounded-xl border border-ring-outer bg-ring-gap p-[2px] shadow-[0_0_0_2px_var(--ring-gap)]">
          <div className="flex flex-col gap-1 rounded-[13px] border border-ring-inner bg-surface-inset px-3 py-2.5">
            <div className="flex items-center gap-2">
              <StatusDot
                status={status ? "running" : "error"}
                aria-label={status ? "Dashboard connected" : "Dashboard disconnected"}
              />
              <span className="text-[13.5px] text-text-secondary">Proxy</span>
              <span className="ml-auto font-mono text-[13.5px] text-text-primary">
                {proxyPort}
              </span>
            </div>
            <p className="text-[13.5px] text-text-muted">
              {status?.low_ports
                ? `Clean URLs active (${proxyPort}/${httpsPort})`
                : `HTTPS on ${httpsPort}`}
            </p>
          </div>
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
