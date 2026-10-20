import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"

import { cn } from "@/lib/utils"

type Crumb = { label: string; to?: string }

/**
 * The one page heading.
 *
 * The old shell put the page title in the top bar, which meant every page
 * inherited the same 48px strip and had nowhere to put its own description or
 * primary action. The title now lives with the content, matching the new
 * reference, while the top bar stays a global status strip.
 */
export function PageHeader({
  title,
  description,
  actions,
  breadcrumb,
  className,
}: {
  title: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  breadcrumb?: Crumb[]
  className?: string
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-end justify-between gap-x-6 gap-y-3",
        className
      )}
    >
      <div className="flex min-w-0 flex-col gap-1">
        {breadcrumb?.length ? (
          <nav
            aria-label="Breadcrumb"
            className="flex items-center gap-1.5 text-[11px] text-text-muted"
          >
            {breadcrumb.map((crumb, i) => (
              <span key={`${crumb.label}-${i}`} className="inline-flex items-center gap-1.5">
                {i > 0 ? <ChevronRight className="size-3 text-text-muted/60" /> : null}
                {crumb.to ? (
                  <Link
                    to={crumb.to}
                    className="transition-colors hover:text-text-primary"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span>{crumb.label}</span>
                )}
              </span>
            ))}
          </nav>
        ) : null}
        <h1 className="text-[26px] leading-[1.15] font-semibold tracking-[-0.025em] text-text-primary">
          {title}
        </h1>
        {description ? (
          <div className="max-w-2xl text-xs leading-relaxed text-text-secondary">
            {description}
          </div>
        ) : null}
      </div>
      {actions ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>
      ) : null}
    </div>
  )
}
