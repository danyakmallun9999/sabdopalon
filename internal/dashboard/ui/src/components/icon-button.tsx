import type { ComponentProps, ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

/**
 * An icon-only button that always has a tooltip AND an accessible name.
 *
 * The app used raw `title=` attributes for these: the native tooltip is
 * mouse-only, unstyled, and does not match the design system — and because it
 * was optional, the same kind of control ended up named three different ways
 * (aria-label here, sr-only there, nothing at all elsewhere). Requiring `label`
 * makes an unnamed icon-only control impossible to ship.
 */
export function IconButton({
  label,
  side,
  children,
  ...props
}: {
  label: string
  side?: "top" | "bottom" | "left" | "right"
  children: ReactNode
} & ComponentProps<typeof Button>) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button aria-label={label} {...props}>
            {children}
          </Button>
        }
      />
      <TooltipContent side={side}>{label}</TooltipContent>
    </Tooltip>
  )
}
