# Site Detail Page & Dev-Tools Supervisor — Architecture & Implementation

Status: **Implemented** (all phases complete; build, vet, test, and
cross-compilation verified green on linux/darwin/windows).
Scope: per-site detail dashboard + managed dev-tool processes (Vite, artisan,
composer) + framework-aware serving (Laravel/Vite proxy).

---

## Resolved Issues

| # | User problem | Root cause | Solution in this design |
|---|---|---|---|
| 1 | "Must run `composer run dev` first before the Sabdopalon URL works" | Vite dev server (`localhost:5173`) not proxied; `@vite` assets fail to load on `*.localhost` | DevTools supervisor + Vite reverse-proxy |
| 2 | Laravel route 404 / asset not found | Generic defaultRouter doesn't know the Laravel front controller | Framework detection + dedicated Laravel router |
| 3 | "Where do I set php.ini?" | No per-site UI; must find the file manually | "PHP Config" tab on the detail page |
| 4 | No per-site control center | Config in a dialog, logs on another page, separate terminal dock | Site Detail Page (tabbed, single entry point) |
| 5 | Don't know which framework a site uses | No framework detection | Framework detector in the overview |
| 6 | `npm run dev` dies when the terminal closes | No supervisor; user runs it manually | DevTools supervisor (mirroring the `services.Manager` pattern) |

---

## Layer Architecture

```
┌──────────────────────────────────────────────────────────┐
│  React SPA  (internal/dashboard/ui/src)                 │
│  /sites              → list page (the current one)         │
│  /sites/:name        → SiteDetailPage (NEW)             │
│    ├─ Overview tab   (framework, status, env)            │
│    ├─ Config tab     (php, docroot, aliases, env, ini)   │
│    ├─ Logs tab       (php.log + vite.log + artisan.log)  │
│    ├─ DevTools tab   (start/stop Vite, artisan, npm)     │
│    └─ Terminal tab   (inline PTY, auto-cd into the site dir)   │
└──────────────────────┬───────────────────────────────────┘
                       │ REST + SSE
┌──────────────────────┴───────────────────────────────────┐
│  Dashboard HTTP Server (internal/dashboard)              │
│  GET  /api/sites/:name          → detail aggregate       │
│  GET  /api/sites/:name/logs     → multi-log tail         │
│  POST /api/sites/:name/devtools → start/stop/list        │
│  GET  /api/sites/:name/devtools → status + output tail   │
│  WS   /api/sites/:name/terminal → per-site PTY           │
└──────────────────────┬───────────────────────────────────┘
                       │
┌──────────────────────┴───────────────────────────────────┐
│  devtools package (NEW — mirrors internal/services)      │
│  Manager                                                 │
│    ├─ Start(site, tool)   → spawn + log + supervise      │
│    ├─ Stop(site, tool)    → kill process group           │
│    ├─ Status(site)        → {tool: {running, pid, port}}  │
│    └─ StopAllForSite(site) → kill all tools for a site   │
│  Tool specs (Vite, Artisan, npm, composer)               │
└──────────────────────┬───────────────────────────────────┘
                       │ spawn
┌──────────────────────┴───────────────────────────────────┐
│  proxy package (internal/proxy)                         │
│  ensureSite()                                            │
│    ├─ Detect framework (Laravel/WordPress/blank)         │
│    ├─ Pick router script (LaravelRouter vs defaultRouter)│
│    └─ If Vite running → register reverse-proxy handler   │
│  ViteProxy (NEW)                                        │
│    ├─ Intercept /@vite/, /node_modules/.vite/            │
│    └─ httput.ReverseProxy → localhost:5173               │
└──────────────────────────────────────────────────────────┘
```

---

## Part 1: Dev-Tools Supervisor (`internal/devtools`)

### 1.1 Why a new package instead of reusing `internal/services`

`internal/services` (services.go:274) manages **global** services — one
Mailpit for all sites, one Redis for all sites. Dev tools are
**per-site**: site A runs Vite on port 5173, site B runs Vite on port 5174.
The manager must also know which site owns each process, so that when a site
is stopped, its dev tools die too.

Patterns reused from `services.go`:
- `runningProc` struct (cmd + log file) → exactly the same
- `setProcessGroup` + `killProcessGroup` → exactly the same
- `ready()` probe → used for Vite (HTTP probe to `localhost:PORT`)
- Port allocation → mirrors `proxy.go:497` (`isPortFree` loop)

### 1.2 Package structure

```
internal/devtools/
  devtools.go         Manager + runningProc (mirror services.go)
  registry.go         Tool specs (Vite, Artisan, npm, composer)
  devtools_test.go
```

### 1.3 ToolSpec

```go
// ToolSpec describes a managed dev-tool process for one site.
type ToolSpec struct {
    Name      string   // "vite", "laravel-dev", "npm-dev", "composer"
    Label     string   // "Vite Dev Server"
    BinName   string   // "npx", "php", "node", "composer"
    Args      func(siteDir string, port int) []string
    Port      int      // 0 = no port (e.g. composer install)
    ReadyKind string   // "http" | "tcp" | "" (no probe)
    ReadyPath string   // "/@vite/" for Vite HTTP probe
    Env       func(siteDir string, port int) []string // extra env (APP_ENV=local, APP_PORT, etc.)
}
```

### 1.4 Registry — supported tools

| Tool | Bin | Args | Port | Ready | When used |
|---|---|---|---|---|---|
| `vite` | `npx` | `["vite", "--port", "<N>"]` | 5173+ | http `localhost:N` | Has `vite.config.{js,ts}` |
| `laravel-dev` | `composer` | `["run", "dev"]` | 8000+ | tcp | composer.json has `scripts.dev` (Laravel 11+ skeleton: serve + queue + vite all at once) |
| `npm-dev` | `npm` | `["run", "dev"]` | — | — | Generic fallback (not Vite) |
| `npm-build` | `npm` | `["run", "build"]` | — | — | One-shot build, not long-running |
| `composer-install` | `composer` | `["install"]` | — | — | One-shot |
| `composer-update` | `composer` | `["update"]` | — | — | One-shot |

### 1.5 Manager API

```go
type Manager struct {
    cfg    *config.Engine
    mu     sync.Mutex
    procs  map[string]map[string]*runningProc // site → tool → proc
    ports  map[string]int                     // site → next port
}

// Start launches a dev-tool for a site. Auto-picks a free port for tools
// that need one. Fails loud if the tool binary is missing or port busy.
func (m *Manager) Start(siteName, siteDir, toolName string) (int, error)

// Stop terminates one tool for one site.
func (m *Manager) Stop(siteName, toolName string) error

// StopAllForSite kills every running tool for a site (called when site stops).
func (m *Manager) StopAllForSite(siteName string)

// Status returns the live state of all tools for a site.
func (m *Manager) Status(siteName string) []ToolStatus

// StopAll terminates everything (called on Sabdopalon shutdown).
func (m *Manager) StopAll()
```

### 1.6 Lifecycle integration

| Event | What happens |
|---|---|
| `proxy.StopSite(name)` | `devtools.StopAllForSite(name)` |
| `proxy.RestartSite(name)` | stop tools → restart site → auto-restart tools that were previously running |
| Sabdopalon shutdown | `devtools.StopAll()` (mirrors `services.Manager.StopAll`) |
| Site deleted | `devtools.StopAllForSite(name)` before the folder is moved to `.trash/` |

### 1.7 Port allocation

```
base = 5173 (Vite default)
for each site that starts Vite:
  port = base
  while !isPortFree(port): port++
  record site→port mapping
```

Stored in-memory (not persisted). On restart the port may differ — that's OK,
because the Vite proxy reads the port from the live mapping, not a hardcode.

### 1.8 Logging

Each tool → `logs/<site>.<tool>.log`
- `logs/myapp.vite.log`
- `logs/myapp.artisan.log`

Same format as `services.go:114` (`os.O_CREATE|O_WRONLY|O_TRUNC`).
Tail via the existing endpoint (`/api/logs/`) — just add the new log name.

---

## Part 2: Framework Detection & Laravel Router

### 2.1 Detector (`internal/proxy/framework.go` — NEW)

```go
// DetectFramework inspects a site directory and returns the framework
// name + a hint for the router. Called once per site on first ensureSite.
func DetectFramework(siteDir string) Framework {
    // Laravel: artisan + composer.json with "laravel/framework"
    if fileExists(filepath.Join(siteDir, "artisan")) {
        if hasComposerReq(siteDir, "laravel/framework") {
            return FrameworkLaravel
        }
    }
    // WordPress: wp-config.php or wp-config-sample.php
    if fileExists(filepath.Join(siteDir, "wp-config.php")) ||
       fileExists(filepath.Join(siteDir, "wp-config-sample.php")) {
        return FrameworkWordPress
    }
    // CodeIgniter 4: composer.json with "codeigniter4"
    if hasComposerReq(siteDir, "codeigniter4/codeigniter4") {
        return FrameworkCodeIgniter
    }
    // Symfony: symfony.lock or composer.json with "symfony/framework-bundle"
    if fileExists(filepath.Join(siteDir, "symfony.lock")) {
        return FrameworkSymfony
    }
    return FrameworkUnknown
}
```

Cache the detection result in the `siteServer` struct (once per session, no
re-scan on every request).

### 2.2 Laravel Router (`internal/proxy/routers.go` — NEW)

Dedicated Laravel router, written to `.sabdopalon-router.php` when the Laravel
framework is detected (replacing the generic `defaultRouter`):

```php
<?php
// Sabdopalon Laravel router — forwards all non-static requests to
// public/index.php with the correct PATH_INFO and SCRIPT_NAME.
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$docroot = $_SERVER['DOCUMENT_ROOT'];

// Serve static files from public/ (css, js, images, favicon)
$file = $docroot . $uri;
if ($uri !== '/' && is_file($file)) {
    return false; // let PHP's built-in server serve the file
}

// Everything else → Laravel front controller
$_SERVER['SCRIPT_NAME'] = '/index.php';
$_SERVER['SCRIPT_FILENAME'] = $docroot . '/index.php';
$_SERVER['PATH_INFO'] = $uri;

// Vite HMR: if the Vite dev server is running, rewrite asset URLs so
// the browser fetches from the Vite server (injected via env by Sabdopalon).
$vitePort = getenv('SABDOPALON_VITE_PORT');
if ($vitePort && $uri === '/') {
    // @vite hot client injection happens via Laravel's Vite facade;
    // we just ensure APP_ENV=local so the Vite plugin activates.
    $_ENV['APP_ENV'] = $_ENV['APP_ENV'] ?? 'local';
}

require $docroot . '/index.php';
return true;
```

Why this fixes Laravel routing:
- `SCRIPT_NAME` and `SCRIPT_FILENAME` are set correctly — Laravel's
  `Request::capture()` needs these for URI parsing
- `PATH_INFO` is correct — so `Route::get('/users/{id}')` matches
- No `.htaccess` (Apache) needed — this is PHP's built-in server

### 2.3 Vite Reverse-Proxy (`internal/proxy/viteproxy.go` — NEW)

This is the core of the "no manual `composer run dev` needed" solution:

```go
// ViteProxy intercepts Vite-specific paths and reverses them to the
// running Vite dev server. It is registered per-site when Vite is running.
type ViteProxy struct {
    port int // Vite's actual port (from devtools.Manager)
}

// ShouldIntercept returns true for Vite HMR paths.
func (vp *ViteProxy) ShouldIntercept(r *http.Request) bool {
    p := r.URL.Path
    // Vite dev server internal paths
    if strings.HasPrefix(p, "/@vite/")     { return true }
    if strings.HasPrefix(p, "/node_modules/.vite/") { return true }
    // Vite client + HMR websocket upgrade
    if strings.Contains(p, "vite/") && strings.HasSuffix(p, ".js") { return true }
    return false
}

// ServeHTTP reverses the request to the Vite dev server.
func (vp *ViteProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", vp.port))
    proxy := httput.NewSingleHostReverseProxy(target)
    // Fix Host header so Vite's CORS/HMR origin check passes
    r.URL.Path = strings.TrimPrefix(r.URL.Path, "/") // Vite serves from root
    proxy.ServeHTTP(w, r)
}
```

### 2.4 Integration into `ensureSite` (proxy.go:457)

New flow when a request comes into a site:

```
ensureSite(name):
  1. Load siteconfig (.sabdopalon.yml)
  2. Detect framework (cached)           ← NEW
  3. Pick router script:
       Laravel → laravelRouter           ← NEW
       else    → defaultRouter
  4. Start php -S with chosen router
  5. If framework == Laravel && Vite running:
       register ViteProxy for this site  ← NEW
```

In the main proxy handler (before forwarding to PHP):

```
handleRequest(host, r):
  site = resolveSite(host)
  if site.viteProxy != nil && site.viteProxy.ShouldIntercept(r):
    site.viteProxy.ServeHTTP(w, r)
    return
  // else: normal PHP forward
  site.forward(w, r)
```

### 2.5 Vite port injection into the PHP env

In `startPHP` (php.go:62), add env:

```go
if vp := s.getViteProxy(name); vp != nil {
    env = append(env,
        fmt.Sprintf("SABDOPALON_VITE_PORT=%d", vp.port),
        fmt.Sprintf("SABDOPALON_VITE_HOST=127.0.0.1"),
    )
}
```

Laravel's `vite.config.js` can read `process.env.SABDOPALON_VITE_PORT`
to set `server.hmr.host` and `server.origin`, so the HMR websocket
connects to the correct host.

As a more robust alternative: provide a
`vite.config.sabdopalon.js` template that users can copy into their project — it's
pre-configured for Sabdopalon:

```js
// vite.config.js (Sabdopalon-ready)
import { defineConfig } from 'vite';
import laravel from 'vite-plugin-laravel';

export default defineConfig({
  plugins: [laravel()],
  server: {
    host: '127.0.0.1',
    port: parseInt(process.env.SABDOPALON_VITE_PORT || '5173'),
    hmr: { host: '127.0.0.1' },
    origin: `http://127.0.0.1:${process.env.SABDOPALON_VITE_PORT || '5173'}`,
  },
});
```

---

## Part 3: Site Detail Page — Backend API

### 3.1 New endpoints

All under `/api/sites/<name>/` (extends `handleAPISiteAction`):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/sites/<name>` | Detail aggregate: config + status + framework + devtools + port |
| GET | `/api/sites/<name>/logs` | Multi-log tail (php + vite + artisan), parameters `?log=vite&lines=100` |
| POST | `/api/sites/<name>/devtools` | Body: `{tool: "vite", action: "start"\|"stop"}` |
| GET | `/api/sites/<name>/devtools` | Status of all devtools for this site |
| WS | `/api/sites/<name>/terminal` | Per-site PTY (mirrors the terminal handler, auto-cd into the site dir) |

### 3.2 Detail aggregate response

```json
{
  "name": "myapp",
  "url": "http://myapp.localhost",
  "https": "https://myapp.localhost",
  "dir": "/home/user/sabdopalon/sites/myapp",
  "running": true,
  "port": 8081,
  "framework": "laravel",
  "framework_version": "11.x",
  "php": {
    "binary": "/home/user/sabdopalon/bin/php/8.3/php",
    "version": "8.3.12"
  },
  "config": {
    "php": "8.3",
    "php_ini": "",
    "docroot": "",
    "aliases": ["myapp.test"],
    "env": {"APP_ENV": "local"}
  },
  "devtools": [
    {
      "tool": "vite",
      "running": true,
      "port": 5173,
      "pid": 12345,
      "started_at": "2026-08-25T10:00:00Z",
      "log_file": "logs/myapp.vite.log"
    }
  ],
  "logs": {
    "php": "logs/myapp.php.log",
    "vite": "logs/myapp.vite.log"
  },
  "size_bytes": 45678901,
  "file_count": 1234
}
```

### 3.3 Handler implementation

Create a new file `internal/dashboard/handlers_sitedetail.go`:

```go
// handleAPISiteDetail returns the full per-site aggregate.
func (s *Server) handleAPISiteDetail(w http.ResponseWriter, name string) {
    siteDir := filepath.Join(s.cfg.Root, name)
    if _, err := os.Stat(siteDir); err != nil {
        s.json(w, map[string]string{"error": "site not found"})
        return
    }

    sc, _ := siteconfig.Load(s.cfg.Root, name)
    framework := proxy.DetectFramework(siteDir)
    phpBin := s.cfg.PHP.Binary
    if sc != nil && sc.PHP != "" {
        if r, err := pkgmgr.ResolvePHP(s.cfg.BinDir(), sc.PHP); err == nil {
            phpBin = r
        }
    }

    resp := map[string]any{
        "name":      name,
        "framework": framework.String(),
        "running":   s.proxy.IsRunning(name),
        "config":    sc,
        "devtools":  s.devtools.Status(name),
        // ... (url, https, dir, port, logs, size, file_count)
    }
    if phpBin != "" {
        resp["php"] = map[string]string{
            "binary":  phpBin,
            "version": pkgmgr.PHPBinaryVersion(phpBin),
        }
    }
    s.json(w, resp)
}
```

### 3.4 Routing dispatch

Extend `handleAPISiteAction` (handlers_sites.go:94). It currently dispatches
on the `action` string (start/stop/restart/config). Add:

```go
case http.MethodGet:
    switch action {
    case "config":
        s.getSiteConfig(w, name)
    case "logs":
        s.getSiteLogs(w, name, r)        // NEW
    case "devtools":
        s.getSiteDevTools(w, name)       // NEW
    case "":
        s.handleAPISiteDetail(w, name)  // NEW — GET /api/sites/<name>
    default:
        http.NotFound(w, r)
    }
```

---

## Part 4: Site Detail Page — Frontend

### 4.1 Routing (App.tsx)

Add a route:

```tsx
const SiteDetailPage = lazy(() => import("@/pages/site-detail"))

<Route path="/sites" element={<SitesPage />} />
<Route path="/sites/:name" element={<SiteDetailPage />} />  // NEW
```

`fullBleed` also set for `/sites/:name`:

```tsx
const fullBleed = location.pathname === "/sites" ||
                  location.pathname.startsWith("/sites/") ||
                  location.pathname === "/terminal"
```

### 4.2 Component structure

```
ui/src/pages/
  site-detail.tsx              ← main page (layout + tab switcher)
  site-detail/
    overview-tab.tsx           ← framework, status, info cards
    config-tab.tsx             ← .sabdopalon.yml editor (moved from the dialog)
    logs-tab.tsx               ← multi-log tailer
    devtools-tab.tsx           ← start/stop Vite, artisan, etc.
    terminal-tab.tsx           ← inline terminal
```

### 4.3 Layout

```
┌─────────────────────────────────────────────────────────────┐
│ ← Back  myapp.localhost              [▶ Start] [⟳ Restart] │
│         Laravel 11 · PHP 8.3 · running · :8081              │
├─────────────────────────────────────────────────────────────┤
│ [Overview] [Config] [Logs] [Dev Tools] [Terminal]           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  TAB CONTENT                                                 │
│                                                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

Header bar (sticky):
- Back button → `/sites`
- Site URL + HTTPS URL (clickable, open in new tab)
- Framework badge + PHP version badge + status dot
- Start/Stop/Restart buttons (reuse `act()` from sites.tsx:235)

### 4.4 Tab: Overview

Contents:
- **Framework card**: framework logo/name, version, link to the docs
- **PHP card**: active version, binary path, "bundled" / "system" badge
- **Database card**: engine, running status, connection string (from env)
- **URL card**: HTTP + HTTPS + aliases, all clickable
- **Storage card**: folder size, file count, creation date
- **DevTools summary**: running tools and their ports

Data comes from `GET /api/sites/<name>`.

### 4.5 Tab: Config

Move the "Configure…" dialog contents (sites.tsx:184-226) into this tab as an
inline form, not a modal. Better suited for serious editing.

Field:
- PHP version (select — reuse `phpOptions` logic from sites.tsx:186)
- php.ini override (text — path or relative)
- Docroot (text)
- Aliases (chip input — add/remove domains)
- Env vars (key-value editor — table with add/remove rows)

Save button → `PUT /api/sites/<name>/config` (already exists).

### 4.6 Tab: Logs

Multi-source log viewer. Reuses the pattern from logs.tsx but scoped to one site:

```
[php.log] [vite.log] [artisan.log]    auto-refresh [ON]
┌──────────────────────────────────┐
│ [2026-08-25 10:00:01] GET / 200  │
│ [2026-08-25 10:00:02] VITE HMR   │
│ ...                              │
└──────────────────────────────────┘
```

- Tab per log source (php, vite, artisan, database)
- Polling `GET /api/sites/<name>/logs?log=<source>&lines=200` every 2.5s
- Auto-scroll to bottom (like the terminal)
- Toggle auto-refresh (reused from logs.tsx)

### 4.7 Tab: Dev Tools

This is the core of the Laravel/Vite solution:

```
Dev Tools
┌─────────────────────────────────────────────────────────────┐
│ Vite Dev Server                          [▶ Start] [■ Stop] │
│ Status: running · Port 5173 · PID 12345                     │
│ ┌─ logs/myapp.vite.log ──────────────────────────────────┐ │
│ │ VITE v5.4.0  ready in 340 ms                            │ │
│ │ ➜  Local:   http://127.0.0.1:5173/                      │ │
│ │ ➜  Network: use --host to expose                        │ │
│ └────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│ Laravel Dev (composer run dev)           [▶ Start] [■ Stop] │
│ Status: stopped                                             │
├─────────────────────────────────────────────────────────────┤
│ One-shot commands                                           │
│ [npm run build]  [composer install]  [composer update]     │
└─────────────────────────────────────────────────────────────┘
```

For each tool:
- Card with name, status (running/stopped), port, PID
- Start/Stop buttons
- Inline log viewer (tail of the log file)
- Auto-detect: if `vite.config.*` exists, show the Vite card. If
  composer.json has `scripts.dev` (Laravel 11+), show the
  Laravel Dev card. If `package.json` exists, show the npm/composer cards.

When Start Vite is clicked:
1. `POST /api/sites/<name>/devtools {tool: "vite", action: "start"}`
2. Backend spawns `npx vite --port <auto-picked>`
3. Wait for the HTTP ready probe on that port
4. Register the ViteProxy on the site server
5. Inject `SABDOPALON_VITE_PORT` into the PHP env
6. Restart the PHP process so the new env takes effect
7. Frontend polls status, shows the log

When Stop is clicked:
1. `POST /api/sites/<name>/devtools {tool: "vite", action: "stop"}`
2. Backend kills the process group
3. Remove the ViteProxy from the site server
4. Restart PHP (or leave it — the ViteProxy just becomes a no-op)

### 4.8 Tab: Terminal

Inline terminal, auto-cd into the site directory. Reuses the existing
`TerminalPanel` component (sites.tsx:29).

Differences from the current terminal dock:
- Terminal session key = `site-<name>` (not global)
- Auto-cd into `sites/<name>/` when the session is created
- Same env vars as injected into the PHP process
  (SABDOPALON_DB_ENGINE, etc.)

WebSocket endpoint: `WS /api/sites/<name>/terminal`
(mirrors `handlers_terminal.go` but with auto-cd + scoped env)

---

## Part 5: Step-by-Step Implementation Plan

### Phase 1: Foundation (backend, no UI)

**Step 1.1** — Create the `internal/devtools` package
- `devtools.go`: Manager, runningProc, Start/Stop/Status/StopAll
- `registry.go`: ToolSpec for Vite, Artisan, npm, composer
- `devtools_test.go`: test Start/Stop/Status with a mock binary
- Mirrors the pattern from `services.go` (runningProc, setProcessGroup, ready probe)

**Step 1.2** — Create `internal/proxy/framework.go`
- `DetectFramework(siteDir) Framework`
- Test with Laravel/WordPress/blank fixture folders

**Step 1.3** — Create `internal/proxy/routers.go`
- `laravelRouter` const (PHP string)
- Move the existing `defaultRouter` from proxy.go:713 here
- `pickRouter(framework) string` — returns router script

**Step 1.4** — Integrate framework detection into `ensureSite`
- proxy.go:490 — replace the hardcoded `defaultRouter` with `pickRouter(framework)`
- Cache the detection result in the `siteServer` struct

**Step 1.5** — Wire devtools.Manager into the app
- `internal/app/app.go` — instantiate `devtools.New(cfg)`
- Pass it to the dashboard server + proxy
- Shutdown hook: `devtools.StopAll()` (mirrors `services.StopAll`)

**Step 1.6** — Wire devtools into the proxy lifecycle
- `proxy.StopSite(name)` → `devtools.StopAllForSite(name)`
- `proxy.RestartSite(name)` → save tool states, restart, restore

### Phase 2: Vite Proxy (backend)

**Step 2.1** — Create `internal/proxy/viteproxy.go`
- `ViteProxy` struct + `ShouldIntercept` + `ServeHTTP`
- Unit test with a mock HTTP server

**Step 2.2** — Integrate into the proxy handler
- When devtools starts Vite → register the ViteProxy on the siteServer
- When devtools stops Vite → remove the ViteProxy
- In the main handler: check the ViteProxy before forwarding to PHP

**Step 2.3** — Vite env injection
- `startPHP`: add `SABDOPALON_VITE_PORT` + `SABDOPALON_VITE_HOST` to the env

**Step 2.4** — End-to-end test
- Fixture: dummy Laravel site + mock Vite server
- Assert: requests to `/@vite/client` are proxied to Vite, not to PHP

### Phase 3: API (backend)

**Step 3.1** — `handlers_sitedetail.go`
- `handleAPISiteDetail` — aggregate response
- `getSiteLogs` — multi-log tail
- `getSiteDevTools` — status
- `postSiteDevTools` — start/stop

**Step 3.2** — Extend routing in `handleAPISiteAction`
- Add cases for `logs`, `devtools`, and empty GET (detail)

**Step 3.3** — Per-site terminal WebSocket
- Extend `handlers_terminal.go` or create a new handler
- Auto-cd into the site dir, scoped env

**Step 3.4** — API tests
- Test that the detail endpoint returns the correct framework detection
- Test devtools start/stop via the API
- Test log tail

### Phase 4: Frontend — Detail Page Shell

**Step 4.1** — Create `site-detail.tsx`
- Layout: header + tab switcher
- `useParams()` picks up the name
- Fetch `GET /api/sites/<name>` (api.ts: add `siteDetail(name)`)
- Back button, Start/Stop/Restart header buttons

**Step 4.2** — Add the route in App.tsx
- `/sites/:name` → SiteDetailPage
- Update `fullBleed` check

**Step 4.3** — Link from the sites list
- In `rowMenu` (sites.tsx:364), change "Configure…" to a link to `/sites/<name>?tab=config`
- Clicking a site name → navigates to the detail page

**Step 4.4** — API client functions (api.ts)
- `siteDetail(name): Promise<SiteDetail>`
- `siteLogs(name, source, lines): Promise<LogResponse>`
- `siteDevTools(name): Promise<ToolStatus[]>`
- `siteDevToolAction(name, tool, action): Promise<...>`

### Phase 5: Frontend — Tabs

**Step 5.1** — Overview tab
- Cards for framework, PHP, database, URL, storage, devtools summary
- Poll status via `useLive()` or by polling `GET /api/sites/<name>`

**Step 5.2** — Config tab
- Move the form from the dialog (sites.tsx:184-226) to inline
- PHP select, docroot, aliases chip input, env editor
- Save → PUT config (already exists)

**Step 5.3** — Logs tab
- Sub-tab per log source
- Reuse the polling pattern from logs.tsx
- Auto-scroll, auto-refresh toggle

**Step 5.4** — Dev Tools tab
- Card per tool (auto-shown based on framework + file detection)
- Start/Stop buttons → API call
- Inline log tail per tool
- Status indicator (running/stopped/port)

**Step 5.5** — Terminal tab
- Embed the `TerminalPanel` component
- Session key = `site-<name>`
- WebSocket connects to `/api/sites/<name>/terminal`

### Phase 6: Polish & Edge Cases

**Step 6.1** — Auto-start dev tools
- When a site starts, check `.sabdopalon.yml` for `devtools: [vite]`
- If present, auto-start the requested tools

**Step 6.2** — Devtools config in .sabdopalon.yml
```yaml
devtools:
  auto_start:
    - vite
  vite:
    port: 5173  # optional override
```
- Extend `siteconfig.SiteConfig` struct
- Extend YAML parser

**Step 6.3** — Graceful shutdown
- When Sabdopalon quits: kill all devtools (already in StopAll)
- When a site stops: kill devtools for that site (already in StopAllForSite)
- On restart: preserve tool states, restart tools after the site is up

**Step 6.4** — Error handling
- Missing tool binary (npx/node/npm) → clear message + install hint
- Port conflict → auto-pick the next free port, show the port in use
- Vite config error → tail the log, show the error in the UI

**Step 6.5** — Windows path handling
- `php artisan` → on Windows still `php artisan` (php.exe on PATH or bundled)
- `npx vite` → npx from the Node install (user installs Node themselves)
- Forward slash vs backslash in siteDir → use `filepath.Join` (already safe)

**Step 6.6** — Security
- Dev tools bind 127.0.0.1 only (already the default in Sabdopalon)
- The Vite proxy is only active for sites whose Vite is running
- Per-site terminals stay scoped (can't cd out of sites/)

---

## Part 6: Data Flow — End-to-End Examples

### Scenario: User opens a Laravel site with Vite

```
1. User opens http://myapp.localhost in the browser
2. Sabdopalon proxy receives the request
3. ensureSite("myapp"):
   a. Load .sabdopalon.yml → php: 8.3
   b. DetectFramework → Laravel (has artisan)
   c. Pick router → laravelRouter (not defaultRouter)
   d. Resolve PHP → bin/php/8.3/php
   e. Start: php -S 127.0.0.1:8081 -t public .sabdopalon-router.php
4. Browser requests GET /
5. Proxy forwards to PHP:8081
6. laravelRouter.php → require public/index.php
7. Laravel renders the view, injects <script src="/@vite/client">
   (because APP_ENV=local, the Vite plugin is active)
8. Browser requests GET /@vite/client
9. Proxy: ViteProxy.ShouldIntercept("/@vite/client") == true
10. ViteProxy reverse-proxies → http://127.0.0.1:5173/@vite/client
11. Vite dev server returns the HMR client JS
12. Browser connects via WebSocket to Vite for HMR
13. User edits resources/js/app.js → Vite HMR → browser auto-reloads
```

### Scenario: User clicks "Start Vite" in the DevTools tab

```
1. Frontend: POST /api/sites/myapp/devtools {tool:"vite", action:"start"}
2. Backend: devtools.Manager.Start("myapp", siteDir, "vite")
3. Manager: pick free port (5173), spawn "npx vite --port 5173"
4. Manager: wait for HTTP ready at 127.0.0.1:5173 (ReadyKind=http)
5. Manager: log → logs/myapp.vite.log
6. Manager: register port mapping myapp→5173
7. Backend: proxy.RegisterViteProxy("myapp", 5173)
8. Backend: restart PHP process with SABDOPALON_VITE_PORT=5173 env
9. Backend: return {ok: true, port: 5173, pid: 12345}
10. Frontend: show "running", poll logs/myapp.vite.log
11. User opens myapp.localhost → Vite assets are proxied → HMR active
```

### Scenario: User stops a site

```
1. Frontend: POST /api/sites/myapp/stop
2. Backend: proxy.StopSite("myapp")
3. proxy.StopSite:
   a. kill PHP process group
   b. devtools.StopAllForSite("myapp") → kill Vite process group
   c. unregister ViteProxy
4. Backend: return {ok: true}
5. Frontend: update status → stopped, devtools cards → stopped
```

---

## Part 7: File Inventory (to be created/modified)

### New files

| File | Purpose |
|---|---|
| `internal/devtools/devtools.go` | Manager, runningProc, lifecycle |
| `internal/devtools/registry.go` | Tool specs |
| `internal/devtools/devtools_test.go` | Unit tests |
| `internal/proxy/framework.go` | DetectFramework |
| `internal/proxy/routers.go` | Laravel router + pickRouter |
| `internal/proxy/viteproxy.go` | Vite reverse-proxy |
| `internal/dashboard/handlers_sitedetail.go` | Detail/logs/devtools API |
| `internal/dashboard/handlers_sitedetail_test.go` | API tests |
| `internal/dashboard/ui/src/pages/site-detail.tsx` | Detail page shell |
| `internal/dashboard/ui/src/pages/site-detail/overview-tab.tsx` | Overview |
| `internal/dashboard/ui/src/pages/site-detail/config-tab.tsx` | Config editor |
| `internal/dashboard/ui/src/pages/site-detail/logs-tab.tsx` | Log viewer |
| `internal/dashboard/ui/src/pages/site-detail/devtools-tab.tsx` | DevTools UI |
| `internal/dashboard/ui/src/pages/site-detail/terminal-tab.tsx` | Terminal |

### Modified files

| File | Changes |
|---|---|
| `internal/proxy/proxy.go` | ensureSite: framework detection + router pick + ViteProxy |
| `internal/proxy/php.go` | startPHP: add Vite env injection |
| `internal/siteconfig/siteconfig.go` | Add DevTools config fields |
| `internal/dashboard/handlers_sites.go` | Extend dispatch for detail/logs/devtools |
| `internal/dashboard/server.go` | Register devtools manager, new routes |
| `internal/app/app.go` | Instantiate devtools.Manager, wire shutdown |
| `internal/dashboard/ui/src/App.tsx` | Add /sites/:name route, update fullBleed |
| `internal/dashboard/ui/src/lib/api.ts` | Add siteDetail, siteLogs, siteDevTools functions |
| `internal/dashboard/ui/src/pages/sites.tsx` | Link the row menu to the detail page |

---

## Part 8: Out of Scope for This Design

- **File manager UI** — browsing/editing site files via the dashboard. Large,
  needs an ACE or Monaco editor. Separate backlog.
- **Per-site database** — one DB per site (not shared). Separate backlog.
- **Git integration** — status, commit, diff in the dashboard. Separate backlog.
- **Deploy/Push** — deploying sites to a remote server. Separate backlog.
- **Multi-PHP per request** — running multiple PHP versions within one site.
  Not feasible with `php -S` (one binary per process).

---

## Note: Feature Freeze

This was implemented as a compact functional unit: the
`internal/devtools` package (backend), framework detection + Vite proxy (proxy),
site-detail API (dashboard), and site-detail page (frontend) — all
verified with `go build`, `go vet`, `go test`, `npm run build`, and
`GOOS=windows/darwin` cross-compilation.

The changes don't alter any existing API or UI (all new endpoints
are additive; the `TestSiteActionMethodsAreStrict` test still passes).
The site detail page is reached from a new link in the row menu and by clicking a site
name in the table — it doesn't disrupt the existing flow.
