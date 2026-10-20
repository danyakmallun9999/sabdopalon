import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "flex field-sizing-fixed min-h-20 w-full resize-none rounded-md border border-ring-inner bg-surface-inset px-2.5 py-2 text-[16px] text-text-primary transition-colors outline-none placeholder:text-text-muted focus-visible:border-brand/70 focus-visible:ring-[3px] focus-visible:ring-brand/15 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 md:text-[13.5px]",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
