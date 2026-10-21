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

export type NavItem = { title: string; url: string; icon: typeof LayoutDashboard }

/**
 * The sidebar groups the nine pages by what the user came to do. One group
 * called "Manage" holding everything told nobody anything; "Tools" and "System"
 * are what people actually look for.
 *
 * This is plain data, so it lives in its own module: the sidebar renders it and
 * the top bar flattens it — neither should have to reach into the other's file.
 */
export const NAV_GROUPS: { label: string; items: NavItem[] }[] = [
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

// The same list, flattened: the top bar names the page you are on.
export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items)
