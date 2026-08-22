import { useRef, useState } from "react"
import { Plus, X } from "lucide-react"

import TerminalPanel, { type TerminalPanelHandle } from "@/components/terminal-panel"
import { Button } from "@/components/ui/button"

type ShellTab = { id: number; title: string }

let nextId = 1

// TerminalPage hosts multiple concurrent shell sessions as tabs. Sessions
// stay mounted while switching (hidden), so background jobs keep running.
export default function TerminalPage() {
  const [tabs, setTabs] = useState<ShellTab[]>([{ id: nextId++, title: "Shell 1" }])
  const [active, setActive] = useState(tabs[0].id)
  const panelRefs = useRef(new Map<number, TerminalPanelHandle>())

  function addTab() {
    const t = { id: nextId++, title: `Shell ${nextId - 1}` }
    setTabs((prev) => [...prev, t])
    setActive(t.id)
  }

  function closeTab(id: number) {
    panelRefs.current.delete(id)
    setTabs((prev) => {
      const rest = prev.filter((t) => t.id !== id)
      if (rest.length === 0) return [{ id: nextId++, title: "Shell 1" }]
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
              className="absolute inset-0 h-auto"
            />
          </div>
        ))}
      </div>
    </div>
  )
}
