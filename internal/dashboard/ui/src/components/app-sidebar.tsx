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

export default function AppSidebar() {
  const { status } = useLive()
  const proxyPort = status?.low_ports ? "80/443" : `${status?.http_port ?? "…"}/${status?.https_port ?? "…"}`

  return (
    <Sidebar collapsible="offcanvas" className="border-r border-sidebar-border">
      <SidebarContent className="px-1 py-3">
        {NAV_GROUPS.map((group) => (
          <SidebarGroup key={group.label} className="px-2 py-1.5">
            <SidebarGroupLabel className="h-7 px-2.5 text-[10px] font-medium tracking-[0.1em] text-text-muted uppercase">
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
                          className="h-[33px] gap-2.5 rounded-lg px-2.5 text-[12.5px] text-text-secondary data-active:border data-active:border-ring-inner data-active:bg-surface-raised data-active:text-text-primary data-active:[&_svg]:text-brand"
                        >
                          <item.icon className="size-[15px]" />
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

      <SidebarFooter className="p-3">
        <div className="rounded-lg border border-ring-inner bg-surface p-2.5">
          <div className="flex items-center gap-2">
            <StatusDot
              status={status ? "running" : "error"}
              aria-label={status ? "Dashboard connected" : "Dashboard disconnected"}
            />
            <span className="text-[11.5px] font-medium text-text-secondary">Proxy</span>
            <span className="ml-auto font-mono text-[10.5px] text-text-muted">:{proxyPort}</span>
          </div>
          <p className="mt-1.5 text-[10.5px] leading-relaxed text-text-muted">
            {status?.low_ports ? "Clean URLs active" : "HTTPS ready"}
          </p>
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
