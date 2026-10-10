import { LoaderCircle } from "lucide-react"

import { cn } from "@/lib/utils"

/**
 * The one loading indicator.
 *
 * It carries role="status" and a name, so assistive tech announces it — the
 * plain "Loading…" paragraphs it replaces were rendered as ordinary text, and
 * the same state was sometimes a skeleton instead. Skeletons stay for content
 * whose shape is known (cards, tables); this is for "something is happening,
 * shape unknown".
 */
function Spinner({ className, ...props }: React.ComponentProps<"svg">) {
  return (
    <LoaderCircle
      data-slot="spinner"
      role="status"
      aria-label="Loading"
      className={cn("text-muted-foreground size-4 animate-spin", className)}
      {...props}
    />
  )
}

export { Spinner }
