import { Card } from "@/components/ui/card"

export type Metric = {
  label: string
  value: string
  /** The small line under the number: a caption, or a row of dots and names. */
  sub?: React.ReactNode
  icon?: React.ComponentType<{ className?: string }>
  /** Makes the whole cell a control (the dashboard's database cell opens it). */
  onClick?: () => void
  title?: string
}

/**
 * The metric strip.
 *
 * In the reference design the four headline numbers are one card, not four:
 * a single panel with a 1px divider between the cells, so the numbers read as
 * one fact about the machine rather than four unrelated cards. The divider is
 * a 1px gap over the divider colour, which keeps it correct at any column
 * count.
 */
export function MetricStrip({ metrics }: { metrics: Metric[] }) {
  return (
    <Card padded={false}>
      <div className="grid grid-cols-1 gap-px bg-ring-inner sm:grid-cols-2 lg:grid-cols-4">
        {metrics.map((m) => {
          const cell = (
            <div className="flex flex-col gap-3.5 bg-card p-[18px_22px]">
              <div className="flex items-center gap-2.5">
                <span className="text-[11.5px] font-medium text-text-secondary">
                  {m.label}
                </span>
                {m.icon ? (
                  <m.icon className="ml-auto size-3.5 text-text-muted" />
                ) : null}
              </div>
              <div className="text-[26px] leading-none font-semibold tracking-[-0.02em] text-text-primary tabular-nums">
                {m.value}
              </div>
              {m.sub ? (
                <div className="text-[11px] text-text-muted">{m.sub}</div>
              ) : null}
            </div>
          )

          return m.onClick ? (
            <button
              key={m.label}
              type="button"
              onClick={m.onClick}
              title={m.title}
              className="cursor-pointer text-left transition-colors hover:bg-surface-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40 focus-visible:ring-inset"
            >
              {cell}
            </button>
          ) : (
            <div key={m.label}>{cell}</div>
          )
        })}
      </div>
    </Card>
  )
}
