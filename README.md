# Sabdopalon

> 🐫 Portable, cross-platform local development environment — no Apache/Nginx needed.

Sabdopalon is a clean-room, open-source (MIT) alternative to Laragon/XAMPP.
Instead of bundling heavy web server binaries, it uses a **Go reverse proxy**
that routes `*.localhost` domains to per-site PHP built-in servers — one process
per project, started on demand. Databases are handled by SQLite (zero-setup) or
MariaDB (auto-downloaded & managed). Includes an interactive web dashboard,
project templates, database backups, SSL trust store integration, and profiles.

**Why this is better than Laragon/XAMPP:**

| | Laragon v7 | XAMPP | **Sabdopalon** |
|---|---|---|---|
| Price | Paid ($49/yr+), nags if free | Free | **Free forever (MIT)** |
| Web server | Bundled Apache/Nginx | Bundled Apache | **Go proxy — nothing to install** |
| OS | Windows only | Win/Mac/Linux | **Win/Mac/Linux (one codebase)** |
| Pretty URLs | ✓ (needs hosts edit) | ✗ | **✓ (.localhost auto-resolves)** |
| Per-site PHP | ✓ | ✗ | **✓ (one server per site)** |
| DB setup | Manual MySQL config | Manual MySQL config | **SQLite zero-setup OR MariaDB auto-managed** |
| HTTPS | Complex | ✗ | **✓ one-command wildcard cert** |
| Dashboard | ✗ | ✗ | **✓ interactive web UI (port 9900)** |
| Project templates | ✗ | ✗ | **✓ blank, Laravel, WordPress, CodeIgniter** |
| DB backups | ✗ | ✗ | **✓ one-command + auto-prune** |
| Profiles | ✓ (v7+) | ✗ | **✓ multiple PHP/DB version sets** |
| Startup weight | Heavy | Heavy | **Light (proxy + PHP on demand)** |

## Status

`v0.4.0` — **feature-complete**. All Phase 1-4 features implemented and tested.
Proxy routing, per-site PHP, SQLite, MariaDB daemon, HTTPS, interactive dashboard,
project templates, DB backups, profiles, SSL trust store, and graceful shutdown
all work. Tested with PHP 8.5 + MariaDB 11.4.12 on Linux Mint.

## Prerequisites

Sabdopalon needs **two things** to run:

1. **PHP** — the runtime for your sites (Sabdopalon manages the proxy, not PHP itself)
2. **Go 1.22+** — only needed to *build* Sabdopalon from source (not to run it)

### Install PHP

**Linux (Debian/Ubuntu/Mint):**
```bash
sudo apt update && sudo apt install -y php-cli php-sqlite3 php-mysql php-curl php-mbstring php-zip php-gd php-xml
php -v   # verify: should show PHP 8.x
```

**macOS:**
```bash
# Option A: Homebrew
brew install php
php -v

# Option B: Laravel Herd (includes PHP + GUI manager)
# Download from https://herd.laravel.com
```

**Windows:**
```bash
# Option A: Download PHP from https://windows.php.net/download/
# Extract to C:\php, add C:\php to PATH
php -v

# Option B: Use Laragon/Herd for Windows to get PHP, then run Sabdopalon separately

# Option C: Scoop
scoop install php
```

> Sabdopalon auto-detects PHP from PATH. If you have multiple PHP versions,
> set the exact path in `config/engine.toml` → `[php] binary = "..."`.

## Installation

### Option A: Build from source (all platforms)

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

### Option B: Pre-built binary (no Go needed)

Download the latest release for your OS from the
[Releases page](https://github.com/danyakmallun9999/sabdopalon/releases),
extract it, and you're ready — no build step required.

```bash
# Example (Linux):
curl -L https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/sabdopalon-linux-amd64 -o sabdopalon
chmod +x sabdopalon
./sabdopalon version
```

### Make it available system-wide (optional)

```bash
# Linux / macOS: symlink to /usr/local/bin
sudo ln -s "$(pwd)/sabdopalon" /usr/local/bin/sabdopalon
# Now you can run 'sabdopalon' from any directory

# Windows: add the folder to PATH, or copy sabdopalon.exe to a PATH directory
```

## Quick start

After installation (build or download), run these steps once:

```bash
# 1. Check that everything is ready
./sabdopalon doctor
#    This shows: PHP version, proxy ports, DB engine, discovered sites
#    If PHP is not found, edit config/engine.toml -> [php] binary = "/path/to/php"

# 2. (Optional) Download MariaDB — Sabdopalon manages it automatically
./sabdopalon add mariadb
#    Then set engine in config/engine.toml:
#    [database]
#    engine = "mariadb"

# 3. (Optional) Enable HTTPS with trusted local certs
./sabdopalon ssl:ca          # generate local root CA
./sabdopalon ssl:wildcard    # issue *.localhost wildcard cert
sudo ./sabdopalon ssl:trust  # install CA into OS trust store (Linux/macOS)
#    On Windows, run as Administrator:
#    sabdopalon.exe ssl:trust

# 4. (Optional) Create a new project from a template
./sabdopalon new blank myapp
#    Templates: blank, laravel (needs composer), wordpress, codeigniter (needs composer)

# 5. Start everything — proxy + DB + dashboard
./sabdopalon
#    Press Ctrl+C to stop. All PHP servers + DB daemon shut down gracefully.
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

| Command | Description |
|---|---|
| `sabdopalon` (or `serve`) | Start proxy + DB + dashboard (default) |
| `sabdopalon doctor` | Check config, PHP binary, ports, DB |
| `sabdopalon sites` | List discovered sites + URLs |
| `sabdopalon new <tmpl> <name>` | Create project (templates: blank, laravel, wordpress, codeigniter) |
| `sabdopalon backup` | Create a database backup now |
| `sabdopalon backup:list` | List existing backups |
| `sabdopalon add <pkg>` | Download & install a package (e.g. `mariadb`) |
| `sabdopalon pkg:list` | List available packages + install status |
| `sabdopalon ssl:ca` | Generate local root CA |
| `sabdopalon ssl:wildcard` | Issue `*.<tld>` wildcard cert for HTTPS |
| `sabdopalon ssl:issue <host>` | Issue a certificate for a specific host |
| `sabdopalon ssl:trust` | Install root CA into OS trust store |
| `sabdopalon profile:list` | List all profiles |
| `sabdopalon profile <name>` | Show a profile's settings |
| `sabdopalon profile:create <name> [php] [db] [desc]` | Create a profile |
| `sabdopalon profile:delete <name>` | Delete a profile |
| `sabdopalon vhost` | Print reference Apache vhosts |
| `sabdopalon version` | Print version |
| `sabdopalon help` | Show help |

## Dashboard

The interactive dashboard runs on `http://localhost:9900/` and provides:

- **Status panel** — version, uptime, PHP, DB, ports
- **Sites list** — running/stopped status, click-through links (HTTP + HTTPS)
- **Backups** — one-click backup creation, list of existing backups
- **Logs viewer** — per-site PHP logs, auto-refreshed

API endpoints:
- `GET /api/status` — system status JSON
- `GET /api/sites` — discovered sites + running status
- `GET /api/logs/<sitename>` — last 100 log lines
- `POST /api/backup` — trigger a database backup
- `GET /api/backups` — list existing backups

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
