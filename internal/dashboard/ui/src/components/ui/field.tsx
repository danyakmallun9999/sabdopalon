import type { ComponentProps } from "react"

import { Label } from "@/components/ui/label"
import { cn } from "@/lib/utils"

/**
 * A form field whose error belongs to the field.
 *
 * Validation used to be a toast: the message appeared far from the input,
 * disappeared after a few seconds, and left the input in its normal state — no
 * aria-invalid, nothing for a screen reader to find later. FieldError renders
 * next to the control and stays until it is fixed; pair it with aria-invalid and
 * aria-describedby on the input (see the setup wizard).
 */
function Field({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="field" className={cn("flex flex-col gap-1.5", className)} {...props} />
}

function FieldLabel({ className, ...props }: ComponentProps<typeof Label>) {
  return <Label data-slot="field-label" className={cn("gap-0", className)} {...props} />
}

function FieldDescription({ className, ...props }: ComponentProps<"p">) {
  return (
    <p
      data-slot="field-description"
      className={cn("text-[11.5px] leading-relaxed text-text-muted", className)}
      {...props}
    />
  )
}

function FieldError({ className, children, ...props }: ComponentProps<"p">) {
  if (!children) return null
  return (
    <p
      data-slot="field-error"
      role="alert"
      className={cn("text-[11.5px] text-destructive", className)}
      {...props}
    >
      {children}
    </p>
  )
}

export { Field, FieldLabel, FieldDescription, FieldError }
