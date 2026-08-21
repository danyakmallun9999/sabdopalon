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

## Quick start

```bash
# Build
cd sabdopalon
go build -o sabdopalon ./cmd/sabdopalon

# Configure: point to your PHP binary (if not auto-detected)
# Edit config/engine.toml -> [php] binary = "/path/to/php"

# Check everything
./sabdopalon doctor

# (Optional) Download MariaDB
./sabdopalon add mariadb

# (Optional) Enable HTTPS
./sabdopalon ssl:ca
./sabdopalon ssl:wildcard
sudo ./sabdopalon ssl:trust    # install CA into OS trust store

# Create a new project from template
./sabdopalon new blank myapp

# Start everything (proxy + DB + dashboard; Ctrl+C to stop)
./sabdopalon

# Open in browser:
#   http://localhost:9900/              ← interactive dashboard
#   http://localhost:8080/              ← dashboard (legacy)
#   http://example-app.localhost:8080/  ← your site (HTTP)
#   https://example-app.localhost:8443/ ← your site (HTTPS)
```

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
