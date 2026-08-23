import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { BrowserRouter } from "react-router-dom"

import "./index.css"
import App from "./App.tsx"
import ErrorBoundary from "@/components/error-boundary"
import { TooltipProvider } from "@/components/ui/tooltip"

// In the Tauri desktop shell, links to external/localhost URLs must open in
// the OS browser (the webview has no navigation bar). Intercept clicks on
// target="_blank" anchors and route them through the Tauri opener when
// available; otherwise fall back to window.open (plain browser behaviour).
document.addEventListener("click", (e) => {
  const anchor = (e.target as HTMLElement)?.closest?.("a[target='_blank']") as HTMLAnchorElement | null
  if (!anchor) return
  const url = anchor.href
  if (!url || url.startsWith("javascript:")) return
  const tauri = (window as unknown as { __TAURI__?: { shell?: { open?: (u: string) => Promise<void> } } }).__TAURI__
  if (tauri?.shell?.open) {
    e.preventDefault()
    tauri.shell.open(url).catch(() => window.open(url, "_blank"))
  }
})

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
