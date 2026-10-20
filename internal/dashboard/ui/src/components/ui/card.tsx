import * as React from "react"

import { cn } from "@/lib/utils"

/**
 * The page surface.
 *
 * Ring cards are the signature of the new visual language: a 1px outer edge,
 * a 3px gap in the app background, then the actual 1px inner panel. The gap is
 * real padding, not a box-shadow, so it survives overflow containers.
 */
function Card({
  className,
  size = "default",
  variant = "ring",
  padded = true,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  size?: "default" | "sm"
  variant?: "ring" | "flat" | "inset"
  /** When false the panel has no padding of its own — for strips (a row of
   *  metric cells) where each cell brings its own. */
  padded?: boolean
}) {
  const ring = variant === "ring"

  return (
    <div
      data-slot="card"
      data-size={size}
      data-variant={variant}
      className={cn(
        "group/card flex min-h-0 flex-col text-[13px] text-card-foreground",
        ring && "rounded-2xl border border-ring-outer bg-ring-gap p-[3px]",
        className
      )}
      {...props}
    >
      <div
        data-slot="card-panel"
        data-size={size}
        className={cn(
          "flex min-h-0 flex-1 flex-col gap-(--card-gap) overflow-hidden",
          padded ? "py-(--card-pad-y)" : "py-0",
          "[--card-pad-y:18px] [--card-pad-x:22px] [--card-gap:14px]",
          "data-[size=sm]:[--card-pad-y:14px] data-[size=sm]:[--card-pad-x:18px] data-[size=sm]:[--card-gap:10px]",
          ring
            ? "rounded-[14px] border border-ring-inner bg-card"
            : variant === "flat"
              ? "rounded-xl border border-ring-inner bg-card"
              : "rounded-xl border border-ring-inner bg-surface-inset",
          "has-[>img:first-child]:pt-0 *:[img:first-child]:rounded-t-xl *:[img:last-child]:rounded-b-xl"
        )}
      >
        {children}
      </div>
    </div>
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        "group/card-header @container/card-header grid auto-rows-min items-start gap-1.5 px-(--card-pad-x) has-data-[slot=card-action]:grid-cols-[1fr_auto] has-data-[slot=card-description]:grid-rows-[auto_auto] [.border-b]:pb-(--card-pad-y)",
        className
      )}
      {...props}
    />
  )
}

function CardTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-title"
      className={cn(
        "font-heading text-[14.5px] leading-[1.3] font-semibold tracking-[-0.01em] text-text-primary",
        className
      )}
      {...props}
    />
  )
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-description"
      className={cn("text-[12.5px] leading-relaxed text-text-secondary", className)}
      {...props}
    />
  )
}

function CardAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-action"
      className={cn(
        "col-start-2 row-span-2 row-start-1 self-start justify-self-end",
        className
      )}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-content"
      className={cn("px-(--card-pad-x)", className)}
      {...props}
    />
  )
}

function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-footer"
      className={cn(
        "flex items-center px-(--card-pad-x) [.border-t]:pt-(--card-pad-y)",
        className
      )}
      {...props}
    />
  )
}

export {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
}
