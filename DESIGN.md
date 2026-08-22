# Sabdopalon — Design Document

> Portable, cross-platform local development environment.
> Clean-room orchestrator over open-source components. MIT licensed, free forever.

**Version:** 0.5.0
**Status:** ✅ Dashboard-first release — see CHANGELOG.md
**Last updated:** 2026-08-22

---

## 1. Background & Motivation

### 1.1 The problem

- **Laragon v7+** (Dec 2024) introduced a paid license: $49/yr or $149 lifetime
  for commercial use. Non-commercial unlicensed use still works but shows nag
  popups. Laragon remains **Windows-only**.
- **XAMPP** is free and open-source but rigid: fixed install dir, no pretty
  URLs, no per-project PHP version switching, no modern dashboard.
- Developers on **Linux/macOS** have no "Laragon equivalent" — they fall back
  to manual Apache/Nginx config or heavyweight Docker setups.

### 1.2 The opportunity

Almost everything Laragon/XAMPP *bundle* (Apache, Nginx, MySQL, PHP, Node,
Python, Redis, PostgreSQL) is **open-source and freely redistributable**. The
"secret sauce" is not proprietary code — it is **orchestration**.

Sabdopalon is a **clean-room reimplementation** of that orchestration layer.
It contains no code derived from Laragon or XAMPP.

### 1.3 The architectural insight

During development we discovered that **Apache and Nginx are unnecessary** for
local development. PHP has a capable built-in HTTP server (`php -S`), and Go can
multiplex requests to it by Host header. This eliminates:

- Downloading/building Apache or Nginx binaries
- Generating and maintaining vhost config files
- Running multiple heavyweight daemons

Combined with the fact that `*.localhost` resolves to `127.0.0.1` automatically
on modern OSes (Linux, macOS, Windows 10+), Sabdopalon needs **zero system
privileges** — no `/etc/hosts` editing, no admin/root.

### 1.4 Name

"Sabdopalon" — a loyal, wise advisor figure in Javanese wayang. Fitting
metaphor for a tool that quietly prepares and serves the dev environment.
Tagline: *"Sabdopalon — Portable Local Dev Server."*

---

## 2. Goals & Non-Goals

### 2.1 Goals

| # | Goal | Status |
|---|------|--------|
| G1 | **Cross-platform**: Windows, Linux, macOS from one Go codebase. | ✅ Go std-lib only |
| G2 | **Portable**: move the whole folder — zero reconfig. | ✅ |
| G3 | **Free forever**: MIT, no nagware, no paywall. | ✅ |
| G4 | **Pretty URLs**: auto `name.localhost` without manual config. | ✅ `.localhost` auto-resolves |
| G5 | **No admin/root needed**. | ✅ |
| G6 | **Per-site PHP**: one PHP server per project, started on demand. | ✅ |
| G7 | **Fast & lightweight**: single static binary, minimal RAM. | ✅ ~9.7 MB binary |
| G8 | **Zero-dependency web server** — no Apache/Nginx needed. | ✅ Go proxy + PHP built-in |
| G9 | **Database**: zero-setup SQLite, with MySQL/MariaDB later. | ✅ SQLite working |
| G10 | **Auto SSL** via mkcert-style root CA. | 🔧 implemented, HTTPS proxy pending |

### 2.2 Non-Goals (for v1)

- Not a replacement for Docker container isolation. Local dev only.
- Not a production server.
- No bundled binaries — PHP is provided by the user (auto-detected).

---

## 3. Architecture

### 3.1 High-level

```
                 ┌─────────────────────────────────────┐
                 │     sabdopalon (Go binary)          │
                 │                                     │
   browser ─────▶│   HTTP proxy (net/http)  :8080      │
                 │     routes by Host header            │
                 │                                     │
                 │  Host: example-app.localhost         │
                 │     │                                │
                 │     ▼  (first access: lazy-start)    │
                 │   httputil.ReverseProxy ──────────▶  php -S 127.0.0.1:9001 -t sites/example-app/public
                 │                                     │
                 │  Host: blog.localhost                │
                 │     ▼                                │
                 │   httputil.ReverseProxy ──────────▶  php -S 127.0.0.1:9002 -t sites/blog/public
                 │                                     │
                 │  Host: localhost (bare)              │
                 │     ▼                                │
                 │   Dashboard (HTML site list)         │
                 └─────────────────────────────────────┘
```

### 3.2 How routing works

1. One Go HTTP server listens on `:8080`.
2. Each request's `Host` header is parsed: `example-app.localhost` → site `example-app`.
3. On first access to a site, Sabdopalon:
   - Picks a free port (starting from 9001).
   - Launches `php -S 127.0.0.1:<port> -t <docroot> <router>`.
   - Creates an `httputil.ReverseProxy` targeting that port.
   - Stores it in a map for reuse by subsequent requests.
4. `*.localhost` resolves to `127.0.0.1` automatically — **no `/etc/hosts`
   edit, no admin/root needed**.
5. Ctrl+C / SIGTERM: graceful shutdown kills all child PHP processes.

### 3.3 PHP router script

Each site gets a `.sabdopalon-router.php` (auto-created if absent). It serves
static files directly and falls back to `index.php` (front-controller pattern):

```php
<?php
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$file = $_SERVER['DOCUMENT_ROOT'] . $uri;
if ($uri !== '/' && is_file($file)) {
    return false; // serve static file
}
$index = $_SERVER['DOCUMENT_ROOT'] . '/index.php';
if (is_file($index)) {
    require $index;
    return true;
}
http_response_code(404);
echo "404 Not Found";
```

### 3.4 Database

- **SQLite (default):** zero-setup. The DB file is created on first use at
  `data/sabdopalon.db`. PHP apps access it via PDO and the `SABDOPALON_DB_PATH`
  env var that Sabdopalon injects.
- **MariaDB/MySQL (auto-managed, v0.2):** the daemon is started on demand when
  `sabdopalon serve` runs. On first run, the data directory is initialized via
  `mariadb-install-db`. The daemon listens on port 3306 with a Unix socket in
  `data/mariadb-sock/`. PHP apps connect via `mysql:host=127.0.0.1;port=3306`.
  The daemon is stopped gracefully on Ctrl+C.
- MariaDB binary is downloaded via `sabdopalon add mariadb` (package manager
  verifies SHA-256, extracts to `bin/mariadb/` with `strip_root`).
- **PostgreSQL (v0.6, Linux/macOS):** `sabdopalon add postgresql` downloads the
  zonky embedded build (includes `initdb`/`postgres`). First run initializes
  `data/postgresql/` via `initdb -U sabdopalon --auth=trust -E utf8`; the daemon
  listens on `127.0.0.1:5432` with a Unix socket in `data/postgresql-sock/`.
  PHP apps connect via PDO `pgsql` using the injected `SABDOPALON_PG_*` env
  vars. Caveat: zonky publishes no Windows binaries, so PostgreSQL on Windows
  requires a system install (PATH fallback).

### 3.6 Optional services (framework)
- Generic `Spec`-driven service framework in `internal/services` (v0.5+):
  each service is declared declaratively — binary discovery (bundled
  `bin/<name>/` then PATH fallback), ports, run args, readiness probe
  (`tcp`/`http`), optional console UI, and the env vars injected into PHP
  sites while the service is running.
- Registry: **Mailpit**, **Redis** (Windows bundled port + system fallback),
  **MinIO** (S3-compatible), **Meilisearch** (search). Toggled per-service in
  `config/engine.toml` under `[services]` or from the dashboard Services page;
  toggles apply at runtime and persist.
- PHP env injection: `proxy.EnvProvider` callback gathers env only from
  *running* services — a stopped service contributes nothing.

### 3.7 First-run & setup (v0.7)

- `config.Load` returns the sentinel `ErrNotBootstrapped` when
  `config/engine.toml` is missing; `main.go` routes to the **CLI wizard**
  (`sabdopalon setup`) or to **setup-mode** (bare `sabdopalon` runs with
  `--setup-mode` injected).
- `internal/bootstrap.EnsureLayout` creates the canonical layout
  (`sites/ logs/ data/ bin/ certs/ backups/ config/ config/vhosts/
  config/profiles/ packages/`) — idempotent, called on every start.
- Setup-mode server boots *only* the dashboard on :9900 (no proxy/DB/services)
  and exposes `GET /api/setup/status`, `POST /api/setup` (async job) and
  `GET /api/setup/job`. The SPA redirects to `/setup` while
  `bootstrapped == false`.
- Wizard defaults: **PHP + MariaDB** core stack (XAMPP-style), **PostgreSQL
  optional**, ports 8080/8443, optional sample site. All downloads go through
  `pkgmgr` (SHA-256 verified).
- DB root credential: **root without password** (XAMPP/Laragon convention),
  bound to 127.0.0.1/socket only. `data/<engine>/.sabdopalon-initialized`
  marks a completed first initialization; constants
  `database.DatabaseRootUser/Password` are reused by backup + WP template.

### 3.8 Desktop app (Tauri v2) & embedded terminal

- **Mode selection**: `SABDOPALON_DIR` env (set by the desktop sidecar)
  overrides `baseDir()`; CLI keeps its portable exe-dir behavior. One
  codebase, two modes.
- **Sidecar**: the desktop app ships the Go binary via `externalBin`; it
  spawns it with `SABDOPALON_DIR=<user-data-dir> --no-open --setup-mode`,
  polls :9900, then points the native window at the dashboard. Tray menu:
  Open Dashboard / Open Sites / Restart / Start-at-Login / Quit. Data lives
  in the OS user-data dir (Herd-style) so the app itself stays read-only.
- **Terminal**: `internal/terminal` wraps a PTY child — creack/pty on Unix,
  a real ConPTY on Windows (`conpty_windows.go`, adapted from go-pty MIT) so
  colors, resize and interactive programs work everywhere. Sessions get
  Sabdopalon's `bin/` dirs prepended to PATH, `TERM=xterm-256color`
  (the desktop sidecar has none), DB **client** defaults
  (`MYSQL_UNIX_PORT` → `data/<engine>-sock/mysqld.sock`, `*_TCP_PORT`,
  `PGHOST/PGPORT/PGUSER/PGDATABASE`) so `mariadb`/`psql` connect without
  flags, and running services' env (same as PHP injection). Resize frames go
  out instantly via xterm's `onResize` (no polling); dropped WebSockets
  auto-reconnect. `/terminal` hosts multiple tabbed sessions; the Sites page
  keeps a persistent right-hand terminal dock (collapsible, persisted width)
  in an app-frame layout that spans exactly header→bottom with per-pane
  scrolling.
- **PHP configuration**: every `php -S` process gets `PHPRC` set to
  `config/php.ini` (created automatically with memory/upload defaults) so
  users can tune global PHP settings without touching the bundled binary.

### 3.5 SSL

- `ssl:ca` generates a local root CA (`certs/sabdopalon-rootCA.crt/key`) using
  Go's `crypto/x509`.
- `ssl:issue <host>` issues a leaf certificate signed by the CA.
- `ssl:wildcard` issues a `*.<tld>` wildcard cert, enabling HTTPS for all sites.
- HTTPS proxy (binding `:8443`) starts automatically if a wildcard or localhost
  cert exists. Uses Go's `http.Server.ListenAndServeTLS`.
- Auto-install root CA into OS trust store: planned (v0.3).

---

## 4. Differentiators

| Feature | Laragon v7 | XAMPP | **Sabdopalon** |
|---|---|---|---|
| Cross-platform (Win+Linux+macOS) | ✗ (Win only) | partial/separate | ✅ one codebase |
| Free forever, no nagware | ✗ (paid) | ✅ | ✅ MIT |
| Web server required | Apache/Nginx bundled | Apache bundled | ✅ **Go proxy — none needed** |
| Admin/root needed | ✓ (hosts edit) | ✗ | ✅ **no privileges needed** |
| Per-site PHP process | ✗ (shared) | ✗ | ✅ isolated per site |
| Zero-setup DB | ✗ | ✗ | ✅ SQLite built-in |
| Pretty URLs | ✓ (hosts edit) | ✗ | ✅ (auto-resolve `.localhost`) |
| Dashboard | ✗ | ✗ | ✅ built-in |
| Startup weight | heavy | heavy | ✅ light (PHP on demand) |

---

## 5. Technology choices

| Concern | Choice | Rationale |
|---|---|---|
| Core language | **Go** | Single static binary; `net/http` + `httputil.ReverseProxy` do all routing; cross-platform; tiny footprint. |
| TOML parsing | **std-lib only** (internal/toml) | Zero external deps. |
| Web server | **Go proxy → PHP built-in** | Eliminates Apache/Nginx entirely. |
| Database | **SQLite (default)** | Zero setup; file-based; PHP PDO supports it natively. |
| SSL | Go `crypto/x509` | mkcert-style; no external dependency. |
| License | MIT | Maximally permissive. |

---

## 6. Clean-room & legal posture

Sabdopalon performs **orchestration only**:

- Contains **no code** from Laragon, XAMPP, WAMP, or any proprietary tool.
- Does **not** decompile or reverse-engineer any binary.
- Bundles **no** third-party binaries; PHP is provided by the user (auto-detected
  from PATH, herd-lite, or configured in `engine.toml`).
- Distributed under MIT.

The "reverse engineering" performed for this design is **architectural
analysis** of how orchestrators work (route by host, start processes, manage
lifecycle) — not decompilation of any executable.

---

## 7. Implementation status

### Phase 1 — MVP (v0.1) ✅ COMPLETE
- [x] Project scaffold + directory layout
- [x] `engine.toml` config loader (std-lib TOML)
- [x] CLI dispatcher (serve, doctor, sites, vhost, ssl:*, version, help)
- [x] **Multiplexing reverse proxy** (Host-based routing)
- [x] **Per-site PHP server management** (lazy start, graceful stop)
- [x] **Dashboard** (HTML site list at `/`)
- [x] **SQLite** database (zero-setup, env injection)
- [x] **PHP router script** (auto-created, front-controller)
- [x] **Auto-detect PHP** (PATH, herd-lite, common paths)
- [x] **Graceful shutdown** (SIGINT/SIGTERM kills all PHP)
- [x] **Per-site logging** (logs/<site>.php.log)
- [x] SSL root CA + per-site cert generation

### Phase 2 — HTTPS & Database (v0.2) ✅ COMPLETE
- [x] **HTTPS proxy on `:8443`** using generated wildcard cert
- [x] **MariaDB daemon management** (auto-init, start on demand, stop on shutdown)
- [x] **Package downloader** (`sabdopalon add mariadb` — download, SHA-256 verify, extract)
- [x] **Package registry** (`packages/packages.toml`)
- [x] **MariaDB 11.4.12** downloaded, verified, installed, and tested end-to-end
- [x] **PHP→MariaDB connection** verified (PDO mysql, visits table)
- [ ] Auto-install root CA into OS trust store (planned v0.3)
- [ ] Per-project `.sabdopalon.yml` version pinning (planned v0.3)

### Phase 3 — Dashboard, Templates & Trust (v0.3) ✅ COMPLETE
- [x] **Interactive web dashboard** on `:9900` (status, sites, logs, backups)
- [x] **Dashboard JSON API** (`/api/status`, `/api/sites`, `/api/logs`, `/api/backup`)
- [x] **Project templates** (`sabdopalon new blank|laravel|wordpress|codeigniter <name>`)
- [x] **SSL trust store installer** (`ssl:trust` — cross-platform: Linux/macOS/Windows)
- [x] **Per-project `.sabdopalon.yml` parser** (PHP version, DB, docroot, aliases, env)
- [x] **Profile system** (`profile:create/list/delete` — multiple PHP/DB environments)

### Phase 4 — Backups & Profiles (v0.4) ✅ COMPLETE
- [x] **Database backup** (SQLite: file copy; MariaDB: dump+gzip, auto-prune)
- [x] **Backup management** (`backup`, `backup:list`, dashboard one-click)
- [x] **Profiles** (`profile:create`, `profile:list`, `profile:delete`)

### Phase 5 — Services framework & PostgreSQL (v0.6) ✅ COMPLETE
- [x] **Generic `Spec`-driven service framework** (`internal/services`): binary
      discovery (bundled + PATH fallback), ports, args, readiness probe,
      console UI, PHP env injection via `proxy.EnvProvider`.
- [x] **Mailpit** as the first service spec (SMTP catcher + web UI).
- [x] **Redis** hybrid: Windows bundled port (`add redis`) + system
      `redis-server` fallback on Linux/macOS.
- [x] **MinIO** S3-compatible storage (console on :9001) + PHP env
      (`SABDOPALON_S3_*`), round-trip probe (`s3check.php`).
- [x] **Meilisearch** instant search (:7700) + PHP env + probe
      (`meilicheck.php`).
- [x] **PostgreSQL** engine (zonky embedded, `initdb`/start/ready) + PDO probe
      (`pgcheck.php`); Linux/macOS only (no Windows zonky binaries).
- [x] **Services page** in dashboard (runtime toggles, .env snippets) + API
      `GET /api/services`, `POST /api/services/<name>/toggle`.
- [ ] Built-in Cloudflare Tunnel (future)
- [ ] System tray (cross-platform) (future)
- [ ] Auto PATH injection (future)
- [ ] Optional container-lite via Podman (future)

### Phase 6 — One-click install, desktop app & terminal (v0.7) ✅ COMPLETE
- [x] **`internal/bootstrap`** — canonical layout (`EnsureLayout`), first-run
      detection (`FirstRun`, state-based: no `engine.toml` / empty `sites/` /
      no SQLite db), default config writer, wizard banner.
- [x] **Config sentinel** — `config.Load` returns `ErrNotBootstrapped` when
      `engine.toml` is missing; `app.New()` surfaces it, `main.go` routes to
      the wizard or setup-mode.
- [x] **CLI wizard** (`sabdopalon setup` / `init`) — interactive, stdlib-only:
      stack (PHP+MariaDB default, optional PostgreSQL), DB engine, ports,
      sample site, then `pkgmgr.Download` with checksums.
- [x] **DB init marker** — `data/<engine>/.sabdopalon-initialized` after first
      `initialize()`; root-without-password constants
      (`DatabaseRootUser`/`DatabaseRootPassword`) reused by backup + WP template.
- [x] **Setup-mode server** — `--setup-mode` (or bare first run) boots only the
      dashboard on :9900 with `GET /api/setup/status`, `POST /api/setup`
      (async job + `GET /api/setup/job`), no proxy/DB/services.
- [x] **SPA setup wizard** — `/setup` route; `App.tsx` redirects there while
      `bootstrapped == false`.
- [x] **One-click installers** — `scripts/install.sh` (curl|bash: extract →
      `~/sabdopalon` → `~/.local/bin` symlink → wizard) and `install.ps1`
      (Expand-Archive → persistent user PATH → wizard); release bundles now
      ship full layout + installer + `packages.toml` + `.gitkeep` dirs.
- [x] **Desktop app (Tauri v2)** — `desktop/`: Rust shell + Go sidecar with
      `SABDOPALON_DIR` (OS user-data dir, Herd-style), tray menu (open
      dashboard/sites, restart, autostart, quit), NSIS/dmg/deb/AppImage
      bundles in CI (`release.yml` desktop matrix). Window shows the existing
      dashboard; never opens a browser (sidecar runs `--no-open --setup-mode`).
- [x] **Embedded terminal** — `internal/terminal` (PTY via creack/pty, pipe
      fallback on Windows) + `GET /api/terminal/ws` (coder/websocket,
      input/resize frames) + SPA `/terminal` page (xterm.js + fit addon).
- [ ] Code-signing (macOS notarization / Windows Authenticode) — needs paid
      accounts; documented in README.
- [ ] `packages.toml` → `go:embed` (fully self-contained binary) (future)

---

## 8. Configuration reference

### 8.1 engine.toml

```toml
[sabdopalon]
tld = "localhost"              # auto-resolves; "test" needs hosts edit
root = "./sites"
logs = "./logs"
data = "./data"

[proxy]
http_port = 8080
https_port = 8443

[php]
binary = "/path/to/php"         # empty = auto-detect

[database]
engine = "sqlite"              # sqlite | mariadb | mysql | postgresql
path = "./data/sabdopalon.db"

[services]
mailpit = false                # local e-mail catcher (:1025 SMTP, :8025 UI)
redis = false                  # cache/queue (:6379)
minio = false                  # S3-compatible storage (:9000 API, :9001 console)
meilisearch = false            # instant search (:7700)

[dashboard]
enabled = true
port = 9900
```

### 8.2 Environment variables injected into PHP

| Variable | Value |
|---|---|
| `SABDOPALON` | `1` |
| `SABDOPALON_DB_ENGINE` | `sqlite` / `mysql` / `mariadb` / `postgresql` |
| `SABDOPALON_DB_PATH` | absolute path to the SQLite file |
| `SABDOPALON_PG_HOST/PORT/USER/DB` | PostgreSQL connection (engine = postgresql) |
| `SABDOPALON_MAIL_SMTP/UI` | Mailpit SMTP addr + web UI URL (when running) |
| `SABDOPALON_REDIS_HOST/PORT` | Redis (when running) |
| `SABDOPALON_S3_ENDPOINT/KEY/SECRET/BUCKET` | MinIO S3 (when running) |
| `SABDOPALON_MEILI_HOST` | Meilisearch (when running) |

---

## 9. Risks & mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| PHP not installed | Can't serve | Auto-detect + clear error message in `doctor` |
| `.localhost` not supported on old OS | No pretty URLs | Fall back to `127.0.0.1:port` direct access; document min OS versions |
| Port 8080 in use | Proxy won't start | Configurable port in `engine.toml` |
| PHP built-in server limitations | Not production-grade | Documented as dev-only; sufficient for local dev |
| MySQL/MariaDB integration | Complexity | Phase 2; launch-on-demand mirrors PHP approach |

---

## 10. Open questions

1. Default TLD: `.localhost` (no hosts edit) is current default — keep it?
2. Should the proxy auto-detect and use ports 80/443 if available (root only)?
3. Dashboard: keep embedded HTML, or add a JS framework in Phase 3?
4. MariaDB vs MySQL as default when DB daemon support lands? (Lean MariaDB.)
