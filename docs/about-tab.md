# About tab — design (architecture and UI/UX)

Status: **designed, not implemented.** Nothing here changes behaviour yet.

An About page is where a local dev tool earns or loses trust: it is the screen a
user opens when something is wrong. It should therefore answer four questions
without anyone having to ask: *what am I running*, *what is it doing to my machine*,
*what does it talk to over the network*, and *how do I update or report a problem*.

## 1. What the tab contains

Seven cards, in this order. Update sits at the top because it is the only card that
ever demands action; everything else is reference material.

| # | Card | Contents |
|---|---|---|
| 1 | **Identity** | App icon, name, `0.11.19`, channel (stable/beta), platform chip (Windows x64), built from commit + date, architecture line (Tauri shell + Go sidecar), "Copy version" |
| 2 | **Update** | State machine UI described in section 4, the primary action of the page |
| 3 | **Stack** | PHP, MariaDB, PostgreSQL, Mailpit, MinIO, Meilisearch, cloudflared, Node, Composer: version, status dot, port, and a "Manage" link to Packages |
| 4 | **Paths** | Program directory, data root, sites, logs, config, PHP ini, TLS CA — each with Copy and Open |
| 5 | **Credits** | Author, repository, license, issue tracker, changelog, third-party notices (collapsed) |
| 6 | **Diagnostics** | A plain-text block and one button: "Copy diagnostics" / "Report a bug" (prefilled issue) |
| 7 | **Privacy** | "No telemetry" plus the exact list of outbound hosts and why |

## 2. Credits (as requested)

Rendered as its own card, first line prominent, not buried in small print:

```
Sabdopalon
Built by danyakmallun
https://github.com/danyakmallun9999/sabdopalon
```

Rows below it:

- **Author** — danyakmallun, linking to the GitHub profile.
- **Repository** — the URL above, with "Star on GitHub" and "Report an issue".
- **License** — read from the repository's LICENSE file at build time so the tab can
  never claim the wrong one. *(The license still has to be confirmed when this is
  implemented; it is deliberately not hard-coded here.)*
- **Third-party notices** (collapsed, with each project's own license):
  PHP (PHP License), MariaDB (GPLv2), PostgreSQL (PostgreSQL License), Cloudflare
  cloudflared (Apache-2.0), Mailpit (MIT), Meilisearch (MIT), Node (MIT), Composer
  (MIT), Redis (BSD-3-Clause for the Windows build), MinIO (AGPLv3 — worth stating
  plainly, since it is the one copyleft dependency we ship), Go, React, Vite,
  Tailwind, shadcn/ui, Tauri.
  Each keeps its own terms; Sabdopalon only bundles them.

Wording for the in-app footer line: `Sabdopalon — built by danyakmallun`. The tab
title itself stays "About", so it reads naturally in the sidebar.

## 3. Architecture

### 3.1 Where the data comes from

One new endpoint, `GET /api/about`, returning a snapshot rather than several calls:

```
{
  "app":      { version, channel, commit, built_at, platform, arch, shell_version },
  "stack":    [ { name, version, installed, running, port, kind } ],
  "paths":    { program, data, sites, logs, config, php_ini, ca },
  "logging":  { file, level, recent_errors },
  "privacy":  { hosts: [ ... ] },
  "credits":  { author, repo, license, build_info }
}
```

`/api/status` keeps serving the fast-changing values (ports, db_states); About is a
snapshot page and must not be polled.

### 3.2 Where the update engine lives

The split follows one hard Windows fact: **a running executable cannot replace
itself.** So:

- **Go sidecar** — checks GitHub Releases, compares versions, selects the asset,
  downloads to a staging path, verifies sha256, reports progress, keeps the previous
  sidecar for rollback. It already has every one of these patterns (pkgmgr does
  verified downloads today).
- **Tauri shell** — applies the update, because it is the process that survives the
  sidecar's restart: stop sidecar → swap the file → start sidecar. For the installer
  path it downloads, runs the installer, quits, and comes back.
- **Dashboard** — talks to the sidecar over HTTP for check/download/verify, and asks
  the shell to apply. *(How the dashboard reaches the shell today — Tauri command
  bridge versus a sidecar-mediated action — must be confirmed against the code
  before this is built; the design does not assume it.)*

State is a small machine, reported by `GET /api/update` and driven by
`POST /api/update/{check,download,apply,cancel}`:

```
idle → checking → up-to-date
                → available → downloading(0..100) → verifying → ready → applying → restarted
                → error(reason)
```

### 3.3 Which asset, and why it decides the path

The release workflow publishes a bare sidecar binary **only when the desktop shell
did not change**, decided by:

```
git diff --name-only <previous-tag> HEAD -- desktop/src-tauri
```

Empty → the release carries a sidecar asset → the fast path applies (about ten
seconds, no installer, no full restart of the app).
Non-empty → no sidecar asset → the installer path applies.

So the presence of the asset *is* the manifest; no extra metadata file is needed, and
the hash comes from GitHub's own `digest` field. History says this covers most
releases: seven of the nine releases from v0.11.9 to v0.11.17 touched only Go and the
embedded UI.

### 3.4 Safety rules (non-negotiable)

1. **Verify before running anything.** sha256 compared with the release digest; a
   mismatch files nothing and reports.
2. **Never update silently.** A dev tool holding a database and running servers must
   not restart itself under the user's hands. Check on start (opt-out in Settings),
   apply only on an explicit click.
3. **Warn about what stops.** Installing stops sites, the PHP workers and the database
   daemons for roughly 30–60 seconds. The card says so before the button is pressed,
   and the button is disabled while a deploy or dev-tool run is in progress.
4. **Keep a way back.** The replaced sidecar is retained as `sabdopalon.exe.prev` and
   About offers "Roll back" until the next successful start.
5. **The stack is never auto-updated.** PHP, MariaDB and PostgreSQL move only when the
   user installs a version from Packages. The updater replaces program code and
   nothing else.
6. **Data is untouched by definition.** The program lives in `%LOCALAPPDATA%`, data in
   `C:\sabdopalon`; an update never runs setup again.

### 3.5 CI changes this needs

- Publish the sidecar binary as a second release asset, guarded by the `git diff`
  above. (The workflow currently publishes exactly one asset on purpose, so the
  download page stays unambiguous — that trade-off has to be accepted knowingly.)
- Attach the release notes already in `CHANGELOG.md` to the release body, so the
  About card can show "what changed" without a second source of truth.
- Later, if Sabdopalon is distributed more widely: Tauri's signed updater
  (`tauri-plugin-updater`, minisign) instead of sha256-only verification.

## 4. UI/UX

### 4.1 Update card states

| State | Shows | Primary action |
|---|---|---|
| Up to date | "You are on the latest version (0.11.19 · stable). Last checked 2 min ago." | "Check again" (secondary) |
| Update available (fast path) | "0.11.20 is available — updates in about 10 seconds, without closing Sabdopalon. Dashboard, proxy and CLI are replaced." + release notes (3 lines, "Show all") | "Update now" |
| Update available (installer path) | "0.11.20 is available. This one includes shell changes, so it installs like a normal update: Sabdopalon closes and reopens, roughly a minute. Your sites and databases stop and come back." | "Update now" |
| Downloading | Progress bar with bytes, transfer rate, cancel | "Cancel" |
| Verifying | "Verifying checksum" (short, no bar theatre) | — |
| Ready | "Ready to update. Save your work — Sabdopalon will restart." | "Restart and update" |
| Applying | Spinner; the app is about to close | — |
| Error | The concrete reason plus the manual fallback link to the release page | "Try again" / "Open release page" |
| Offline / rate-limited | "Couldn't reach GitHub. Releases are checked at most hourly." | "Try again" |

### 4.2 Visual rules

- One primary action per card; the Update card is the only one that may use the
  accent colour for a button.
- State is never colour alone: an icon and a word accompany every dot (running,
  stopped, up to date, update available).
- Progress is `aria-live="polite"`; the apply button is keyboard reachable and is
  disabled while a deploy or dev-tool run is active, with the reason in its tooltip.
- Respect reduced motion: no animated spinner when the user has asked for reduced
  motion, use a static "working…" indicator instead.
- Channel is a small chip next to the version, never a dropdown inside the identity
  card — changing it belongs in Settings.

### 4.3 Copy tone

Short, specific, no marketing. Examples:

- Up to date: `You are on the latest version (0.11.19 · stable).`
- Fast path: `Updates in about 10 seconds, without closing Sabdopalon.`
- Installer path: `Includes shell changes, so Sabdopalon closes and reopens — about a
  minute. Sites and databases stop and come back.`
- Restart warning: `Save your work. Sabdopalon will restart.`
- Failure: the actual error, then `Open release page`.

### 4.4 Language

The dashboard is written in English today, so About should be English too — a single
Indonesian card in an English UI reads like an oversight rather than a choice. If the
dashboard is ever localised, About follows with everything else. *(Say the word and I
will write this page in Indonesian instead; it just should not be half of each.)*

### 4.5 Diagnostics block

A copyable snapshot, tuned for a bug report rather than for completeness:

```
Sabdopalon 0.11.19 (stable) · Windows 11 26100 · x64
Shell 0.11.19 · sidecar 0.11.19 · built 2026-10-11 from a051a18
Data root: C:\sabdopalon (user override: none)
PHP 8.5.5 (62 extensions) · php.ini: C:\sabdopalon\config\php.ini
MariaDB 11.4.13 :3306 (running) · PostgreSQL 17.10 :5433 (running)
Proxy :80 and :443 (bound) · dashboard :9900 · 2 sites running, 8 workers
TLS CA installed: yes · cloudflared 2026.10.0 (idle)
PATH: C:\sabdopalon\bin first, 4 dead entries, shadowed by: none
Recent errors: 2 in the last 24h (php/worker.log)
```

That "PATH" line is not decoration: it is the exact class of problem this session
spent hours on, and it is the first thing a maintainer would otherwise have to ask
for.

## 5. Phasing

| Phase | Scope | Value |
|---|---|---|
| 1 | About tab with identity, stack, paths, credits, diagnostics, privacy + `GET /api/about` | Trust, support, and the credits you asked for — no update logic |
| 2 | Check for updates, banner, release notes, "open release page" | Never miss a release; still manual |
| 3 | Sidecar fast path: download, verify, swap via shell, rollback | Seconds instead of a 59 MB installer |
| 4 | Installer path, channels (beta), prefilled issue, optional signed updater | Complete update story |

Phase 1 is independently useful and carries no risk to the current install, which is
the right place to start.
