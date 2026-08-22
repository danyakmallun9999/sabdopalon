import { forwardRef, useEffect, useImperativeHandle, useRef } from "react"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import { ClipboardAddon } from "@xterm/addon-clipboard"
import "@xterm/xterm/css/xterm.css"

export type TermStatus = "connecting" | "connected" | "closed"

export type TerminalPanelHandle = {
  /** Clear the scrollback + viewport. */
  clear: () => void
  /** Kill the WebSocket and spawn a fresh shell session. */
  restart: () => void
}

type Props = {
  /** Absolute directory the shell starts in ("" = Sabdopalon sites root). */
  dir?: string
  /** Extra CSS class for the container (default: h-[24rem]). */
  className?: string
  /** Connection state reporter for parent chrome (badges etc.). */
  onStatus?: (s: TermStatus) => void
}

// TerminalPanel is a reusable embedded terminal (xterm.js + WebSocket PTY
// against /api/terminal/ws). Used by the full Terminal page and the per-site
// right dock on the Sites page.
//
// Resize is event-driven: xterm's onResize fires whenever FitAddon changes
// the cell grid and the frame goes out immediately (no polling lag).
const TerminalPanel = forwardRef<TerminalPanelHandle, Props>(
  function TerminalPanel({ dir = "", className = "h-[24rem]", onStatus }, ref) {
    const hostRef = useRef<HTMLDivElement>(null)
    const wsRef = useRef<WebSocket | null>(null)
    const restartRef = useRef<() => void>(() => {})
    const clearRef = useRef<() => void>(() => {})

    useEffect(() => {
      const host = hostRef.current
      if (!host) return

      let disposed = false
      let reconnectTimer: ReturnType<typeof setTimeout> | undefined

      const term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, monospace',
        theme: { background: "#0b0f14" },
        scrollback: 5000,
      })
      const fit = new FitAddon()
      term.loadAddon(fit)
      term.loadAddon(new ClipboardAddon())
      term.open(host)

      let lastCols = 0
      let lastRows = 0
      const sendResize = () => {
        const ws = wsRef.current
        if (!ws || ws.readyState !== WebSocket.OPEN) return
        if (term.cols === lastCols && term.rows === lastRows) return
        lastCols = term.cols
        lastRows = term.rows
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }))
      }
      // Fires whenever fit() actually changes dimensions — push instantly.
      term.onResize(sendResize)
      // User-facing clipboard shortcuts (Ctrl+Shift+C / V) + OSC52 via addon.
      term.attachCustomKeyEventHandler((ev) => {
        if (ev.type !== "keydown") return true
        if (ev.ctrlKey && ev.shiftKey && ev.key.toLowerCase() === "c") {
          const sel = term.getSelection()
          if (sel) navigator.clipboard.writeText(sel).catch(() => {})
          return false
        }
        if (ev.ctrlKey && ev.shiftKey && ev.key.toLowerCase() === "v") {
          navigator.clipboard.readText().then((t) => term.paste(t)).catch(() => {})
          return false
        }
        return true
      })

      const fitNow = () => {
        try {
          fit.fit()
        } catch {
          /* container may be 0-sized mid-layout */
        }
      }

      const connect = () => {
        if (disposed) return
        onStatus?.("connecting")
        const qs = dir ? `?dir=${encodeURIComponent(dir)}` : ""
        const url = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/terminal/ws${qs}`
        const ws = new WebSocket(url)
        wsRef.current = ws

        ws.onopen = () => {
          if (disposed) return
          lastCols = 0
          lastRows = 0
          fitNow()
          sendResize()
          onStatus?.("connected")
          term.focus()
        }
        ws.onmessage = (ev) => {
          if (typeof ev.data === "string") term.write(ev.data)
        }
        ws.onclose = () => {
          if (disposed) return
          onStatus?.("closed")
          term.write("\r\n\x1b[90m— connection lost, retrying —\x1b[0m\r\n")
          reconnectTimer = setTimeout(connect, 1500)
        }
      }

      restartRef.current = () => {
        const ws = wsRef.current
        if (ws) { ws.onclose = null; ws.close() }
        term.reset()
        connect()
      }
      clearRef.current = () => {
        term.clear()
        term.focus()
      }

      connect()

      const ro = new ResizeObserver(fitNow)
      ro.observe(host)
      window.addEventListener("resize", fitNow)

      return () => {
        disposed = true
        if (reconnectTimer) clearTimeout(reconnectTimer)
        window.removeEventListener("resize", fitNow)
        ro.disconnect()
        const ws = wsRef.current
        if (ws) { ws.onclose = null; ws.close() }
        term.dispose()
      }
    }, [dir])

    useImperativeHandle(ref, () => ({
      clear: () => clearRef.current?.(),
      restart: () => restartRef.current?.(),
    }))

    return <div ref={hostRef} className={`${className} overflow-hidden rounded-lg border bg-[#0b0f14] p-2`} />
  },
)
export default TerminalPanel
