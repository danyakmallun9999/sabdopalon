import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Tool version strings come from CLIs that colour their output (Composer
// prints "\x1b[32mComposer\x1b[39m ..."). The escape codes are invisible in a
// terminal but show up as garbage in the dashboard, so they are stripped
// before the value is rendered.
const ANSI = /\u001B\[[0-9;]*[A-Za-z]/g

export function stripAnsi(value: string): string {
  return value.replace(ANSI, "")
}
