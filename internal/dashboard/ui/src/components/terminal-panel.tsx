import { useEffect, useRef } from "react"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import "@xterm/xterm/css/xterm.css"

type Props = {
  /** Absolute directory the shell starts in ("" = Sabdopalon sites root). */
  dir?: string
  /** Extra CSS height class for the container (default: h-[24rem]). */
  heightClass?: string
}

// TerminalPanel is a reusable embedded terminal (xterm.js + WebSocket PTY
// against /api/terminal/ws). Used by the full Terminal page and the per-site
// side panel on the Sites page.
export default function TerminalPanel({ dir = "", heightClass = "h-[24rem]" }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"JetBrains Mono", ui-monospace, monospace',
      theme: { background: "#0b0f14" },
      scrollback: 5000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    termRef.current = term

    const host = hostRef.current
    if (host) term.open(host)

    const qs = dir ? `?dir=${encodeURIComponent(dir)}` : ""
    const url = `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/terminal/ws${qs}`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      fit.fit()
      term.focus()
      term.onData((data) => ws.send(JSON.stringify({ type: "input", data })))
    }

    ws.onmessage = (ev) => {
      if (typeof ev.data === "string") term.write(ev.data)
    }

    ws.onclose = () => term.write("\r\n\x1b[31m(connection closed — refresh to reconnect)\x1b[0m\r\n")

    const onResize = () => fit.fit()
    window.addEventListener("resize", onResize)

    // Fit the terminal when its container changes size (e.g. the resizable
    // split-pane on the Sites page). Guard against the fit→resize→fit loop
    // ResizeObserver can trigger (scrollbar appears/disappears changing the
    // container by 1px): only call fit when the proposed cell dimensions
    // differ from the current ones.
    let ro: ResizeObserver | undefined
    if (typeof ResizeObserver !== "undefined" && host) {
      ro = new ResizeObserver(() => {
        if (typeof fit.proposeDimensions === "function") {
          const dims = fit.proposeDimensions()
          if (dims && dims.cols === term.cols && dims.rows === term.rows) return
        }
        fit.fit()
      })
      ro.observe(host)
    }

    // Push initial size and keep the PTY in sync.
    const sizeTimer = setInterval(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }),
        )
      }
    }, 2000)

    return () => {
      clearInterval(sizeTimer)
      window.removeEventListener("resize", onResize)
      ro?.disconnect()
      ws.close()
      term.dispose()
    }
  }, [dir])

  return (
    <div
      ref={hostRef}
      className={`${heightClass} overflow-hidden rounded-lg border bg-[#0b0f14] p-2`}
    />
  )
}
