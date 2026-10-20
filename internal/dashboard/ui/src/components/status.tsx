import type { ComponentProps } from "react"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

/**
 * The single definition of what a status looks like.
 *
 * Before this, "running" was emerald-500 in one file and emerald-400 behind a
 * dark: variant in another, drawn at three different dot sizes across eight
 * files — forty hard-coded palette classes with no shared meaning, so the same
 * state could look different on two pages.
 */
export type DotStatus = "running" | "connecting" | "stopped" | "error"

const DOT: Record<DotStatus, string> = {
  running: "bg-success",
  // Pulsing is the established cue for "working on it" (a site starting, the
  // shell connecting, an engine booting), so it belongs to the semantic rather
  // than being re-decided per page.
  connecting: "bg-warning animate-pulse",
  stopped: "bg-muted-foreground/50",
  error: "bg-destructive",
}

/**
 * StatusDot is the colour half of a status and never the whole message: always
 * pair it with a label (see ToneBadge) or an aria-label, because a coloured dot
 * on its own tells nobody who cannot see the colour.
 */
export function StatusDot({
  status,
  className,
  ...props
}: { status: DotStatus } & ComponentProps<"span">) {
  // Hidden from assistive tech ONLY when the caller gives it a name it can
  // already announce (aria-label/title) or when a label sits next to it.
  const labelled = props["aria-label"] != null || props.title != null
  return (
    <span
      aria-hidden={labelled ? undefined : true}
      className={cn("size-1.5 shrink-0 rounded-full", DOT[status], className)}
      {...props}
    />
  )
}

export type Tone = "success" | "warning" | "danger" | "muted"

const TONE: Record<Tone, string> = {
  success: "border-success/20 bg-success-soft text-success",
  warning: "border-warning/20 bg-warning-soft text-warning",
  danger: "border-destructive/20 bg-danger-soft text-destructive",
  muted: "border-ring-inner bg-surface-raised text-text-secondary",
}

/**
 * ToneBadge is a Badge with a semantic tone instead of a palette colour, so
 * every state chip in the app ends up with the same shape, border and tint.
 * Tones read from the --success/--warning/--destructive tokens.
 */
export function ToneBadge({
  tone,
  className,
  children,
  ...props
}: ComponentProps<typeof Badge> & { tone: Tone }) {
  return (
    <Badge variant="outline" className={cn("h-[22px] gap-1.5 border px-2.5 text-[13.5px]", TONE[tone], className)} {...props}>
      {children}
    </Badge>
  )
}
