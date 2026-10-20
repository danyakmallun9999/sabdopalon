import { cn } from "@/lib/utils"

/**
 * The row of actions that opens a page.
 *
 * The page title and its one-line description live in the top bar now, so the
 * page itself starts with what you can DO with it. A page with no actions
 * renders nothing at all.
 */
export function PageActions({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-end gap-3.5",
        className
      )}
    >
      {children}
    </div>
  )
}
