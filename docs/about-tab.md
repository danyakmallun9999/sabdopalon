# About tab — design (architecture and UI/UX)

Status: **phases 1 to 3 built and merged; phase 4 is three quarters built.** What exists
now, and what each piece is verified by, is recorded below. Nothing in the design is
outstanding except the signed updater, which waits on a keypair only the maintainer
should generate, and the live test that needs two releases.

Implemented, on `main`:

| Piece | Verified by |
|---|---|
| `GET /api/about` — identity, paths, credits, outbound hosts, log facts | `handlers_about_test.go` |
| About page: Identity, Updates, Stack, Paths (Copy **and** Open), Credits, Privacy, Diagnostics | UI type-check, build, and `cargo check` for the Open command |
| `GET`/`POST /api/update` — check and its result | `handlers_update_test.go` (test server, never GitHub) |
| Beta channel that sees pre-releases | `TestBetaChannelSeesPreReleases`, `TestBetaChannelRejectsEmptiness` |
| `POST /api/update/download` + progress, sha256-verified | `internal/update` tests and `TestDownloadEndpoint*` |
| CI publishes a sidecar asset only when the shell did not change | the `git diff` rule run against real history |
| Shell commands: `apply_sidecar_update`, `apply_installer_update`, `open_path` | `cargo check` |
| Prefilled bug report carrying the diagnostics | UI type-check and build |

Not implemented: the signed updater (section 6), and the live application of either
update path.

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
  card. **Built differently from the first draft:** the channel travels with the check
  request and is remembered in the browser's localStorage, rather than living in the
  config file. Following beta is a preference of whoever is using that dashboard, not a
  property of the installation, so it needs no config key and no migration when it
  changes. The chip beside the version shows the channel you follow; the handler still
  reports the channel a build came from, which is a different axis and, today, always
  the same answer. Moving it into Settings later is a small change if it is ever wanted.

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

## 6. Signed updates — the one step that needs a key

Phase 4's last piece replaces sha256-only verification with a signature: Tauri's own
updater (`tauri-plugin-updater`) verifies a minisign signature on every artifact, so
the guarantee no longer rests on TLS to the same origin that served the file.

It is **not switched on yet, deliberately**. Setting `createUpdaterArtifacts: true`
makes the Tauri build require a signing key, and with no key present that build fails —
which would take the working release pipeline down to tick a box. So the key comes
first, and the order below matters.

**What only the maintainer can do (once, on a trusted machine):**

1. `cd desktop && pnpm tauri signer generate -w ~/.tauri/sabdopalon.key`
   (or `cargo tauri signer generate`). It prints a **public** key and writes a private
   one. Keep the private key off the repository; treat it like a password, because it
   is what makes an update trustworthy.
2. Add two repository secrets:
   `TAURI_SIGNING_PRIVATE_KEY` (the private key contents) and
   `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` (the password chosen above, if any).

**What I will do once those exist:**

1. `desktop/src-tauri/Cargo.toml`: add `tauri-plugin-updater`.
2. `main.rs`: register the plugin.
3. `capabilities/default.json`: add the updater permission for the dashboard origin.
4. `tauri.conf.json`: `plugins.updater.pubkey` (the public key) and
   `plugins.updater.endpoints`, plus `bundle.createUpdaterArtifacts: true`.
5. `release-windows.yml`: pass the two secrets to the Tauri build step, and let the
   release carry the generated `latest.json` and `.sig` files alongside the installer.
6. About: prefer the signed path when the release carries a signature, and keep the
   current sha256 path as the fallback for releases that do not.
7. Verify by building a test tag and watching an update apply on a machine that has the
   previous version installed.

Until then the sha256 verification already in place stays the guarantee, and it is a
real one: the hash comes from the same release metadata that names the file, and a
mismatch is refused before anything runs.

## 7. Phasing

| Phase | Scope | Value |
|---|---|---|
| 1 | About tab with identity, stack, paths, credits, diagnostics, privacy + `GET /api/about` | Trust, support, and the credits you asked for — no update logic |
| 2 | Check for updates, banner, release notes, "open release page" | Never miss a release; still manual |
| 3 | Sidecar fast path: download, verify, swap via shell, rollback | Seconds instead of a 59 MB installer |
| 4 | Installer path, channels (beta), prefilled issue, optional signed updater | Complete update story |

Phase 1 is independently useful and carries no risk to the current install, which is
the right place to start.
