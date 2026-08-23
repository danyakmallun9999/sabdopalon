import { Component, type ErrorInfo, type ReactNode } from "react"

// ErrorBoundary keeps one crashing page from taking down the whole app:
// without it a render exception unmounts the entire React tree and the user
// is left staring at a blank (black) window with no clue what happened.
type Props = { children: ReactNode }
type State = { error: Error | null }

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Unhandled UI error:", error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-dvh flex-col items-center justify-center gap-4 p-8 text-center">
          <h1 className="text-lg font-semibold">Terjadi kesalahan pada antarmuka</h1>
          <pre className="max-w-xl overflow-auto rounded-lg border bg-muted/40 p-4 text-left font-mono text-xs whitespace-pre-wrap">
            {this.state.error.message}
          </pre>
          <div className="flex gap-2">
            <button
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
              onClick={() => this.setState({ error: null })}
            >
              Coba lagi
            </button>
            <button
              className="rounded-md border px-4 py-2 text-sm font-medium"
              onClick={() => location.reload()}
            >
              Muat ulang
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
