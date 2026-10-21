import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"

/* eslint-disable react-refresh/only-export-components -- a provider and the
   two hooks that talk to it belong together; splitting them would put the
   context in a file of its own for no gain. */

type PageRefreshValue = {
  /** A page registers its refresh action here; null takes it back. */
  setRefresh: (fn: (() => void) | null) => void
  /** True while a page has something to refresh. */
  canRefresh: boolean
  /** Runs the registered action. */
  refresh: () => void
}

const PageRefreshContext = createContext<PageRefreshValue | null>(null)

/**
 * The top bar's refresh button.
 *
 * A page that has something to reload (the dashboard, with its services and
 * traffic) registers an action; the bar shows a refresh button for as long as
 * one is registered. The button lives in the bar because a refresh belongs to
 * the page you are looking at, and the bar already knows which page that is.
 */
export function PageRefreshProvider({ children }: { children: ReactNode }) {
  const handlerRef = useRef<(() => void) | null>(null)
  const [canRefresh, setCanRefresh] = useState(false)

  const setRefresh = useCallback((fn: (() => void) | null) => {
    handlerRef.current = fn
    setCanRefresh(fn !== null)
  }, [])

  const refresh = useCallback(() => handlerRef.current?.(), [])

  const value = useMemo(
    () => ({ setRefresh, canRefresh, refresh }),
    [setRefresh, canRefresh, refresh]
  )

  return (
    <PageRefreshContext.Provider value={value}>
      {children}
    </PageRefreshContext.Provider>
  )
}

/** Called by a page that wants the top bar to offer a refresh. */
export function usePageRefresh(action: () => void) {
  const ctx = useContext(PageRefreshContext)
  const actionRef = useRef(action)
  const setRefresh = ctx?.setRefresh

  useEffect(() => {
    actionRef.current = action
  })

  useEffect(() => {
    if (!setRefresh) return
    setRefresh(() => actionRef.current())
    return () => setRefresh(null)
    // setRefresh is stable, so registering once is enough.
  }, [setRefresh])
}

/** Called by the top bar to render the refresh button. */
export function usePageRefreshState() {
  const ctx = useContext(PageRefreshContext)
  return {
    canRefresh: ctx?.canRefresh ?? false,
    refresh: ctx?.refresh ?? (() => {}),
  }
}
