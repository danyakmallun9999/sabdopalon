# Sabdopalon

> 🐫 Portable, cross-platform local development environment — no Apache/Nginx needed.

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

`v0.5.0` — dashboard-first release. Proxy routing, per-site PHP with version
pinning, multi-PHP (8.1–8.5), SQLite/MariaDB, HTTPS wizard with trust
detection, full web dashboard, project templates, DB backups, profiles,
Mailpit mail catcher, clean URLs on :80/:443 (`enable-ports`), checksum-verified
package installs, and graceful shutdown. Unit tests + CI on Linux/macOS/Windows.

## Prerequisites

**None.** Sabdopalon is fully self-contained — no Go, no PHP, no MariaDB
needed on your system. Just download the binary and run it:

- **PHP** — auto-downloaded on first run (8.4.8, ~8MB, 30+ extensions)
- **MariaDB** — auto-downloaded when you run `sabdopalon add mariadb`
- **Go** — only needed if you want to build from source yourself

> If you already have PHP installed, Sabdopalon detects and uses it
> automatically. The auto-download only triggers if no PHP is found.

## Installation

### Option A: Download pre-built binary (recommended — no Go needed)

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

After installing (download or build), just run it — PHP downloads automatically:

```bash
# 1. Start Sabdopalon — PHP auto-downloads on first run if not found
./sabdopalon
#    You'll see: "⬇ PHP not found — downloading automatically..."
#    Then: "✓ PHP ready: bin/php/php" (PHP 8.4.8, ~8MB, with 30+ extensions)
#    Press Ctrl+C to stop.

# 2. (Optional) Use MariaDB instead of the default SQLite
./sabdopalon add mariadb          # auto-downloads MariaDB 11.4.12
#    Then edit config/engine.toml:
#    [database]
#    engine = "mariadb"

# 3. (Optional) Enable HTTPS with trusted local certs
./sabdopalon ssl:ca              # generate local root CA
./sabdopalon ssl:wildcard        # issue *.localhost wildcard cert
sudo ./sabdopalon ssl:trust      # install CA into OS trust store (Linux/macOS)
#    On Windows, run as Administrator:
#    sabdopalon.exe ssl:trust

# 4. (Optional) Create a new project from a template
./sabdopalon new blank myapp
#    Templates: blank, laravel (needs composer), wordpress, codeigniter (needs composer)

# 5. Check everything is ready
./sabdopalon doctor
#    Shows: PHP version, proxy ports, DB engine, discovered sites
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
                 │  Host: example-app.localhost              │
                 │     ▼  (first access: lazy-start)        │
                 │   ReverseProxy ──────────────────────▶   php -S :9001 -t sites/example-app/public
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

The dashboard binds to `127.0.0.1` only and is the single control surface:

| Page | What you can do |
|---|---|
| 🌐 Sites | Create from templates, open, start/stop/restart, delete → `.trash/` |
| 🗄️ Database | Engine status, one-click backups + retention pruning |
| 📦 Packages | Install MariaDB, Mailpit, PHP 8.1–8.5 with live progress |
| 🔒 SSL | CA → wildcard → trust wizard; detects stale/untrusted CAs |
| ⚙️ Settings | TLD, ports, DB engine, auto-open, Mailpit toggle; apply profiles |
| 📜 Logs | Live per-site PHP, DB and Mailpit logs |

JSON API (used by the UI, also handy for scripting): `/api/status`, `/api/sites`
(GET/POST), `/api/sites/<name>/start|stop|restart` (POST), `/api/sites/<name>`
(DELETE), `/api/packages`, `/api/packages/install` + `/api/packages/job`,
`/api/ssl` (+ `/ca`, `/wildcard`, `/trust`), `/api/config` (GET/PUT),
`/api/profiles` (+ `/apply`), `/api/services`, `/api/backup(s)`, `/api/logs/<name>`.

### Clean URLs (`https://myapp.localhost` — no port)

Sabdopalon automatically tries ports 80/443 first. Without privileges it falls
back to 8080/8443. To allow low ports permanently:

```bash
./sabdopalon enable-ports   # Linux: sudo setcap cap_net_bind_service=+ep <binary>
```

## Environment variables (available in PHP)

| Variable | Example |
|---|---|
| `SABDOPALON` | `1` |
| `SABDOPALON_DB_ENGINE` | `sqlite` / `mariadb` / `mysql` |
| `SABDOPALON_DB_PATH` | `/path/to/sabdopalon.db` (sqlite only) |

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
│   ├── database/    # MariaDB/MySQL daemon lifecycle manager
│   ├── pkgmgr/      # package downloader (download, verify, extract)
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
