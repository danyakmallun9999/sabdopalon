import { NavLink } from "react-router-dom"

import {
  Database,
  Globe,
  Package,
  ScrollText,
  Settings,
  ShieldCheck,
} from "lucide-react"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import type { Status } from "@/lib/api"

export const NAV_ITEMS = [
  { title: "Sites", url: "/sites", icon: Globe },
  { title: "Database", url: "/database", icon: Database },
  { title: "Packages", url: "/packages", icon: Package },
  { title: "SSL / HTTPS", url: "/ssl", icon: ShieldCheck },
  { title: "Settings", url: "/settings", icon: Settings },
  { title: "Logs", url: "/logs", icon: ScrollText },
]

export default function AppSidebar({ status }: { status: Status | null }) {
  return (
    <Sidebar collapsible="offcanvas">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              className="data-[slot=sidebar-menu-button]:p-1.5!"
              render={<NavLink to="/sites" />}
            >
              <img
                src="/logo.png"
                alt="Sabdopalon logo"
                className="size-7 rounded-md"
                draggable={false}
              />
              <span className="flex flex-col">
                <span className="text-base leading-tight font-semibold">Sabdopalon</span>
                <span className="text-muted-foreground text-xs">
                  v{status?.version ?? "…"} · up {status?.uptime ?? "—"}
                </span>
              </span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Manage</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV_ITEMS.map((item) => (
                <SidebarMenuItem key={item.url}>
                  <NavLink to={item.url}>
                    {({ isActive }) => (
                      <SidebarMenuButton
                        tooltip={item.title}
                        isActive={isActive}
                        render={<span />}
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
      </SidebarContent>

      <SidebarFooter>
        <div className="text-muted-foreground px-3 py-2 text-xs">
          {status?.low_ports ? (
            <p>Clean URLs active (:80/:443)</p>
          ) : (
            <p>
              Proxy :{status?.http_port ?? "…"} · HTTPS :{status?.https_port ?? "…"}
            </p>
          )}
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
