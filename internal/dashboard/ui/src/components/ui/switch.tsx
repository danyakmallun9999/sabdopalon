import { Switch as SwitchPrimitive } from "@base-ui/react/switch"

import { cn } from "@/lib/utils"

function Switch({
  className,
  size = "default",
  ...props
}: SwitchPrimitive.Root.Props & {
  size?: "sm" | "default"
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "peer group/switch relative inline-flex shrink-0 items-center rounded-full border border-ring-inner p-[3px] transition-colors outline-none after:absolute after:-inset-x-2 after:-inset-y-2 focus-visible:border-brand/70 focus-visible:ring-[3px] focus-visible:ring-brand/20 aria-invalid:border-destructive aria-invalid:ring-[3px] aria-invalid:ring-destructive/15 data-[size=default]:h-5 data-[size=default]:w-9 data-[size=sm]:h-[17px] data-[size=sm]:w-[30px] data-checked:bg-brand-fill data-unchecked:bg-surface-raised data-disabled:cursor-not-allowed data-disabled:opacity-45",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block rounded-full bg-text-muted transition-transform group-data-[size=default]/switch:size-3.5 group-data-[size=sm]/switch:size-[11px] group-data-[size=default]/switch:data-checked:translate-x-4 group-data-[size=sm]/switch:data-checked:translate-x-[13px] group-data-[size=default]/switch:data-unchecked:translate-x-0 group-data-[size=sm]/switch:data-unchecked:translate-x-0 data-checked:bg-brand-fg"
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
