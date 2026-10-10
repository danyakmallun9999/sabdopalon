import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { BrowserRouter } from "react-router-dom"

import "./index.css"
import App from "./App.tsx"
import ErrorBoundary from "@/components/error-boundary"
import { TooltipProvider } from "@/components/ui/tooltip"
import { inShell, openExternal } from "@/lib/external"

// In the desktop shell every external link has to leave the webview: it has no
// navigation bar, so target="_blank" leads nowhere. Routing the click through
// openExternal is what makes the site URLs in the dashboard real links.
//
// This used to test window.__TAURI__.shell.open, which does not exist in Tauri
// v2 (withGlobalTauri exposes the CORE api; plugin apis arrive with their own
// injected script) — so the check quietly matched nothing and the click fell
// through to a navigation the webview cannot perform.
//
// The listener is registered in the CAPTURE phase on purpose. React 17+ attaches
// its own listener at the root container, and a synthetic stopPropagation() also
// calls the native one — so a bubble-phase listener here never sees clicks on
// links inside a cell that stops propagation to keep the row from navigating
// (the Sites table's URL column). Capture runs before all of that, so the anchor
// opens regardless; the Tauri plugin's own window listener then sees
// defaultPrevented and stands down, so nothing opens twice.
document.addEventListener(
  "click",
  (e) => {
    if (!inShell()) return // plain browser: the anchor's own new-tab behaviour is right
    const anchor = (e.target as HTMLElement)?.closest?.("a[target='_blank']") as HTMLAnchorElement | null
    if (!anchor) return
    const url = anchor.href
    if (!url || url.startsWith("javascript:")) return
    e.preventDefault() // never let the webview attempt a navigation it cannot do
    void openExternal(url)
  },
  true,
)

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <BrowserRouter>
        <TooltipProvider>
          <App />
        </TooltipProvider>
      </BrowserRouter>
    </ErrorBoundary>
  </StrictMode>,
)
