import { useEffect, useState } from "react"
import { Copy, Minus, Square, X } from "lucide-react"

import { cn } from "@/lib/utils"

// Height of the custom title bar; layout math reads it via --tb-h.
export const TITLEBAR_H = "2.25rem"

// True only inside the Tauri webview — the same dashboard also runs in
// plain browsers, where the native OS chrome stays untouched.
export function isTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window
}

function isMacOS(): boolean {
  return typeof navigator !== "undefined" && /Mac OS X|Macintosh/.test(navigator.userAgent)
}

type WinAction = "minimize" | "toggleMaximize" | "close"

async function winAction(action: WinAction) {
  const { getCurrentWindow } = await import("@tauri-apps/api/window")
  const win = getCurrentWindow()
  if (action === "minimize") await win.minimize()
  else if (action === "toggleMaximize") await win.toggleMaximize()
  else await win.close()
}

function ControlButton({
  onClick,
  danger,
  label,
  children,
}: {
  onClick: () => void
  danger?: boolean
  label: string
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={cn(
        "text-muted-foreground hover:bg-muted hover:text-foreground flex h-full w-12 items-center justify-center transition-colors",
        danger && "hover:bg-red-500 hover:text-white",
      )}
    >
      {children}
    </button>
  )
}

/**
 * Custom window title bar for the Tauri desktop shell (native decorations
 * are disabled). Renders nothing in a regular browser. On macOS the native
 * traffic lights stay (titleBarStyle overlay), so only the drag surface and
 * brand are drawn here.
 */
export function AppTitlebar() {
  const [maximized, setMaximized] = useState(false)
  const tauri = isTauri()

  useEffect(() => {
    if (!tauri) return
    let cleanup: (() => void) | undefined
    let disposed = false
    ;(async () => {
      const { getCurrentWindow } = await import("@tauri-apps/api/window")
      const win = getCurrentWindow()
      setMaximized(await win.isMaximized())
      const unlisten = await win.onResized(async () => {
        setMaximized(await win.isMaximized())
      })
      if (disposed) unlisten()
      else cleanup = unlisten
    })().catch(() => {})
    return () => {
      disposed = true
      cleanup?.()
    }
  }, [tauri])

  if (!tauri) return null
  const mac = isMacOS()

  return (
    <div
      data-tauri-drag-region
      style={{ height: TITLEBAR_H }}
      className="bg-background/85 border-border/60 relative z-50 flex shrink-0 select-none items-center border-b backdrop-blur"
    >
      <div data-tauri-drag-region className={cn("flex items-center gap-2 pl-3", mac && "pl-20")}>
        <span aria-hidden className="text-base leading-none">
          🐫
        </span>
        <span className="text-sm font-semibold tracking-tight">Sabdopalon</span>
      </div>
      <div className="ml-auto flex h-full">
        {mac ? (
          // Traffic lights are drawn by the OS overlay on the left.
          <div className="w-[4.5rem]" />
        ) : (
          <>
            <ControlButton onClick={() => winAction("minimize")} label="Minimalkan">
              <Minus className="size-4" />
            </ControlButton>
            <ControlButton onClick={() => winAction("toggleMaximize")} label={maximized ? "Pulihkan" : "Maksimalkan"}>
              {maximized ? <Copy className="size-3.5" /> : <Square className="size-3.5" />}
            </ControlButton>
            <ControlButton onClick={() => winAction("close")} danger label="Tutup">
              <X className="size-4" />
            </ControlButton>
          </>
        )}
      </div>
    </div>
  )
}
