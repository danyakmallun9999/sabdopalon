// Typed client for the Sabdopalon JSON API (served by the Go backend).

export type Status = {
  version: string
  uptime: string
  http_port: number
  https_port: number
  low_ports: boolean
  tld: string
  database: string
  db_running?: boolean
  php?: string
  sites_count: number
  services: boolean
  php_version?: string
}

export type ServiceStatus = {
  name: string
  label: string
  installed: boolean
  running: boolean
  enabled: boolean
  ui?: string
  ports?: string[]
  env_keys?: string[]
  hint?: string
  last_error?: string
}

export type SystemPHP = {
  path: string
  version: string
  active: boolean
}

export type SiteConfigPayload = {
  php: string
  docroot: string
  aliases: string[]
  env: Record<string, string>
  running?: boolean
}

export type Site = {
  name: string
  url: string
  https: string
  dir?: string
  running: boolean
  php: string
  docroot: string
  aliases: string[]
}

export type Package = {
  name: string
  version: string
  short: string
  license: string
  installed: boolean
  is_php: boolean
}

export type InstallJob = {
  name: string
  running: boolean
  done: boolean
  output: string
  error?: string
}

export type SslStatus = {
  ca_exists: boolean
  wildcard_cert: boolean
  installed: boolean
  fingerprint_match: boolean
  source?: "system" | "user"
  detail?: string
  https_port: number
  tld: string
}

export type SslAction = {
  ok: boolean
  needs_su?: boolean
  message?: string
  command?: string
  note?: string
  error?: string
}

export type ConfigPayload = {
  tld?: string
  http_port?: number
  https_port?: number
  db_engine?: string
  db_port?: number
  dashboard_enabled?: boolean
  dashboard_port?: number
  auto_open?: boolean
  mailpit_enabled?: boolean
}

export type Profile = {
  Name: string
  PHP: string
  DBEngine: string
  Description: string
}

export type Backup = {
  name: string
  size: number
  time: string
}

export type MailpitStatus = {
  installed: boolean
  running: boolean
  ui?: string
  smtp?: string
}

export type LogResponse = {
  file: string
  lines: string[]
  count: number
  error?: string
}

export type SetupStatus = {
  bootstrapped: boolean
  dirs_ok: boolean
  php_installed: boolean
  db_engine: string
  root_dir: string
}

export type SetupJob = {
  running: boolean
  done: boolean
  output: string
  error?: string
}

export type TrafficPoint = {
  t: string
  requests: number
}

export type TrafficStats = {
  total: number
  per_minute: TrafficPoint[]
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  const data = (await res.json()) as T & { error?: string }
  if (!res.ok && !data.error) data.error = `HTTP ${res.status}`
  return data
}

function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body ?? {}),
  })
}

const api = {
  status: () => request<Status>("/api/status"),

  listSites: () => request<Site[]>("/api/sites"),
  createSite: (name: string, template: string) =>
    post<{ ok: boolean; name: string; url: string; message?: string; error?: string }>(
      "/api/sites",
      { name, template },
    ),
  siteAction: (name: string, action: "start" | "stop" | "restart") =>
    post<{ ok: boolean; port?: number; was_running?: boolean; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/${action}`,
    ),
  siteConfig: (name: string) =>
    request<SiteConfigPayload>(`/api/sites/${encodeURIComponent(name)}/config`),
  saveSiteConfig: (name: string, payload: SiteConfigPayload) =>
    request<{ ok: boolean; message: string; restarted: boolean; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/config`,
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) },
    ),
  deleteSite: (name: string) =>
    request<{ ok: boolean; message?: string; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}`,
      { method: "DELETE" },
    ),

  listPackages: () => request<Package[]>("/api/packages"),
  systemPHPs: () => request<SystemPHP[]>("/api/php/system"),
  installPackage: (name: string) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/packages/install", { name }),
  installJob: () => request<InstallJob>("/api/packages/job"),

  sslStatus: () => request<SslStatus>("/api/ssl"),
  sslCa: () => post<SslAction>("/api/ssl/ca"),
  sslWildcard: () => post<SslAction>("/api/ssl/wildcard"),
  sslTrust: () => post<SslAction>("/api/ssl/trust"),

  getConfig: () => request<ConfigPayload>("/api/config"),
  saveConfig: (payload: ConfigPayload) =>
    request<{ ok: boolean; message: string; restart_required: boolean; error?: string }>(
      "/api/config",
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) },
    ),

  listProfiles: () => request<Profile[]>("/api/profiles"),
  applyProfile: (name: string) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/profiles/apply", { name }),

  services: () =>
    request<{ services: ServiceStatus[] }>("/api/services"),
  toggleService: (name: string, enabled: boolean) =>
    post<{ ok: boolean; message?: string; status?: ServiceStatus; error?: string }>(
      `/api/services/${encodeURIComponent(name)}/toggle`,
      { enabled },
    ),
  startService: (name: string) =>
    post<{ ok: boolean; message?: string; status?: ServiceStatus; error?: string }>(
      `/api/services/${encodeURIComponent(name)}/start`,
    ),
  stopService: (name: string) =>
    post<{ ok: boolean; message?: string; status?: ServiceStatus; error?: string }>(
      `/api/services/${encodeURIComponent(name)}/stop`,
    ),

  backupNow: () =>
    post<{ backup: string; pruned: number; message?: string; error?: string }>("/api/backup"),
  listBackups: () => request<Backup[]>("/api/backups"),

  logs: (name: string) => request<LogResponse>(`/api/logs/${encodeURIComponent(name)}`),

  setupStatus: () => request<SetupStatus>("/api/setup/status"),
  runSetup: (payload: Record<string, unknown>) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/setup", payload),
  setupJob: () => request<SetupJob>("/api/setup/job"),

  traffic: () => request<TrafficStats>("/api/stats/traffic"),

  databaseControl: (action: "start" | "stop" | "restart") =>
    post<{ ok: boolean; engine: string; db_running: boolean; message?: string; error?: string }>(
      `/api/database/${action}`,
    ),
}

export default api

// Small helper for periodic polling with immediate first call.
// Anti-overlap: if the previous call is still in flight, the next tick is
// skipped so slow responses never pile up (fixes janky intervals).
export function poll(fn: () => void | Promise<void>, ms: number) {
  let running = false
  const tick = () => {
    if (running) return
    running = true
    Promise.resolve(fn()).finally(() => {
      running = false
    })
  }
  tick()
  return setInterval(tick, ms)
}
