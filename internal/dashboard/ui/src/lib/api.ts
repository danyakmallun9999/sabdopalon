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
  db_states?: Record<string, boolean>
  db_errors?: Record<string, string>
  php?: string
  sites_count: number
  services: boolean
  php_version?: string
  /** Live public-tunnel state per site, keyed by site name. */
  tunnels?: Record<string, TunnelState>
  /** Whether the tunnel feature can run at all (is cloudflared present?). */
  cloudflared?: { installed: boolean; path: string; version: string; install_hint?: string }
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
  php_ini: string
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
  active: boolean
}

export type InstallJob = {
  name: string
  running: boolean
  done: boolean
  output: string
  error?: string
}

// CreateJob is the state of a site being scaffolded: the template's own output
// as it arrives, plus where the site will be reachable once it is done.
//
// `output` is the readable form — terminal colour codes, carriage-return
// redraws and composer's one-line-per-package noise removed. `output_raw` is
// what the tool actually wrote, for when something needs diagnosing.
export type CreateJob = {
  running: boolean
  done: boolean
  name: string
  template: string
  url: string
  error: string
  // note is a successful creation with a caveat — typically a hosts entry that
  // needs administrator rights. It is not an error: the files exist.
  note?: string
  output: string
  output_raw: string
  started: string
  finished: string
}

// System tools (Node.js, Composer) live on the user's system, not in bin/.
export type SysTool = {
  name: string
  label: string
  installed: boolean
  version: string
}

export type SysToolJob = {
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

// HostsState is the read-only report behind Settings → Local DNS: what the
// managed hosts block maps right now, and what the active TLD needs.
export type HostsState = {
  tld: string
  path: string
  self_resolving: boolean
  managed: boolean
  wanted: string[]
  current: string[]
  missing: string[]
  stale: string[]
  ok: boolean
  can_elevate: boolean
  error?: string
}

// HostsElevation acknowledges the synchronize button. `started` means a UAC
// prompt was raised; the page then polls hostsState() until the block lands.
export type HostsElevation = {
  ok: boolean
  started: boolean
  message: string
}

export type ConfigPayload = {
  tld?: string
  http_port?: number
  https_port?: number
  lan?: boolean
  db_engine?: string
  db_port?: number
  db_mariadb_enabled?: boolean
  db_mariadb_port?: number
  db_pg_enabled?: boolean
  db_pg_port?: number
  db_installed?: Record<string, boolean>
  db_running?: boolean
  db_states?: Record<string, boolean>
  db_errors?: Record<string, string>
  dashboard_enabled?: boolean
  dashboard_port?: number
  auto_open?: boolean
  mailpit_enabled?: boolean
}

export type PhpIni = {
  path: string
  content: string
  version?: string
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

export type DevToolStatus = {
  tool: string
  label: string
  running: boolean
  port?: number
  pid?: number
  log_file?: string
  last_error?: string
}

export type SiteDetail = {
  name: string
  url: string
  https: string
  dir: string
  running: boolean
  port?: number
  framework: string
  php: {
    binary: string
    version?: string
  }
  config: SiteConfigPayload
  // Optional: an older sidecar answered with null for "no tools"/"no logs yet",
  // so page code must not assume an array is present.
  devtools?: DevToolStatus[] | null
  logs?: string[] | null
}

export type TunnelStatus = "off" | "starting" | "online" | "error"

export type TunnelState = {
  enabled: boolean
  status: TunnelStatus
  url: string
  allowed_mail: string[]
  error: string
  started_at: string
}

export type SetupComponent = {
  key: string
  label: string
  version?: string
  installed: boolean
}

export type SetupTool = {
  key: string
  label: string
  description: string
  installed: boolean
}

export type SetupStatus = {
  bootstrapped: boolean
  dirs_ok: boolean
  db_engine: string
  root_dir: string
  components?: SetupComponent[]
  tools?: SetupTool[]
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
  // Update check. Reading the last result never touches the network; checking is
  // an explicit action, and nothing here installs anything.
  update: () =>
    request<{
      current: string
      latest?: string
      available: boolean
      kind?: string
      notes?: string
      url?: string
      size?: number
      published_at?: string
      checked_at?: string
      channel?: string
      error?: string
    }>("/api/update"),
  // The channel rides along with the check: it is the browser's preference, and the
  // endpoint records which one produced the answer.
  checkUpdate: (channel?: string) =>
    post<{
      current: string
      latest?: string
      available: boolean
      kind?: string
      notes?: string
      url?: string
      size?: number
      published_at?: string
      checked_at?: string
      channel?: string
      error?: string
    }>(`/api/update/check${channel === "beta" ? "?channel=beta" : ""}`, {}),
  // The download job: POST starts it, GET reports progress. Same shape as the
  // Packages page's job, so polling is already a familiar pattern here.
  downloadUpdate: () =>
    post<{ running: boolean; done: number; total: number; path?: string; error?: string }>(
      "/api/update/download",
      {},
    ),
  downloadState: () =>
    request<{ running: boolean; done: number; total: number; path?: string; error?: string }>(
      "/api/update/download",
    ),
  // About is a snapshot: identity, paths, credits, and the outbound-host list.
  about: () =>
    request<{
      app: {
        version: string
        channel: string
        platform: string
        arch: string
        runtime: string
        author: string
        repo: string
      }
      paths: Record<string, string>
      logs?: { dir: string; files: number; bytes: number; newest: string }
      privacy: { host: string; why: string }[]
      error?: string
    }>("/api/about"),
  // Creating a site is a job, not one long request: a Laravel scaffold runs
  // composer (and npm, for a starter kit) for minutes, and the dialog shows the
  // log it produces instead of a button that only says "Creating…".
  createSite: (name: string, template: string, database?: string, testing?: string) =>
    post<{ ok: boolean; name: string; job?: CreateJob; message?: string; error?: string }>(
      "/api/sites",
      { name, template, database, testing },
    ),
  createJob: () => request<CreateJob>("/api/sites/job"),
  siteAction: (name: string, action: "start" | "stop" | "restart") =>
    post<{ ok: boolean; port?: number; was_running?: boolean; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/${action}`,
    ),
  siteConfig: (name: string) =>
    request<SiteConfigPayload>(`/api/sites/${encodeURIComponent(name)}/config`),
  saveSiteConfig: (name: string, payload: SiteConfigPayload) =>
    request<{ ok: boolean; message: string; restarted: boolean; php?: string; warning?: string; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/config`,
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) },
    ),
  getSitePhpIni: (name: string) =>
    request<{ content: string; exists: boolean }>(`/api/sites/${encodeURIComponent(name)}/phpini`),
  saveSitePhpIni: (name: string, content: string) =>
    request<{ ok: boolean; message: string; restarted: boolean; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/phpini`,
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ content }) },
    ),
  deleteSite: (name: string) =>
    request<{ ok: boolean; message?: string; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}`,
      { method: "DELETE" },
    ),

  listPackages: () => request<Package[]>("/api/packages"),
  // null, not just [], is possible from a sidecar built before the list was made
  // to marshal as an array: consuming it with .forEach threw and emptied the PHP
  // version dropdown.
  systemPHPs: () => request<SystemPHP[] | null>("/api/php/system"),
  installPackage: (name: string) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/packages/install", { name }),
  installJob: () => request<InstallJob>("/api/packages/job"),

  // System tools (Node.js, Composer) — installed onto the user's system.
  listSysTools: () => request<SysTool[]>("/api/sys-tools"),
  installSysTool: (name: string) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/sys-tools/install", { name }),
  sysToolJob: () => request<SysToolJob>("/api/sys-tools/job"),

  sslStatus: () => request<SslStatus>("/api/ssl"),
  sslCa: () => post<SslAction>("/api/ssl/ca"),
  sslWildcard: () => post<SslAction>("/api/ssl/wildcard"),
  sslTrust: () => post<SslAction>("/api/ssl/trust"),

  // Local DNS: the hosts file a custom TLD needs. Reading the state never asks
  // for rights; the POST raises one UAC prompt on Windows, and refuses when the
  // file is already in step (an approval that fixes nothing teaches people to
  // click Yes blindly).
  hostsState: () => request<HostsState>("/api/hosts"),
  hostsElevate: () => post<HostsElevation>("/api/hosts/elevate"),

  getConfig: () => request<ConfigPayload>("/api/config"),
  saveConfig: (payload: ConfigPayload) =>
    request<{ ok: boolean; message: string; restart_required: boolean; error?: string }>(
      "/api/config",
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) },
    ),

  getPhpIni: () => request<PhpIni>("/api/php-ini"),
  savePhpIni: (content: string) =>
    request<{ ok: boolean; message: string; restarted: number; error?: string }>(
      "/api/php-ini",
      { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ content }) },
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

  siteDetail: (name: string) =>
    request<SiteDetail>(`/api/sites/${encodeURIComponent(name)}`),
  siteLogs: (name: string, source: string) =>
    request<LogResponse>(`/api/sites/${encodeURIComponent(name)}/logs?log=${encodeURIComponent(source)}`),
  siteDevTools: (name: string) =>
    request<{ tools: DevToolStatus[] }>(`/api/sites/${encodeURIComponent(name)}/devtools`),
  siteDevToolAction: (name: string, tool: string, action: "start" | "stop") =>
    post<{ ok?: boolean; port?: number; error?: string }>(
      `/api/sites/${encodeURIComponent(name)}/devtools`,
      { tool, action },
    ),

  // Public sharing via a Cloudflare quick tunnel: one endpoint, three verbs.
  siteTunnel: (name: string) =>
    request<TunnelState>(`/api/sites/${encodeURIComponent(name)}/tunnel`),
  startSiteTunnel: (name: string, allowedMail: string[]) =>
    post<TunnelState>(`/api/sites/${encodeURIComponent(name)}/tunnel`, { allowed_mail: allowedMail }),
  stopSiteTunnel: (name: string) =>
    request<TunnelState>(`/api/sites/${encodeURIComponent(name)}/tunnel`, { method: "DELETE" }),

  setupStatus: () => request<SetupStatus>("/api/setup/status"),
  runSetup: (payload: Record<string, unknown>) =>
    post<{ ok: boolean; message?: string; error?: string }>("/api/setup", payload),
  setupJob: () => request<SetupJob>("/api/setup/job"),

  traffic: () => request<TrafficStats>("/api/stats/traffic"),

  databaseControl: (engine: "mariadb" | "postgresql", action: "start" | "stop" | "restart") =>
    post<{ ok: boolean; engine: string; db_states?: Record<string, boolean>; message?: string; error?: string }>(
      `/api/database/${engine}/${action}`,
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
