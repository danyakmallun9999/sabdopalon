import type { Status } from "@/lib/api"

// dbEngineRunning reports whether the configured DB engine is actually up.
// sqlite needs no daemon; "mysql" maps onto the managed "mariadb" daemon.
// Unknown state (no snapshot yet) counts as NOT running — never fake ✓.
export function dbEngineRunning(status?: Status | null): boolean {
  if (!status?.database) return false
  if (status.database === "sqlite") return true
  const key = status.database === "mysql" ? "mariadb" : status.database
  return Boolean(status.db_states?.[key])
}
