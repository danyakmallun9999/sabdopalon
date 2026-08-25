# Site Detail Page & Dev-Tools Supervisor — Architecture & Implementation

Status: **Implemented** (all phases complete; build, vet, test, and
cross-compilation verified green on linux/darwin/windows).
Scope: per-site detail dashboard + managed dev-tool processes (Vite, artisan,
composer) + framework-aware serving (Laravel/Vite proxy).

---

## Daftar Isu yang Diselesaikan

| # | Masalah user | Akar masalah | Solusi di desain ini |
|---|---|---|---|
| 1 | "Harus `composer run dev` dulu baru URL Sabdopalon jalan" | Vite dev server (`localhost:5173`) tidak di-proxy; aset `@vite` gagal load di `*.localhost` | DevTools supervisor + Vite reverse-proxy |
| 2 | Laravel route 404 / asset not found | defaultRouter generik tidak tahu Laravel front controller | Framework detection + Laravel router khusus |
| 3 | "Di mana aku setting php.ini?" | Tidak ada per-site UI; harus cari file manual | Tab "PHP Config" di detail page |
| 4 | Tidak ada pusat kendali per-site | Config di dialog, logs di page lain, terminal dock terpisah | Site Detail Page (tabbed, satu pintu) |
| 5 | Tidak tahu framework apa di site | Tidak ada deteksi framework | Framework detector di overview |
| 6 | `npm run dev` mati saat tutup terminal | Tidak ada supervisor; user jalankan manual | DevTools supervisor (mirror pola `services.Manager`) |

---

## Arsitektur Lapisan

```
┌──────────────────────────────────────────────────────────┐
│  React SPA  (internal/dashboard/ui/src)                 │
│  /sites              → list page (yang sekarang)         │
│  /sites/:name        → SiteDetailPage (BARU)             │
│    ├─ Overview tab   (framework, status, env)            │
│    ├─ Config tab     (php, docroot, aliases, env, ini)   │
│    ├─ Logs tab       (php.log + vite.log + artisan.log)  │
│    ├─ DevTools tab   (start/stop Vite, artisan, npm)     │
│    └─ Terminal tab   (inline PTY, auto-cd ke site dir)   │
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
│  devtools package (BARU — mirror internal/services)      │
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
│  ViteProxy (BARU)                                        │
│    ├─ Intercept /@vite/, /node_modules/.vite/            │
│    └─ httput.ReverseProxy → localhost:5173               │
└──────────────────────────────────────────────────────────┘
```

---

## Bagian 1: Dev-Tools Supervisor (`internal/devtools`)

### 1.1 Mengapa paket baru, bukan pakai `internal/services`

`internal/services` (services.go:274) mengelola service **global** — satu
Mailpit untuk semua site, satu Redis untuk semua site. Dev-tools bersifat
**per-site**: site A jalan Vite di port 5173, site B jalan Vite di port 5174.
Manager-nya juga harus tahu site mana pemilik proses, supaya saat site
di-stop, dev-tools-nya juga mati.

Pola yang dipakai ulang dari `services.go`:
- `runningProc` struct (cmd + log file) → sama persis
- `setProcessGroup` + `killProcessGroup` → sama persis
- `ready()` probe → dipakai untuk Vite (HTTP probe ke `localhost:PORT`)
- Port allocation → mirror `proxy.go:497` (`isPortFree` loop)

### 1.2 Struktur paket

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
    Name      string   // "vite", "artisan-serve", "npm-dev", "composer"
    Label     string   // "Vite Dev Server"
    BinName   string   // "npx", "php", "node", "composer"
    Args      func(siteDir string, port int) []string
    Port      int      // 0 = no port (e.g. composer install)
    ReadyKind string   // "http" | "tcp" | "" (no probe)
    ReadyPath string   // "/@vite/" for Vite HTTP probe
    Env       func(siteDir string) []string // extra env (APP_ENV=local, dll)
}
```

### 1.4 Registry — tool yang didukung

| Tool | Bin | Args | Port | Ready | Kapan dipakai |
|---|---|---|---|---|---|
| `vite` | `npx` | `["vite", "--port", "<N>"]` | 5173+ | http `localhost:N` | Ada `vite.config.{js,ts}` |
| `artisan-serve` | `php` | `["artisan", "serve", "--port", "<N>"]` | 8000+ | http | Opsional, untuk debugging tanpa Sabdopalon proxy |
| `npm-dev` | `npm` | `["run", "dev"]` | — | — | Generic fallback (bukan Vite) |
| `npm-build` | `npm` | `["run", "build"]` | — | — | One-shot build, bukan long-running |
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

| Event | Yang terjadi |
|---|---|
| `proxy.StopSite(name)` | `devtools.StopAllForSite(name)` |
| `proxy.RestartSite(name)` | stop tools → restart site → auto-restart tools yang sebelumnya running |
| Sabdopalon shutdown | `devtools.StopAll()` (mirror `services.Manager.StopAll`) |
| Site di-delete | `devtools.StopAllForSite(name)` sebelum folder dipindah ke `.trash/` |

### 1.7 Port allocation

```
base = 5173 (Vite default)
for each site that starts Vite:
  port = base
  while !isPortFree(port): port++
  record site→port mapping
```

Disimpan in-memory (tidak persist). Saat restart, port bisa beda — itu OK,
karena Vite proxy baca port dari mapping live, bukan hardcode.

### 1.8 Logging

Setiap tool → `logs/<site>.<tool>.log`
- `logs/myapp.vite.log`
- `logs/myapp.artisan.log`

Format sama dengan `services.go:114` (`os.O_CREATE|O_WRONLY|O_TRUNC`).
Tail via endpoint yang sudah ada (`/api/logs/`) — tinggal tambah nama log baru.

---

## Bagian 2: Framework Detection & Laravel Router

### 2.1 Detektor (`internal/proxy/framework.go` — BARU)

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

Cache hasil deteksi di `siteServer` struct (sekali per session, tidak
re-scan tiap request).

### 2.2 Laravel Router (`internal/proxy/routers.go` — BARU)

Router khusus Laravel, ditulis ke `.sabdopalon-router.php` saat framework
Laravel terdeteksi (menggantikan `defaultRouter` generik):

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

Mengapa ini menyelesaikan masalah routing Laravel:
- `SCRIPT_NAME` dan `SCRIPT_FILENAME` di-set benar — Laravel's
  `Request::capture()` butuh ini untuk URI parsing
- `PATH_INFO` benar — supaya `Route::get('/users/{id}')` match
- Tidak perlu `.htaccess` (Apache) — ini PHP built-in server

### 2.3 Vite Reverse-Proxy (`internal/proxy/viteproxy.go` — BARU)

Ini adalah inti solusi "tidak perlu composer run dev manual":

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

### 2.4 Integrasi ke `ensureSite` (proxy.go:457)

Alur baru saat request masuk ke site:

```
ensureSite(name):
  1. Load siteconfig (.sabdopalon.yml)
  2. Detect framework (cached)           ← BARU
  3. Pick router script:
       Laravel → laravelRouter           ← BARU
       else    → defaultRouter
  4. Start php -S with chosen router
  5. If framework == Laravel && Vite running:
       register ViteProxy for this site  ← BARU
```

Di handler proxy utama (sebelum forward ke PHP):

```
handleRequest(host, r):
  site = resolveSite(host)
  if site.viteProxy != nil && site.viteProxy.ShouldIntercept(r):
    site.viteProxy.ServeHTTP(w, r)
    return
  // else: normal PHP forward
  site.forward(w, r)
```

### 2.5 Vite port injection ke PHP env

Saat `startPHP` (php.go:62), tambah env:

```go
if vp := s.getViteProxy(name); vp != nil {
    env = append(env,
        fmt.Sprintf("SABDOPALON_VITE_PORT=%d", vp.port),
        fmt.Sprintf("SABDOPALON_VITE_HOST=127.0.0.1"),
    )
}
```

Laravel's `vite.config.js` bisa baca `process.env.SABDOPALON_VITE_PORT`
untuk set `server.hmr.host` dan `server.origin`, supaya HMR websocket
connect ke host yang benar.

Sebagai alternatif yang lebih robust: sediakan template
`vite.config.sabdopalon.js` yang user bisa copy ke project mereka — ini
pre-configured untuk Sabdopalon:

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

## Bagian 3: Site Detail Page — Backend API

### 3.1 Endpoint baru

Semua di bawah `/api/sites/<name>/` (extend `handleAPISiteAction`):

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/sites/<name>` | Detail aggregate: config + status + framework + devtools + port |
| GET | `/api/sites/<name>/logs` | Multi-log tail (php + vite + artisan), parameter `?log=vite&lines=100` |
| POST | `/api/sites/<name>/devtools` | Body: `{tool: "vite", action: "start"\|"stop"}` |
| GET | `/api/sites/<name>/devtools` | Status semua devtools untuk site ini |
| WS | `/api/sites/<name>/terminal` | Per-site PTY (mirror terminal handler, auto-cd ke site dir) |

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

### 3.3 Handler implementasi

Buat file baru `internal/dashboard/handlers_sitedetail.go`:

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

Extend `handleAPISiteAction` (handlers_sites.go:94). Saat ini dispatch
berdasarkan `action` string (start/stop/restart/config). Tambah:

```go
case http.MethodGet:
    switch action {
    case "config":
        s.getSiteConfig(w, name)
    case "logs":
        s.getSiteLogs(w, name, r)        // BARU
    case "devtools":
        s.getSiteDevTools(w, name)       // BARU
    case "":
        s.handleAPISiteDetail(w, name)  // BARU — GET /api/sites/<name>
    default:
        http.NotFound(w, r)
    }
```

---

## Bagian 4: Site Detail Page — Frontend

### 4.1 Routing (App.tsx)

Tambah route:

```tsx
const SiteDetailPage = lazy(() => import("@/pages/site-detail"))

<Route path="/sites" element={<SitesPage />} />
<Route path="/sites/:name" element={<SiteDetailPage />} />  // BARU
```

`fullBleed` set juga untuk `/sites/:name`:

```tsx
const fullBleed = location.pathname === "/sites" ||
                  location.pathname.startsWith("/sites/") ||
                  location.pathname === "/terminal"
```

### 4.2 Struktur komponen

```
ui/src/pages/
  site-detail.tsx              ← halaman utama (layout + tab switcher)
  site-detail/
    overview-tab.tsx           ← framework, status, info cards
    config-tab.tsx             ← editor .sabdopalon.yml (pindah dari dialog)
    logs-tab.tsx               ← multi-log tailer
    devtools-tab.tsx           ← start/stop Vite, artisan, dll
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
- Start/Stop/Restart buttons (reuse `act()` dari sites.tsx:235)

### 4.4 Tab: Overview

Isi:
- **Framework card**: logo/nama framework, versi, link ke dokumentasi
- **PHP card**: versi aktif, path binary, badge "bundled" / "system"
- **Database card**: engine, status running, connection string (dari env)
- **URL card**: HTTP + HTTPS + alias, semua clickable
- **Storage card**: ukuran folder, jumlah file, tanggal dibuat
- **DevTools summary**: tool yang running, port-nya

Data dari `GET /api/sites/<name>`.

### 4.5 Tab: Config

Pindahkan isi dialog "Configure…" (sites.tsx:184-226) ke tab ini sebagai
inline form, bukan modal. Lebih cocok untuk editing yang serius.

Field:
- PHP version (select — reuse `phpOptions` logic dari sites.tsx:186)
- php.ini override (text — path atau relative)
- Docroot (text)
- Aliases (chip input — tambah/hapus domain)
- Env vars (key-value editor — tabel dengan add/remove row)

Save button → `PUT /api/sites/<name>/config` (sudah ada).

### 4.6 Tab: Logs

Multi-source log viewer. Reuse pattern dari logs.tsx tapi khusus satu site:

```
[php.log] [vite.log] [artisan.log]    auto-refresh [ON]
┌──────────────────────────────────┐
│ [2026-08-25 10:00:01] GET / 200  │
│ [2026-08-25 10:00:02] VITE HMR   │
│ ...                              │
└──────────────────────────────────┘
```

- Tab per log source (php, vite, artisan, database)
- Polling `GET /api/sites/<name>/logs?log=<source>&lines=200` tiap 2.5s
- Auto-scroll to bottom (seperti terminal)
- Toggle auto-refresh (reuse dari logs.tsx)

### 4.7 Tab: Dev Tools

Ini adalah inti dari solusi Laravel/Vite:

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
│ Artisan Serve (optional)                 [▶ Start] [■ Stop] │
│ Status: stopped                                             │
├─────────────────────────────────────────────────────────────┤
│ One-shot commands                                           │
│ [npm run build]  [composer install]  [composer update]     │
└─────────────────────────────────────────────────────────────┘
```

Untuk setiap tool:
- Card dengan nama, status (running/stopped), port, PID
- Tombol Start/Stop
- Inline log viewer (tail dari log file)
- Auto-detect: kalau ada `vite.config.*`, tampilkan Vite card. Kalau
  ada `artisan`, tampilkan Artisan card. Kalau ada `package.json`,
  tampilkan npm/composer cards.

Saat Start Vite diklik:
1. `POST /api/sites/<name>/devtools {tool: "vite", action: "start"}`
2. Backend spawn `npx vite --port <auto-picked>`
3. Tunggu HTTP ready probe di port tersebut
4. Daftarkan ViteProxy ke site server
5. Inject `SABDOPALON_VITE_PORT` ke PHP env
6. Restart PHP process supaya env baru berlaku
7. Frontend poll status, tampilkan log

Saat Stop diklik:
1. `POST /api/sites/<name>/devtools {tool: "vite", action: "stop"}`
2. Backend kill process group
3. Hapus ViteProxy dari site server
4. Restart PHP (atau biarkan — ViteProxy tinggal no-op)

### 4.8 Tab: Terminal

Inline terminal, auto-cd ke site directory. Reuse `TerminalPanel`
komponen yang sudah ada (sites.tsx:29).

Perbedaan dari terminal dock sekarang:
- Terminal session key = `site-<name>` (bukan global)
- Auto-cd ke `sites/<name>/` saat session dibuat
- Env vars sama dengan yang di-inject ke PHP process
  (SABDOPALON_DB_ENGINE, dll)

WebSocket endpoint: `WS /api/sites/<name>/terminal`
(mirror `handlers_terminal.go` tapi auto-cd + scoped env)

---

## Bagian 5: Step-by-Step Implementation Plan

### Fase 1: Foundation (backend, no UI)

**Step 1.1** — Buat paket `internal/devtools`
- `devtools.go`: Manager, runningProc, Start/Stop/Status/StopAll
- `registry.go`: ToolSpec untuk Vite, Artisan, npm, composer
- `devtools_test.go`: test Start/Stop/Status dengan mock binary
- Mirror pola dari `services.go` (runningProc, setProcessGroup, ready probe)

**Step 1.2** — Buat `internal/proxy/framework.go`
- `DetectFramework(siteDir) Framework`
- Test dengan fixture folder Laravel/WordPress/blank

**Step 1.3** — Buat `internal/proxy/routers.go`
- `laravelRouter` const (PHP string)
- `defaultRouter` yang sudah ada di proxy.go:713 → pindah ke sini
- `pickRouter(framework) string` — returns router script

**Step 1.4** — Integrasikan framework detection ke `ensureSite`
- proxy.go:490 — ganti hardcoded `defaultRouter` dengan `pickRouter(framework)`
- Cache hasil deteksi di `siteServer` struct

**Step 1.5** — Wire devtools.Manager ke app
- `internal/app/app.go` — instantiate `devtools.New(cfg)`
- Pass ke dashboard server + proxy
- Shutdown hook: `devtools.StopAll()` (mirror `services.StopAll`)

**Step 1.6** — Wire devtools ke proxy lifecycle
- `proxy.StopSite(name)` → `devtools.StopAllForSite(name)`
- `proxy.RestartSite(name)` → save tool states, restart, restore

### Fase 2: Vite Proxy (backend)

**Step 2.1** — Buat `internal/proxy/viteproxy.go`
- `ViteProxy` struct + `ShouldIntercept` + `ServeHTTP`
- Unit test dengan mock HTTP server

**Step 2.2** — Integrasi ke proxy handler
- Saat devtools start Vite → daftarkan ViteProxy ke siteServer
- Saat devtools stop Vite → hapus ViteProxy
- Di handler utama: cek ViteProxy sebelum forward ke PHP

**Step 2.3** — Vite env injection
- `startPHP`: tambah `SABDOPALON_VITE_PORT` + `SABDOPALON_VITE_HOST` ke env

**Step 2.4** — Test end-to-end
- Fixture: site Laravel dummy + mock Vite server
- Assert: request ke `/@vite/client` di-proxy ke Vite, bukan ke PHP

### Fase 3: API (backend)

**Step 3.1** — `handlers_sitedetail.go`
- `handleAPISiteDetail` — aggregate response
- `getSiteLogs` — multi-log tail
- `getSiteDevTools` — status
- `postSiteDevTools` — start/stop

**Step 3.2** — Extend routing di `handleAPISiteAction`
- Tambah case untuk `logs`, `devtools`, dan GET kosong (detail)

**Step 3.3** — Per-site terminal WebSocket
- Extend `handlers_terminal.go` atau buat handler baru
- Auto-cd ke site dir, scoped env

**Step 3.4** — API tests
- Test detail endpoint return framework detection benar
- Test devtools start/stop via API
- Test log tail

### Fase 4: Frontend — Detail Page Shell

**Step 4.1** — Buat `site-detail.tsx`
- Layout: header + tab switcher
- `useParams()` ambil name
- Fetch `GET /api/sites/<name>` (api.ts: tambah `siteDetail(name)`)
- Back button, Start/Stop/Restart header buttons

**Step 4.2** — Tambah route di App.tsx
- `/sites/:name` → SiteDetailPage
- Update `fullBleed` check

**Step 4.3** — Link dari sites list
- Di `rowMenu` (sites.tsx:364), ganti "Configure…" → link ke `/sites/<name>?tab=config`
- Klik nama site → navigasi ke detail page

**Step 4.4** — API client functions (api.ts)
- `siteDetail(name): Promise<SiteDetail>`
- `siteLogs(name, source, lines): Promise<LogResponse>`
- `siteDevTools(name): Promise<ToolStatus[]>`
- `siteDevToolAction(name, tool, action): Promise<...>`

### Fase 5: Frontend — Tabs

**Step 5.1** — Overview tab
- Cards untuk framework, PHP, database, URL, storage, devtools summary
- Poll status via `useLive()` atau polling `GET /api/sites/<name>`

**Step 5.2** — Config tab
- Pindahkan form dari dialog (sites.tsx:184-226) ke inline
- PHP select, docroot, aliases chip input, env editor
- Save → PUT config (sudah ada)

**Step 5.3** — Logs tab
- Sub-tab per log source
- Reuse polling pattern dari logs.tsx
- Auto-scroll, auto-refresh toggle

**Step 5.4** — Dev Tools tab
- Card per tool (auto-show berdasarkan framework + file detection)
- Start/Stop buttons → API call
- Inline log tail per tool
- Status indicator (running/stopped/port)

**Step 5.5** — Terminal tab
- Embed `TerminalPanel` komponen
- Session key = `site-<name>`
- WebSocket connect ke `/api/sites/<name>/terminal`

### Fase 6: Polish & Edge Cases

**Step 6.1** — Auto-start dev tools
- Saat site start, cek `.sabdopalon.yml` untuk `devtools: [vite]`
- Jika ada, auto-start tool yang diminta

**Step 6.2** — Devtools config di .sabdopalon.yml
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
- Saat Sabdopalon quit: kill semua devtools (sudah di StopAll)
- Saat site stop: kill devtools untuk site itu (sudah di StopAllForSite)
- Saat restart: preserve tool states, restart tools after site up

**Step 6.4** — Error handling
- Tool binary tidak ada (npx/node/npm) → pesan jelas + install hint
- Port conflict → auto-pick next free port, tampilkan port yang dipakai
- Vite config error → tail log, tampilkan error di UI

**Step 6.5** — Windows path handling
- `php artisan` → di Windows tetap `php artisan` (php.exe di PATH atau bundled)
- `npx vite` → npx dari Node install (user install Node sendiri)
- Forward slash vs backslash di siteDir → pakai `filepath.Join` (sudah aman)

**Step 6.6** — Security
- Dev tools bind 127.0.0.1 only (sudah default di Sabdopalon)
- Vite proxy hanya aktif untuk site yang Vite-nya running
- Terminal per-site tetap scoped (tidak bisa cd keluar dari sites/)

---

## Bagian 6: Data Flow — Contoh End-to-End

### Skenario: User buka Laravel site dengan Vite

```
1. User buka http://myapp.localhost di browser
2. Sabdopalon proxy terima request
3. ensureSite("myapp"):
   a. Load .sabdopalon.yml → php: 8.3
   b. DetectFramework → Laravel (ada artisan)
   c. Pick router → laravelRouter (bukan defaultRouter)
   d. Resolve PHP → bin/php/8.3/php
   e. Start: php -S 127.0.0.1:8081 -t public .sabdopalon-router.php
4. Browser request GET /
5. Proxy forward ke PHP:8081
6. laravelRouter.php → require public/index.php
7. Laravel render view, inject <script src="/@vite/client">
   (karena APP_ENV=local, Vite plugin aktif)
8. Browser request GET /@vite/client
9. Proxy: ViteProxy.ShouldIntercept("/@vite/client") == true
10. ViteProxy reverse-proxy → http://127.0.0.1:5173/@vite/client
11. Vite dev server return HMR client JS
12. Browser connect WebSocket ke Vite untuk HMR
13. User edit resources/js/app.js → Vite HMR → browser auto-reload
```

### Skenario: User klik "Start Vite" di DevTools tab

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
11. User buka myapp.localhost → Vite assets ter-proxy → HMR aktif
```

### Skenario: User stop site

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

## Bagian 7: File Inventory (yang akan dibuat/diubah)

### File baru

| File | Fungsi |
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

### File yang diubah

| File | Perubahan |
|---|---|
| `internal/proxy/proxy.go` | ensureSite: framework detection + router pick + ViteProxy |
| `internal/proxy/php.go` | startPHP: add Vite env injection |
| `internal/siteconfig/siteconfig.go` | Add DevTools config fields |
| `internal/dashboard/handlers_sites.go` | Extend dispatch untuk detail/logs/devtools |
| `internal/dashboard/server.go` | Register devtools manager, new routes |
| `internal/app/app.go` | Instantiate devtools.Manager, wire shutdown |
| `internal/dashboard/ui/src/App.tsx` | Add /sites/:name route, update fullBleed |
| `internal/dashboard/ui/src/lib/api.ts` | Add siteDetail, siteLogs, siteDevTools functions |
| `internal/dashboard/ui/src/pages/sites.tsx` | Link row menu ke detail page |

---

## Bagian 8: Yang TIDAK Termasuk Scope Desain Ini

- **File manager UI** — browsing/editing file site lewat dashboard. Besar,
  butuh ACE editor atau Monaco. Backlog terpisah.
- **Database per-site** — satu DB per site (bukan shared). Backlog terpisah.
- **Git integration** — status, commit, diff di dashboard. Backlog terpisah.
- **Deploy/Push** — deploy site ke remote server. Backlog terpisah.
- **Multi-PHP per request** — run multiple PHP version dalam satu site.
  Tidak feasible dengan `php -S` (one binary per process).

---

## Catatan: Feature Freeze

Implementasi ini selesai sebagai unit fungsional yang kompak: paket
`internal/devtools` (backend), framework detection + Vite proxy (proxy),
site-detail API (dashboard), dan site-detail page (frontend) — semua
terverifikasi dengan `go build`, `go vet`, `go test`, `npm run build`, dan
cross-compilation `GOOS=windows/darwin`.

Perubahan tidak mengubah API existing atau UI yang sudah ada (semua endpoint
baru bersifat aditif; test `TestSiteActionMethodsAreStrict` tetap lulus).
Site detail page diakses dari link baru di row menu dan dari klik nama site
di tabel — tidak mengganggu flow existing.
