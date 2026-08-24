import { useEffect, useRef, useState } from "react"
import { Plus, X } from "lucide-react"

import TerminalPanel, { type TerminalPanelHandle } from "@/components/terminal-panel"
import { Button } from "@/components/ui/button"

type ShellTab = { id: number; title: string; sk: string }

// Tabs (and their session keys) persist across route changes and reloads:
// the server keeps each named shell alive, so coming back reattaches and
// replays instead of spawning a fresh one.
const TABS_KEY = "sabdopalon.terminal.tabs"

function newShell(id: number): ShellTab {
  // The session key is a per-tab lifetime nonce — never reused, so a new
  // tab can never accidentally reattach to a closed tab's old shell.
  const nonce = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
  return { id, title: `Shell ${id}`, sk: `term-page:${nonce}` }
}

function loadTabs(): ShellTab[] {
  try {
    const raw = localStorage.getItem(TABS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ShellTab[]
      if (Array.isArray(parsed) && parsed.length > 0 && parsed.every((t) => typeof t.id === "number" && typeof t.sk === "string" && t.sk.startsWith("term-page:"))) {
        return parsed
      }
    }
  } catch {
    /* fresh */
  }
  return [newShell(1)]
}

function killSession(sk: string) {
  // Fire-and-forget: the server destroys the shell; janitor would
  // eventually do it anyway, but closing a tab should be immediate.
  const proto = location.protocol === "https:" ? "https" : "http"
  fetch(`${proto}://${location.host}/api/terminal/ws?session=${encodeURIComponent(sk)}&kill=1`).catch(() => {})
}

// TerminalPage hosts multiple concurrent shell sessions as tabs. Tabs and
// sessions survive route changes and reloads: panels reattach by session key.
export default function TerminalPage() {
  const [tabs, setTabs] = useState<ShellTab[]>(loadTabs)
  const [active, setActive] = useState<number>(() => tabs[0].id)
  const panelRefs = useRef(new Map<number, TerminalPanelHandle>())

  useEffect(() => {
    localStorage.setItem(TABS_KEY, JSON.stringify(tabs))
  }, [tabs])

  function addTab() {
    const id = Math.max(...tabs.map((t) => t.id)) + 1
    const t = newShell(id)
    setTabs((prev) => [...prev, t])
    setActive(t.id)
  }

  function closeTab(id: number) {
    const dead = tabs.find((t) => t.id === id)
    if (dead) killSession(dead.sk)
    panelRefs.current.delete(id)
    setTabs((prev) => {
      const rest = prev.filter((t) => t.id !== id)
      if (rest.length === 0) {
        const fresh = newShell(1)
        setActive(fresh.id)
        return [fresh]
      }
      if (active === id) setActive(rest[rest.length - 1].id)
      return rest
    })
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 px-4 pt-2 lg:px-6">
      {/* Tab bar */}
      <div className="flex items-center gap-1">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setActive(t.id)}
            className={`group inline-flex h-8 items-center gap-2 rounded-lg border px-3 text-sm transition-colors ${
              t.id === active
                ? "bg-accent font-medium"
                : "text-muted-foreground hover:bg-muted/60 border-transparent"
            }`}
          >
            {t.title}
            <span
              role="button"
              tabIndex={0}
              aria-label={`Close ${t.title}`}
              onClick={(e) => {
                e.stopPropagation()
                closeTab(t.id)
              }}
              onKeyDown={(e) => e.key === "Enter" && closeTab(t.id)}
              className="hover:bg-background rounded-sm p-0.5 opacity-50 group-hover:opacity-100"
            >
              <X className="size-3.5" />
            </span>
          </button>
        ))}
        <Button variant="ghost" size="icon-sm" onClick={addTab} title="New shell">
          <Plus />
        </Button>
      </div>

      {/* Panels — kept mounted so background sessions survive tab switches. */}
      <div className="relative min-h-0 flex-1">
        {tabs.map((t) => (
          <div key={t.id} className={`absolute inset-0 ${t.id === active ? "" : "hidden"}`}>
            <TerminalPanel
              ref={(h) => {
                if (h) panelRefs.current.set(t.id, h)
              }}
              sessionKey={t.sk}
              className="absolute inset-0 h-auto"
            />
          </div>
        ))}
      </div>
    </div>
  )
}
