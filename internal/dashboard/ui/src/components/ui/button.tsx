import { Button as ButtonPrimitive } from "@base-ui/react/button"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

/**
 * Buttons in the reference design are two layers: a 1px outer edge, a 2px
 * gap in the app background, then the button surface with its own 1px edge.
 * The gap is drawn with box-shadow so the control stays a single element
 * (focus, disabled and form behaviour are unchanged).
 *
 * The shadow values are written out in full on purpose: Tailwind finds
 * class names by reading the source, and a class assembled from a
 * variable (`"shadow-[" + RING + "]"`) is invisible to it - the ring
 * silently never reaches the stylesheet.
 */
const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg border text-[13px] leading-none font-medium whitespace-nowrap transition-colors outline-none select-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/35 active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-45 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/20 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-[13px]",
  {
    variants: {
      variant: {
        default:
          "border-ring-inner bg-brand-fill font-semibold text-brand-fg shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)] hover:bg-[#a84600]",
        outline:
          "border-ring-inner bg-surface-raised text-text-primary shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)] hover:bg-[#303030]",
        secondary:
          "border-ring-inner bg-surface-raised text-text-primary shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)] hover:bg-[#303030]",
        ghost:
          "border-transparent text-text-secondary hover:bg-surface-raised hover:text-text-primary",
        destructive:
          "border-destructive/25 bg-danger-soft text-destructive shadow-[0_0_0_2px_var(--ring-gap),0_0_0_3px_var(--ring-outer)] hover:bg-destructive/15 focus-visible:border-destructive/40 focus-visible:ring-destructive/20",
        link: "border-transparent text-brand underline-offset-4 hover:underline",
      },
      size: {
        default:
          "h-[30px] px-3.5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        xs: "h-6 gap-1 px-2.5 text-[12px] has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-[26px] gap-1.5 px-3 text-[12px] has-data-[icon=inline-end]:pr-2.5 has-data-[icon=inline-start]:pl-2.5",
        lg: "h-10 px-4.5 text-[13.5px] has-data-[icon=inline-end]:pr-4 has-data-[icon=inline-start]:pl-4",
        icon: "size-[30px]",
        "icon-xs": "size-6 [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-[26px]",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
