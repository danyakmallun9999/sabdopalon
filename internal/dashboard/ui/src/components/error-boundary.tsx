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
          <h1 className="text-[20px] font-semibold tracking-[-0.02em] text-text-primary">Something went wrong</h1>
          <pre className="max-w-xl overflow-auto rounded-xl border border-ring-inner bg-surface-inset p-4 text-left font-mono text-[13.5px] whitespace-pre-wrap text-text-secondary">
            {this.state.error.message}
          </pre>
          <div className="flex gap-2">
            <button
              className="rounded-lg border border-ring-inner bg-brand-fill px-4 py-2 text-[13.5px] font-medium text-brand-fg"
              onClick={() => this.setState({ error: null })}
            >
              Try again
            </button>
            <button
              className="rounded-lg border border-ring-inner bg-surface-raised px-4 py-2 text-[13.5px] font-medium text-text-primary"
              onClick={() => location.reload()}
            >
              Reload
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
