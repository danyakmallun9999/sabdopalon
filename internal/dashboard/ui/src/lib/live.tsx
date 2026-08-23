import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react"

import type { Status } from "@/lib/api"

type LiveState = {
  /** Canonical server snapshot — THE source every page renders from. */
  status: Status | null
  /** true while no snapshot has arrived yet (first paint / reconnecting). */
  connecting: boolean
}

const LiveContext = createContext<LiveState>({ status: null, connecting: true })

/**
 * LiveProvider subscribes ONCE to the server's SSE feed (/api/events) and
 * shares the canonical status snapshot with the whole app through
 * useLive(). This replaces the per-page status pollers that used to race
 * each other and show disagreeing data.
 *
 * If the stream errors (server restarting), it falls back to light polling
 * until the stream comes back — EventSource retries on its own, we just
 * bridge gaps so badges don't flicker to "disconnected".
 */
export function LiveProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status | null>(null)
  const [connecting, setConnecting] = useState(true)

  useEffect(() => {
    let closed = false
    let fallbackTimer: ReturnType<typeof setInterval> | undefined

    const es = new EventSource("/api/events")
    es.addEventListener("status", (ev) => {
      try {
        setStatus(JSON.parse((ev as MessageEvent).data) as Status)
        setConnecting(false)
      } catch {
        /* malformed frame — ignore */
      }
    })
    es.onerror = () => {
      setConnecting(true)
      // Bridge until SSE returns: gentle polling keeps UI honest offline.
      if (!fallbackTimer && !closed) {
        fallbackTimer = setInterval(() => {
          fetch("/api/status")
            .then((r) => (r.ok ? r.json() : null))
            .then((d) => {
              if (d) {
                setStatus(d as Status)
                setConnecting(false)
              }
            })
            .catch(() => {})
        }, 5000)
      }
    }
    es.onopen = () => {
      if (fallbackTimer) {
        clearInterval(fallbackTimer)
        fallbackTimer = undefined
      }
    }

    return () => {
      closed = true
      if (fallbackTimer) clearInterval(fallbackTimer)
      es.close()
    }
  }, [])

  return (
    <LiveContext.Provider value={{ status, connecting }}>
      {children}
    </LiveContext.Provider>
  )
}

/** Read the live server snapshot from anywhere. */
export function useLive() {
  return useContext(LiveContext)
}
