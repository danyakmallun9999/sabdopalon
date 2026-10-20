import * as React from "react"
import { Input as InputPrimitive } from "@base-ui/react/input"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-[34px] w-full min-w-0 rounded-md border border-ring-inner bg-surface-inset px-2.5 py-1 text-[16px] text-text-primary transition-colors outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-[13.5px] file:font-medium file:text-text-primary placeholder:text-text-muted focus-visible:border-brand/70 focus-visible:ring-[3px] focus-visible:ring-brand/15 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 md:text-[13.5px]",
        className
      )}
      {...props}
    />
  )
}

export { Input }
