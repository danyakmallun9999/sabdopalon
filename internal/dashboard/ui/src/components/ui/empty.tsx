import type { ComponentProps } from "react"

import { cn } from "@/lib/utils"

/**
 * The one empty state.
 *
 * Four different shapes were in use for "there is nothing here": a bare
 * sentence, a dashed box, a card whose title was the sentence, and a centred
 * paragraph — so the same idea looked like four different products. The text
 * stays with the caller: naming the missing thing is all this needs to do.
 */
function Empty({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="empty"
      className={cn(
        "flex flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed p-8 text-center",
        className
      )}
      {...props}
    />
  )
}

function EmptyTitle({ className, ...props }: ComponentProps<"p">) {
  return (
    <p data-slot="empty-title" className={cn("text-sm font-medium", className)} {...props} />
  )
}

function EmptyDescription({ className, ...props }: ComponentProps<"p">) {
  return (
    <p
      data-slot="empty-description"
      className={cn("text-muted-foreground text-xs", className)}
      {...props}
    />
  )
}

export { Empty, EmptyTitle, EmptyDescription }
