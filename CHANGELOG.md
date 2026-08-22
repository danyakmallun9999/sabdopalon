# Changelog

All notable changes to Sabdopalon are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/); versioning is semver-ish.

## [0.7.0] — 2026-08-22

### One-click install + desktop app + terminal release

### Added
- **One-click installers** — `scripts/install.sh` (curl|bash) & `install.ps1`
  (irm|iex): download → extract to `~/sabdopalon` → PATH → setup wizard.
- **Full release bundles** — archives now ship binary + default
  `config/engine.toml` + `packages/packages.toml` + `.gitkeep` data dirs +
  installers; version injected via ldflags from the tag.
- **Interactive CLI wizard** (`sabdopalon setup`) — stack (PHP + MariaDB
  default, optional PostgreSQL), DB engine, ports, sample site; runs
  automatically on first launch (config sentinel `ErrNotBootstrapped`).
- **Setup-mode server** — config-less boot serves only the dashboard with
  `GET /api/setup/status`, `POST /api/setup` (+ `/api/setup/job` progress).
- **SPA setup wizard** at `/setup` — auto-redirect while not bootstrapped.
- **`SABDOPALON_DIR`** override — Herd-style user-data dir for desktop mode.
- **Desktop app (Tauri v2)** in `desktop/` — native window wrapping the
  dashboard, tray menu, autostart, GUI wizard on first run; CI matrix builds
  NSIS/dmg/deb/AppImage.
- **Embedded terminal** — `internal/terminal` (PTY) + `/api/terminal/ws`
  (WebSocket) + SPA `/terminal` page (xterm.js).
- **DB init marker** — `data/<engine>/.sabdopalon-initialized` after first
  initialization; `database.DatabaseRootUser/Password` constants reused by
  backup and WordPress template (root, no password — XAMPP-style).

### Changed
- `config.Load` returns `ErrNotBootstrapped` (not a hard error) when
  `engine.toml` is missing; `main.go` routes first run to the wizard or
  setup-mode.
- `app.New()`/`serve()` call `bootstrap.EnsureLayout` (fixes missing
  `sites/` on fresh installs) and create `cfg.Root`.
- Go deps added: `creack/pty` (terminal), `coder/websocket` (terminal WS).

### Fixed
- Fresh install no longer dies with "no such file or directory" — layout is
  self-healing.

## [0.6.0] — 2026-08-22

### Services & PostgreSQL release

### Added
- **Generic service framework** (`internal/services`): declarative `Spec`
  (binary discovery, ports, args, readiness probe, console UI, PHP env
  injection) — adding a service is one spec + one package entry.
- **Services dashboard page**: live runtime toggles for Mailpit / Redis /
  MinIO / Meilisearch, install hints, open consoles, and Laravel `.env`
  snippets with a copy button.
- **Redis** hybrid: bundled Windows port (`sabdopalon add redis`, tporadowski
  5.0.14.1 zip) + system `redis-server` fallback on Linux/macOS; injects
  `SABDOPALON_REDIS_HOST/PORT` into PHP.
- **MinIO** S3-compatible storage (API :9000, console :9001, health probe) +
  `SABDOPALON_S3_*` env into PHP; round-trip probe `s3check.php`.
- **Meilisearch** instant search (:7700, health probe) + `SABDOPALON_MEILI_HOST`
  env; probe `meilicheck.php`.
- **PostgreSQL** engine: zonky embedded 17.10 (Linux/macOS), `initdb` +
  managed daemon on :5432, TCP readiness, PDO probe `pgcheck.php`.
- **Adminer** package (`sabdopalon add adminer`) — single-file DB web GUI
  served at `http://adminer.localhost`.
- `/api/status` now reports a generic `services` flag (any service running)
  instead of a Mailpit-specific key.

### Changed
- Settings page no longer carries a hardcoded Mailpit toggle — services live
  on the dedicated Services page.
- `pkgmgr.Download` gives a clear error when a package has no download source
  for the current platform (e.g. Redis outside Windows).

### Fixed
- `sabdopalon add redis` on non-Windows now explains the system redis path
  instead of failing obscurely.

## [0.5.0] — 2026-08-22

### Dashboard-first release — manage everything from the browser

### Added
- **Web dashboard control panel** (binds `127.0.0.1:9900`): create sites from
  templates, start/stop/restart sites, delete-to-`.trash/`, install packages
  with live progress, SSL wizard, settings editor, profiles, live logs.
  UI rebuilt as multi-page `html/template` + `go:embed` assets (`web/`).
- **Multi-PHP**: bundled versions live in `bin/php/<X.Y>/php`; registry ships
  PHP 8.1–8.5; per-site pinning via `.sabdopalon.yml` (`php: "8.3"`); new
  commands `add php@<ver>` and `php:list`; legacy layout auto-migrates.
- **Mailpit** mail catcher package + supervised service (`[services] mailpit`);
  injects `SABDOPALON_MAIL_SMTP` / `SABDOPALON_MAIL_UI` into PHP.
- **HTTPS that just works**: startup warning when the local CA is not trusted;
  trust status with fingerprint match detection (stale-trust after CA
  regeneration); one-click Trust with exact sudo instructions when elevation
  is missing; Firefox/NSS note; TLS handshake log noise suppressed to once.
- **Clean URLs**: proxy tries ports 80/443 first and falls back automatically;
  `sabdopalon enable-ports` grants `cap_net_bind_service` on Linux.
- **Per-site `.sabdopalon.yml` wiring**: docroot override, custom env vars and
  alias domains are now honoured by the proxy (previously parsed only).
- `--profile <name>` flag actually applies profile overlays at serve-time.
- Minimal serve output (one headline line) + auto-open browser
  (`[dashboard] auto_open`, default true); `--verbose` restores details.
- Checksum hardening: per-platform pinned SHA-256 for PHP/MariaDB/Mailpit plus
  automatic lockfile verification for unpinned artifacts on re-install.
- Unit tests (config, pkgmgr, proxy, siteconfig, toml, dashboard) and a CI
  workflow (`gofmt` + `vet` + `test -race` on Linux/macOS/Windows).

### Changed
- `config/engine.toml` regenerated by the dashboard via new `Save()`
  (canonical format; no longer ships a machine-specific PHP path).
- Bare `localhost` now redirects to the dashboard instead of a second page.
- Site start/stop events print only with `--verbose`.

### Fixed
- Version string drift (`0.3.0-dev`) — now `0.5.0` everywhere.
- Dashboard mutex crash when two package installs raced.
- Partial config updates no longer wipe unspecified boolean settings.

## [0.4.0]
- Database backups (SQLite copy / MariaDB dump+gzip) with auto-prune,
  environment profiles, pre-built release binaries for 5 platforms.

## [0.3.0]
- Interactive dashboard v1, project templates, OS trust-store installer,
  per-site `.sabdopalon.yml` parser, profile management.

## [0.2.0]
- HTTPS proxy on :8443 with wildcard certs, MariaDB daemon lifecycle,
  package downloader with SHA-256 verification.

## [0.1.0]
- Initial MVP: multiplexing reverse proxy, per-site PHP servers, SQLite,
  pretty `*.localhost` URLs, graceful shutdown.
