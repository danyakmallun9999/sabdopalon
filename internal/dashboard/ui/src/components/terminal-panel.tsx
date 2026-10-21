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
  /**
   * Named server-side session: the shell survives disconnects (route
   * changes, reloads) and reconnects replay its buffered output. Omit for
   * the legacy ephemeral behaviour.
   */
  sessionKey?: string
  /**
   * Optional command override: instead of an interactive shell the server
   * spawns this program as the session child (e.g. ["mariadb"] to drop
   * straight into the MariaDB client). Omit for the default shell.
   */
  cmd?: string[]
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
  function TerminalPanel({ dir = "", sessionKey, cmd, className = "h-[24rem]", onStatus }, ref) {
    const hostRef = useRef<HTMLDivElement>(null)
    const wsRef = useRef<WebSocket | null>(null)
    const restartRef = useRef<() => void>(() => {})
    const clearRef = useRef<() => void>(() => {})
    // One-shot flag: the next (re)connect replaces the named server session
    // with a brand-new shell instead of reattaching.
    const freshRef = useRef(false)
    // cmd/onStatus have no stable identity — the parent renders an array
    // literal and may redefine the callback — and resubscribing the socket on
    // that churn would drop a live terminal on every render. Both are read
    // through refs; a deliberate reconnect reacts to dir/sessionKey/cmdKey.
    const cmdRef = useRef(cmd)
    const onStatusRef = useRef(onStatus)
    useEffect(() => {
      cmdRef.current = cmd
      onStatusRef.current = onStatus
    })
    // Stabilize the cmd override: a fresh array literal per render would
    // otherwise retrigger the effect (and reconnect). Join to a string key.
    const cmdKey = cmd?.join(" ") ?? ""

    useEffect(() => {
      const host = hostRef.current
      if (!host) return

      let disposed = false
      let reconnectTimer: ReturnType<typeof setTimeout> | undefined

      const term = new Terminal({
        cursorBlink: true,
        fontSize: 12,
        fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, monospace',
        theme: {
          background: "#0a0a0a",
          foreground: "#e5e5e5",
          cursor: "#e07b28",
          cursorAccent: "#0a0a0a",
          selectionBackground: "#e07b2840",
          black: "#171717",
          red: "#ff6b6b",
          green: "#00d492",
          yellow: "#f5b544",
          blue: "#6aa9ff",
          magenta: "#c792ea",
          cyan: "#4dd0e1",
          white: "#fafafa",
          brightBlack: "#8c8c8c",
          brightRed: "#ff8f8f",
          brightGreen: "#5fe3b5",
          brightYellow: "#ffd27f",
          brightBlue: "#a8c8ff",
          brightMagenta: "#d8b4fe",
          brightCyan: "#8ae5f0",
          brightWhite: "#ffffff",
        },
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
      // Keyboard/paste input → the CURRENT socket (wsRef survives reconnects).
      term.onData((data) => {
        const ws = wsRef.current
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: "input", data }))
        }
      })
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
        onStatusRef.current?.("connecting")
        const qs = new URLSearchParams()
        if (dir) qs.set("dir", dir)
        const cmdArg = cmdRef.current
        if (cmdArg?.length) cmdArg.forEach((c) => qs.append("cmd", c))
        if (sessionKey) {
          qs.set("session", sessionKey)
          if (freshRef.current) {
            qs.set("fresh", "1")
            freshRef.current = false
          }
        }
        const url = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/terminal/ws${qs.size ? `?${qs}` : ""}`
        const ws = new WebSocket(url)
        wsRef.current = ws

        ws.onopen = () => {
          if (disposed) return
          lastCols = 0
          lastRows = 0
          fitNow()
          sendResize()
          onStatusRef.current?.("connected")
          term.focus()
        }
        ws.onmessage = (ev) => {
          if (typeof ev.data === "string") term.write(ev.data)
        }
        ws.onclose = () => {
          if (disposed) return
          onStatusRef.current?.("closed")
          term.write("\r\n\x1b[90m— connection lost, retrying —\x1b[0m\r\n")
          reconnectTimer = setTimeout(connect, 1500)
        }
      }

      restartRef.current = () => {
        const ws = wsRef.current
        if (ws) { ws.onclose = null; ws.close() }
        if (sessionKey) freshRef.current = true // replace the server session
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
      // xterm's canvas renderer can stay blank if open() ran while the host
      // was mid-layout (common when the panel mounts inside a tab). A fit()
      // on the next animation frame, once the browser has a real size, forces
      // a redraw and recovers the visible terminal.
      requestAnimationFrame(() => {
        if (disposed) return
        fitNow()
        sendResize()
      })

      return () => {
        disposed = true
        if (reconnectTimer) clearTimeout(reconnectTimer)
        window.removeEventListener("resize", fitNow)
        ro.disconnect()
        const ws = wsRef.current
        if (ws) { ws.onclose = null; ws.close() }
        term.dispose()
      }
    }, [dir, sessionKey, cmdKey])

    useImperativeHandle(ref, () => ({
      clear: () => clearRef.current?.(),
      restart: () => restartRef.current?.(),
    }))

    return <div ref={hostRef} className={`${className} overflow-hidden bg-[#0a0a0a] p-2`} />
  },
)
export default TerminalPanel
