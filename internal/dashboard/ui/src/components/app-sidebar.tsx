import { NavLink } from "react-router-dom"

import {
  Boxes,
  Database,
  Globe,
  Info,
  LayoutDashboard,
  Package,
  ScrollText,
  Settings,
  ShieldCheck,
  TerminalSquare,
} from "lucide-react"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { StatusDot } from "@/components/status"
import { useLive } from "@/lib/live"

type NavItem = { title: string; url: string; icon: typeof LayoutDashboard }

/**
 * The sidebar groups the nine pages by what the user came to do. One group
 * called "Manage" holding everything told nobody anything; "Tools" and "System"
 * are what people actually look for.
 */
const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
  {
    label: "Manage",
    items: [
      { title: "Dashboard", url: "/dashboard", icon: LayoutDashboard },
      { title: "Sites", url: "/sites", icon: Globe },
      { title: "Database", url: "/database", icon: Database },
    ],
  },
  {
    label: "Tools",
    items: [
      { title: "Services", url: "/services", icon: Boxes },
      { title: "Packages", url: "/packages", icon: Package },
      { title: "Terminal", url: "/terminal", icon: TerminalSquare },
      { title: "Logs", url: "/logs", icon: ScrollText },
    ],
  },
  {
    label: "System",
    items: [
      { title: "SSL / HTTPS", url: "/ssl", icon: ShieldCheck },
      { title: "Settings", url: "/settings", icon: Settings },
      { title: "About", url: "/about", icon: Info },
    ],
  },
]

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
          <SidebarGroup key={group.label} className="px-0 py-0">
            <SidebarGroupLabel className="h-6 px-2.5 text-[10px] font-semibold tracking-[0.08em] text-text-muted uppercase">
              {group.label}
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="gap-0.5">
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.url}>
                    <NavLink to={item.url}>
                      {({ isActive }) => (
                        <SidebarMenuButton
                          tooltip={item.title}
                          isActive={isActive}
                          render={<span />}
                          className={
                            "h-9 gap-2.5 rounded-lg border border-ring-inner px-2.5 text-[13px] font-normal text-text-secondary [&_svg]:size-4 [&_svg]:text-text-muted " +
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
              <span className="text-[11.5px] text-text-secondary">Proxy</span>
              <span className="ml-auto font-mono text-[11px] text-text-primary">
                {proxyPort}
              </span>
            </div>
            <p className="text-[11px] text-text-muted">
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
