<div align="center">
  <img src="images/logo-text.png" alt="Sabdopalon" width="340">
  <p><strong>Portable, cross-platform local development environment — no Apache/Nginx needed.</strong></p>
  <p>Free forever (MIT) · Alternative to Laragon / XAMPP / Herd</p>
</div>

---

Sabdopalon is a clean-room, open-source (MIT) alternative to Laragon/XAMPP/Herd.
Instead of bundling heavy web server binaries, it uses a **Go reverse proxy**
that routes `*.localhost` domains to per-site PHP built-in servers — one process
per project, started on demand. Databases are handled by SQLite (zero-setup) or
MariaDB (auto-downloaded & managed).

**Dashboard-first:** run `sabdopalon` once and everything happens in the browser —
create sites from templates, start/stop them, install PHP versions / MariaDB /
Mailpit, set up trusted HTTPS in three clicks, edit settings, browse logs and
take database backups. No config files required.

**Why this is better than Laragon/XAMPP:**

| | Laragon v7 | XAMPP | **Sabdopalon** |
|---|---|---|---|
| Price | Paid ($49/yr+), nags if free | Free | **Free forever (MIT)** |
| Web server | Bundled Apache/Nginx | Bundled Apache | **Go proxy — nothing to install** |
| OS | Windows only | Win/Mac/Linux | **Win/Mac/Linux (one codebase)** |
| Pretty URLs | ✓ (needs hosts edit) | ✗ | **✓ (.localhost auto-resolves)** |
| Per-site PHP | ✓ | ✗ | **✓ (one server per site)** |
| DB setup | Manual MySQL config | Manual MySQL config | **SQLite zero-setup OR MariaDB auto-managed** |
| Multi-PHP versions | ✓ | ✗ | **✓ 8.1 – 8.5 side by side, per-site pinning** |
| Mail catcher | Paid/extra | ✗ | **✓ bundled Mailpit** |
| HTTPS | Complex | ✗ | **✓ 3-click wizard, auto trust detection** |
| Dashboard | Basic | phpMyAdmin only | **✓ full control panel — manage everything from the browser** |
| Project templates | ✗ | ✗ | **✓ blank, Laravel, WordPress, CodeIgniter** |
| DB backups | ✗ | ✗ | **✓ one-command + auto-prune** |
| Profiles | ✓ (v7+) | ✗ | **✓ multiple PHP/DB version sets** |
| Startup weight | Heavy | Heavy | **Light (proxy + PHP on demand)** |

## Status

`v0.7.0` — one-click install release. First run opens an interactive setup
wizard (CLI or dashboard); the release bundles a full portable layout plus
one-click installers (`install.sh` / `install.ps1`); a native desktop app
(Tauri v2) wraps the dashboard with tray + autostart + GUI wizard; an
embedded terminal (xterm.js + PTY) runs shells with Sabdopalon's bin/ on
PATH. Everything from v0.6 stays: proxy routing, per-site PHP with version
pinning, multi-PHP (8.1–8.5), SQLite/MariaDB/PostgreSQL, HTTPS wizard,
dashboard, templates, backups, profiles, optional services (Mailpit / Redis
/ MinIO / Meilisearch), clean URLs, checksum-verified installs, unit tests +
CI on Linux/macOS/Windows.

## Prerequisites

**None.** Sabdopalon is fully self-contained — no Go, no PHP, no MariaDB
needed on your system. Just install and run it:

- **PHP** — auto-downloaded by the setup wizard / on first serve
  (8.4, ~8MB, 30+ extensions)
- **MariaDB** — auto-downloaded by the setup wizard (default stack)
- **PostgreSQL** — optional, one click in the wizard
- **Go** — only needed if you want to build from source yourself

> If you already have PHP installed, Sabdopalon detects and uses it
> automatically. The auto-download only triggers if no PHP is found.

## Installation

### Option A: One-click installer (recommended)

```bash
# Linux / macOS:
curl -sSL https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.sh | bash

# Windows (PowerShell):
irm https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.ps1 | iex
```

The installer downloads the bundle for your OS, extracts it to `~/sabdopalon`
(`%USERPROFILE%\sabdopalon` on Windows), adds it to your PATH, and runs the
interactive **setup wizard** — you just answer a few questions (stack: PHP +
MariaDB default, optional PostgreSQL, ports, sample site).

### Option B: Download pre-built binary

Download the latest release for your OS from the
[Releases page](https://github.com/danyakmallun9999/sabdopalon/releases).
Binaries are built automatically via GitHub Actions for all platforms:

| Platform | File |
|---|---|
| Linux x86_64 (Intel/AMD) | `sabdopalon-linux-x86_64.tar.gz` |
| Linux aarch64 (ARM64) | `sabdopalon-linux-aarch64.tar.gz` |
| macOS x86_64 (Intel) | `sabdopalon-macos-x86_64.tar.gz` |
| macOS aarch64 (Apple Silicon) | `sabdopalon-macos-aarch64.tar.gz` |
| Windows x86_64 | `sabdopalon-windows-x86_64.zip` |

Every archive is a **full bundle**: binary + default config + package
registry + empty data dirs + installer scripts. Extract anywhere, then run
`./sabdopalon` — the setup wizard starts automatically on first run.

```bash
# Example (Linux x86_64):
curl -L https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/sabdopalon-linux-x86_64.tar.gz | tar xz
chmod +x sabdopalon
./sabdopalon version

# macOS (Apple Silicon):
curl -L https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/sabdopalon-macos-aarch64.tar.gz | tar xz
chmod +x sabdopalon
./sabdopalon version

# Windows (PowerShell):
# Download sabdopalon-windows-x86_64.zip, extract, then:
.\sabdopalon.exe version
```

### Option B: Build from source (requires Go 1.22+)

```bash
# 1. Clone the repository
git clone https://github.com/danyakmallun9999/sabdopalon.git
cd sabdopalon

# 2. Build the binary
#    Linux / macOS:
go build -o sabdopalon ./cmd/sabdopalon

#    Windows (PowerShell / CMD):
go build -o sabdopalon.exe .\cmd\sabdopalon

# 3. Verify
./sabdopalon version    # Linux/macOS
.\sabdopalon.exe version # Windows
```

### Make it available system-wide (optional)

```bash
# Linux / macOS: symlink to /usr/local/bin
sudo ln -s "$(pwd)/sabdopalon" /usr/local/bin/sabdopalon
# Now you can run 'sabdopalon' from any directory

# Windows: add the folder to PATH, or copy sabdopalon.exe to a PATH directory
```

## Quick start

After installing, the **setup wizard runs automatically** on first launch.
You can also run it manually any time:

```bash
./sabdopalon setup        # interactive wizard (PHP + MariaDB default)
./sabdopalon              # normal start — dashboard + proxy + DB
./sabdopalon doctor       # health check: PHP, ports, database, SSL
```

## Running your sites

```bash
# Start Sabdopalon (blocks until Ctrl+C)
./sabdopalon

# Then open in your browser:
#   http://localhost:9900/              ← interactive dashboard (status, logs, backups)
#   http://localhost:8080/              ← dashboard (proxy port, site list)
#   http://example-app.localhost:8080/  ← your site (HTTP)
#   https://example-app.localhost:8443/ ← your site (HTTPS, if cert generated)
```

### Add a new site

```bash
# Method 1: from a template
./sabdopalon new blank myblog
# → creates sites/myblog/public/index.php
# → visit http://myblog.localhost:8080/

# Method 2: manual
mkdir -p sites/myapp/public
echo '<?php echo "Hello!";' > sites/myapp/public/index.php
# → visit http://myapp.localhost:8080/ (no restart needed)
```

### What gets auto-downloaded

| Component | When | Size | Source |
|---|---|---|---|
| **PHP 8.4.8** | First `sabdopalon` run if no PHP found | ~8 MB | [static-php.dev](https://dl.static-php.dev/static-php-cli/common/) |
| **MariaDB 11.4.12** | `sabdopalon add mariadb` | ~250 MB | [archive.mariadb.org](https://archive.mariadb.org/) |

Both are verified (SHA-256 where available) and extracted into `bin/` — no
system-wide installation, no pollution of your OS, completely portable.

## Services (optional)

Optional local services are managed declaratively and start on demand. Toggle
them in the dashboard **Services** page (applies immediately and persists to
`config/engine.toml` under `[services]`), or set them in the config file:

```toml
[services]
mailpit = false      # local e-mail catcher — SMTP :1025, web UI :8025
redis = false        # cache & queue — :6379
minio = false        # S3-compatible storage — API :9000, console :9001
meilisearch = false  # instant search — :7700
```

| Service | Install | Ports | When enabled, PHP gets |
|---|---|---|---|
| **Mailpit** | `add mailpit` | SMTP 1025 · UI 8025 | `SABDOPALON_MAIL_SMTP`, `SABDOPALON_MAIL_UI` |
| **Redis** | `add redis` (Windows) or system `redis-server` (Linux/macOS) | 6379 | `SABDOPALON_REDIS_HOST`, `SABDOPALON_REDIS_PORT` |
| **MinIO** | `add minio` | API 9000 · Console 9001 | `SABDOPALON_S3_ENDPOINT/KEY/SECRET/BUCKET` |
| **Meilisearch** | `add meilisearch` | 7700 | `SABDOPALON_MEILI_HOST` |

Laravel `.env` snippets for each running service are shown right on the
Services page (Redis cache/queue, MinIO S3 filesystem, Meilisearch Scout) with
a copy button. Verification probes live in `sites/example-app/public/`:
`pgcheck.php`, `s3check.php`, `meilicheck.php`.

### Database engines

- **SQLite** (default) — zero setup, file at `data/sabdopalon.db`.
- **MariaDB** — `sabdopalon add mariadb`, then `engine = "mariadb"` in
  `config/engine.toml`. Managed daemon on `:3306`, Unix socket in `data/`.
- **PostgreSQL** — `sabdopalon add postgresql` (Linux/macOS; Windows needs a
  system install), then `engine = "postgresql"`. Managed daemon on `:5432`,
  superuser `sabdopalon` with trust auth on `127.0.0.1`.

Root credentials follow the XAMPP/Laragon convention: **root user with an
empty password**, only reachable from `127.0.0.1`/the Unix socket — perfect for
phpMyAdmin-style tooling and the bundled Adminer site (`add adminer` →
`http://adminer.localhost`).

### Platform notes

| | Linux | macOS | Windows |
|---|---|---|---|
| `*.localhost` resolves | ✅ automatic | ✅ automatic | ✅ automatic (Win 10+) |
| Needs `/etc/hosts` edit | No | No | No |
| PHP install | `apt install php-cli` | `brew install php` | windows.php.net / Scoop |
| Build command | `go build -o sabdopalon ./cmd/sabdopalon` | same | `go build -o sabdopalon.exe .\cmd\sabdopalon` |
| SSL trust needs | `sudo` | `sudo` | Run as Administrator |
| Binary name | `sabdopalon` | `sabdopalon` | `sabdopalon.exe` |

> **`.localhost` works everywhere**: modern Linux, macOS, and Windows 10+ all
> resolve `*.localhost` to `127.0.0.1` automatically — no `/etc/hosts` editing,
> no admin/root needed.

## How it works

```
                 ┌──────────────────────────────────────────┐
                 │   sabdopalon (Go binary)                 │
                 │                                          │
   browser ─────▶│   HTTP proxy :8080  +  HTTPS :8443       │
                 │     routes by Host header                │
                 │                                          │
                 │   Dashboard :9900 (interactive UI + API) │
                 │                                          │
                 │  Host: example-app.localhost             │
                 │     ▼  (first access: lazy-start)        │
                 │   ReverseProxy ───────▶   php -S :9001 -t sites/example-app/public
                 └──────────────┬───────────────────────────┘
                                │
                 ┌──────────────▼───────────────────────────┐
                 │  MariaDB 11.4.12 (auto-managed)          │
                 │  port 3306, auto-backup, socket in data/ │
                 └──────────────────────────────────────────┘
```

## Commands

Everything below is also clickable in the dashboard — the CLI is optional.

| Command | Description |
|---|---|
| `sabdopalon` (or `serve`) | Start everything and open the dashboard (`--profile <name>`, `--verbose`) |
| `sabdopalon doctor` | Health check: PHP, ports, DB, SSL trust, bundled versions |
| `sabdopalon sites` | List discovered sites + URLs |
| `sabdopalon new <tmpl> <name>` | Create project (templates: blank, laravel, wordpress, codeigniter) |
| `sabdopalon add <pkg>` | Install package: `mariadb`, `mailpit`, `php@8.2` … |
| `sabdopalon pkg:list` | Available packages + install status |
| `sabdopalon php:list` | Bundled PHP versions (8.1–8.5) with active default |
| `sabdopalon ssl:ca` / `ssl:wildcard` / `ssl:issue <host>` / `ssl:trust` | Local HTTPS in four steps |
| `sabdopalon enable-ports` | Allow clean URLs on :80/:443 (Linux setcap helper) |
| `sabdopalon backup` / `backup:list` | Database backups |
| `sabdopalon profile:create/list/delete` | Environment profiles |
| `sabdopalon vhost` | Print reference Apache vhosts |
| `sabdopalon setup` | Re-run the interactive setup wizard |
| `sabdopalon version` / `help` | Meta |

### Per-site configuration (`.sabdopalon.yml`)

Drop this file into any site folder for per-project overrides:

```yaml
php: "8.3"          # version from `add php@8.3`, or a path/PATH command
docroot: public     # document root relative to the site folder
aliases:            # extra domains routed to this site automatically
  - www.myapp.test
env:
  APP_ENV: local
```

## Dashboard

Built with **React + shadcn/ui** (dashboard-01 template, Lucide icons, dark theme).
The compiled bundle is embedded into the Go binary (`go:embed`) — releases stay a
single self-contained file with no Node.js required at runtime.

The dashboard binds to `127.0.0.1` only and is the single control surface:

| Page | What you can do |
|---|---|---|
| 🌐 Sites | Create from templates, open, start/stop/restart, delete → `.trash/` |
| 🗄️ Database | Engine status, one-click backups + retention pruning |
| 🧩 Services | Toggle Mailpit/Redis/MinIO/Meilisearch live, .env snippets, open consoles |
| 📦 Packages | Install MariaDB, PostgreSQL, Mailpit, PHP 8.1–8.5 with live progress |
| 🔒 SSL | CA → wildcard → trust wizard; detects stale/untrusted CAs |
| ⚙️ Settings | TLD, ports, DB engine, auto-open; apply profiles |
| 🖥️ Terminal | Embedded shell (xterm.js + PTY) with bin/ on PATH — run php, mysql, composer |
| 📜 Logs | Live per-site PHP, DB and Mailpit logs |

> **First run / desktop app:** when no config exists yet the dashboard boots
> in **setup mode** and shows the setup wizard at `/setup` (also the landing
> page of the Tauri desktop app).

### Developing / building the dashboard UI

Requires Node.js 20+ (build-time only):

```bash
cd internal/dashboard/ui
npm install
npm run dev        # dev server on :5173 — proxies /api to :9900 (run sabdopalon alongside)
npm run build      # production bundle → dist/ (picked up by the next go build)
```

Then rebuild the binary: `go build -o sabdopalon ./cmd/sabdopalon`.
Without a UI build the binary still works and serves a placeholder page plus the
full JSON API at `/api/*`.

JSON API (used by the UI, also handy for scripting): `/api/status`, `/api/sites`
(GET/POST), `/api/sites/<name>/start|stop|restart` (POST), `/api/sites/<name>`
(DELETE), `/api/packages`, `/api/packages/install` + `/api/packages/job`,
`/api/ssl` (+ `/ca`, `/wildcard`, `/trust`), `/api/config` (GET/PUT),
`/api/profiles` (+ `/apply`), `/api/services`, `/api/setup/status` +
`/api/setup` + `/api/setup/job`, `/api/terminal/ws` (WebSocket PTY),
`/api/backup(s)`, `/api/logs/<name>`.

## Desktop app (Tauri v2)

A native desktop app wraps the same dashboard — a real OS window (WebView2 /
WebKit, no URL bar), tray icon, "Start at Login" autostart, and a **GUI setup
wizard** on first run (no CLI needed, especially on Windows).

- **Windows** — NSIS installer (per-user, no admin), Start Menu shortcut.
- **macOS** — `.dmg` drag-to-Applications, menu-bar app.
- **Linux** — `.deb` + `.AppImage`.

Data lives in the OS user-data dir (Herd-style) via `SABDOPALON_DIR`
(`%LOCALAPPDATA%\Sabdopalon`, `~/Library/Application Support/Sabdopalon`,
`~/.local/share/sabdopalon`) — the app itself installs read-only.

Build it yourself:

```bash
cd desktop
npm install
bash scripts/build-sidecar.sh   # builds the Go sidecar for your platform
npm run dev                     # tauri dev (needs Rust toolchain)
```

> **Signing note:** unsigned macOS/Windows builds show a warning on first
> launch (right-click-open / "More info"). Signing requires a paid Apple
> Developer ID / Authenticode certificate — out of scope for now.

### Clean URLs (`https://myapp.localhost` — no port)

Sabdopalon automatically tries ports 80/443 first. Without privileges it falls
back to 8080/8443. To allow low ports permanently:

```bash
./sabdopalon enable-ports   # Linux: sudo setcap cap_net_bind_service=+ep <binary>
```

## PHP configuration

Every PHP process (all sites) is started with `PHPRC` pointing at
`config/php.ini`, created automatically on first run with sensible defaults:

```ini
memory_limit = 256M
upload_max_filesize = 64M
post_max_size = 64M
max_execution_time = 120
date.timezone = UTC
```

Edit the file and restart Sabdopalon (or just the site) to apply. Per-site
overrides live in `.sabdopalon.yml` (PHP version, docroot, env vars).

## Environment variables (available in PHP)

| Variable | Example |
|---|---|
| `SABDOPALON` | `1` |
| `SABDOPALON_DB_ENGINE` | `sqlite` / `mariadb` / `mysql` / `postgresql` |
| `SABDOPALON_DB_PATH` | `/path/to/sabdopalon.db` (sqlite only) |
| `SABDOPALON_PG_HOST` / `PORT` / `USER` / `DB` | `127.0.0.1` / `5432` / `sabdopalon` / `postgres` (engine = postgresql) |
| `SABDOPALON_MAIL_SMTP` / `SABDOPALON_MAIL_UI` | Mailpit SMTP + UI (when running) |
| `SABDOPALON_REDIS_HOST` / `SABDOPALON_REDIS_PORT` | Redis (when running) |
| `SABDOPALON_S3_ENDPOINT` / `KEY` / `SECRET` / `BUCKET` | MinIO S3 (when running) |
| `SABDOPALON_MEILI_HOST` | Meilisearch (when running) |

## Layout

```
sabdopalon/
├── sabdopalon                 # the binary
├── cmd/sabdopalon/main.go     # entry point
├── internal/
│   ├── app/         # CLI dispatch + all command handlers
│   ├── config/      # engine.toml loader + PHP auto-detect
│   ├── toml/        # std-lib TOML parser (zero deps)
│   ├── proxy/       # multiplexing reverse proxy + PHP manager + HTTPS
│   ├── database/    # MariaDB/MySQL/PostgreSQL daemon lifecycle manager
│   ├── pkgmgr/      # package downloader (download, verify, extract)
│   ├── services/    # optional managed services (Spec framework: mailpit, redis, minio, meilisearch)
│   ├── dashboard/   # interactive web UI + JSON API
│   ├── templates/   # project scaffolding (blank, Laravel, WordPress, CI4)
│   ├── backup/      # database backup (SQLite copy, MariaDB dump+gzip)
│   ├── profiles/    # multiple environment profiles (PHP/DB overrides)
│   ├── trust/       # OS trust store CA installer (cross-platform)
│   ├── siteconfig/  # per-project .sabdopalon.yml parser
│   ├── vhost/       # reference Apache vhost generator
│   └── ssl/         # local CA + per-site certs (crypto/x509)
├── config/engine.toml         # global config
├── config/profiles/           # profile overrides
├── sites/                     # web root: 1 folder = 1 site
├── bin/mariadb/               # downloaded MariaDB (gitignored)
├── data/                      # DB data dirs (gitignored)
├── logs/                      # per-site PHP + DB logs (gitignored)
├── backups/                   # database backups (gitignored)
└── certs/                     # SSL certs (gitignored)
```

## License

MIT — see [LICENSE](LICENSE). Sabdopalon contains no code from Laragon or
XAMPP. It orchestrates open-source components (PHP, MariaDB) you provide or
download via the package system.
