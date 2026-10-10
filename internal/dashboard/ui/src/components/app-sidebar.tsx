import { NavLink } from "react-router-dom"

import {
  Boxes,
  Database,
  Globe,
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
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
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
    ],
  },
]

// Flat view of the same list, for the header that resolves the current page
// title from the path — one source, so a page cannot be in the sidebar and
// nameless in the header.
export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items)

export default function AppSidebar() {
  const { status } = useLive()
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
        {NAV_GROUPS.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => (
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
        ))}
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
