import { createContext, useContext } from "react"

import type { Status } from "@/lib/api"

export type LiveState = {
  /** Canonical server snapshot — THE source every page renders from. */
  status: Status | null
  /** true while no snapshot has arrived yet (first paint / reconnecting). */
  connecting: boolean
}

export const LiveContext = createContext<LiveState>({ status: null, connecting: true })

/** Read the live server snapshot from anywhere. */
export function useLive() {
  return useContext(LiveContext)
}
