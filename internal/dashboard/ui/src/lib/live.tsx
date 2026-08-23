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

const CHANNEL = "sabdopalon-live"
const LOCK = "sabdopalon-live-leader"

type Msg =
  | { type: "status"; payload: Status }
  | { type: "who" }

/**
 * LiveProvider shares ONE canonical server snapshot with the whole app via
 * useLive(). It replaces the per-page status pollers that used to race each
 * other and show disagreeing data.
 *
 * Transport:
 * - Exactly one browser tab ("leader", elected via Web Locks) holds the
 *   SSE connection to /api/events and broadcasts snapshots to the rest.
 * - Followers render broadcast snapshots; if silence exceeds ~8 s (leader
 *   closed, or SSE down), any visible tab temporarily polls /api/status
 *   so the UI stays honest until the stream recovers.
 */
export function LiveProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status | null>(null)
  const [connecting, setConnecting] = useState(true)

  useEffect(() => {
    let closed = false

    const apply = (s: Status) => {
      if (!closed) {
        setStatus(s)
        setConnecting(false)
      }
    }

    // --- followers: receive broadcasts from the leader tab ---
    const chan = "BroadcastChannel" in window ? new BroadcastChannel(CHANNEL) : null
    let lastMsgAt = 0
    let watchdog: ReturnType<typeof setInterval> | undefined
    let gapPoller: ReturnType<typeof setInterval> | undefined

    if (chan)
      chan.onmessage = (ev: MessageEvent<Msg>) => {
        lastMsgAt = Date.now()
        if (ev.data?.type === "status") apply(ev.data.payload)
      }

    const stopGapPoller = () => {
      if (gapPoller) {
        clearInterval(gapPoller)
        gapPoller = undefined
      }
    }
    const startGapPoller = () => {
      if (!gapPoller && !closed) {
        gapPoller = setInterval(() => {
          fetch("/api/status")
            .then((r) => (r.ok ? r.json() : null))
            .then((d) => d && apply(d as Status))
            .catch(() => {})
        }, 5000)
      }
    }

    // Watchdog: no leader chatter while visible → bridge with polling.
    watchdog = setInterval(() => {
      if (closed) return
      if (lastMsgAt === 0 || Date.now() - lastMsgAt > 8000) startGapPoller()
      else stopGapPoller()
    }, 3000)

    // --- leader election + SSE stream ---
    type LockMgr = Navigator & {
      locks?: {
        request: (
          name: string,
          opts: { ifAvailable: boolean },
          cb: (lock: unknown) => Promise<void>,
        ) => Promise<unknown>
      }
    }
    const nav = navigator as LockMgr
    nav.locks
      ?.request(
        LOCK,
        { ifAvailable: true },
        async (lock) => {
          if (!lock || closed) return // another tab leads (or we're gone)
          setConnecting(true)

          const broadcast = (payload: Status) =>
            chan?.postMessage({ type: "status", payload } satisfies Msg)

          const run = () => {
            const es = new EventSource("/api/events")
            es.addEventListener("status", (ev) => {
              try {
                const parsed = JSON.parse((ev as MessageEvent).data) as Status
                lastMsgAt = Date.now()
                apply(parsed)
                broadcast(parsed)
              } catch {
                /* malformed frame — ignore */
              }
            })
            es.onerror = () => {
              setConnecting(true)
              startGapPoller() // EventSource retries internally; we bridge
            }
            es.onopen = stopGapPoller
          }
          run()

          // Hold the lock until this tab closes/unloads.
          await new Promise<void>(() => {})
        },
      )
      .catch(() => {})

    return () => {
      closed = true
      stopGapPoller()
      if (watchdog) clearInterval(watchdog)
      chan?.close()
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
