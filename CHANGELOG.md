# Changelog

All notable changes to Sabdopalon are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/); versioning is semver-ish.

## [Unreleased]

## [0.11.0] — 2026-08-27

### Fixed
- **Bundled PHP/Composer/other binaries never detected on Windows** — the
  `lookPathInDir` helper rejected every bundled binary on Windows because Go
  never sets the Unix execute bit (mode `0o111`) there, so `php.exe`,
  `composer.exe`, etc. were never found and `resolveBin` fell through to
  PATH/system tools. On Windows any non-directory file is now accepted
  (extension implies runnable, matching `exec.LookPath`/`PATHEXT`), and the
  `.exe` variant is also probed alongside the bare name. The dying-process
  service test is now skipped on Windows (its stub binary is a `#!/bin/sh`
  script that cannot run there); the behavior stays covered on Unix.

## [0.10.2] — 2026-08-27

### Fixed
- **Setup wizard UI spacing on small screens** — the setup page and install
  panel used generous vertical padding/margins that pushed content below the
  fold on short windows (common on laptops with scaled displays). Padding and
  gaps are now responsive (`py-4`/`gap-3` base, `sm:py-5`/`sm:p-3.5` on larger
  screens), and the install log area shrinks from 180px to 140px minimum on
  small screens so the full status card stays visible during installation.

## [0.10.1] — 2026-08-26

### Fixed
- **System tools (Node.js, npm, Composer, nvm) not detected when launched from
  AppImage/desktop entry** — processes spawned from a desktop entry or AppImage
  never run shell rc files, so their PATH omitted directories added by nvm,
  asdf, Homebrew, volta, etc. `exec.LookPath` then failed to find tools the
  user clearly had installed. A login-shell PATH fallback now spawns
  `$SHELL -lic` once (cached via `sync.Once`, 5s timeout) to reconstruct the
  PATH a real terminal would have, using inline BEGIN/END markers so rc
  greeting noise is stripped reliably. When `exec.LookPath` misses, the
  reconstructed PATH is walked manually to find the binary. Also falls back
  to `/etc/passwd` when `$SHELL` is unset (common under systemd user services
  and AppImage launchers that scrub the environment).

## [0.10.0] — 2026-08-25

### Added
- **System-tool installer (`internal/sysinstall`)** — Sabdopalon can now install
  development tools onto the **user's system** (not into `bin/`) with no
  admin/sudo rights. This first release covers Node.js (`node`) and Composer,
  installable via `sabdopalon pkg:add node` / `sabdopalon pkg:add composer`. On
  Linux/macOS the tool is unpacked to a per-user prefix and the user's shell rc
  gets one idempotent, guarded `export` block so new terminals pick it up; on
  Windows the per-user PATH (HKCU\Environment) is updated and
  `WM_SETTINGCHANGE` is broadcast so new terminals/Explorer inherit it
  without a logoff. The install is checksum-verified, idempotent (re-running
  is a no-op), and safe: a missing rc file is created, and only missing-file
  read errors are ignored so other I/O errors are never silently swallowed.
- **Dashboard Packages page** gained a system-tools section that surfaces
  Node.js/Composer install state and version, and a button to install them
  directly from the UI.

### Fixed
- **Database page reset to the Daemons tab on every reload** — the active tab
  (`Daemons` / `Backups` / `Terminal`) and the Terminal tab's selected engine
  (`mariadb` / `postgresql`) lived only in component state, so navigating away
  or refreshing the page always snapped back to Daemons. Both now persist via
  `localStorage` (`sabdopalon.database.tab`, `sabdopalon.database.engine`),
  mirroring the Terminal page's tab persistence, so the page returns to the
  view the user left it on. A stored engine whose package is no longer
  installed falls back to the first running/installed engine.

## [0.9.2] — 2026-08-25

### Fixed
- **Database terminal hung on "connecting" forever (mariadb/psql not found)** —
  the Database page's embedded terminal spawns the engine's CLI client
  (`mariadb`/`psql`) via a PTY so you can drop straight into a DB prompt. But
  `exec.Command("mariadb", …)` resolves the executable with
  `exec.LookPath` at construction time against the **server's own PATH**
  (`os.Environ()`), not against the child PATH `envFor` builds (which leads
  with `<bin>/<engine>/bin`). Since `mariadb`/`psql` live only in
  `bin/mariadb/bin` or `bin/postgresql/bin` — directories the server process
  never has on its PATH — the lookup failed before the child could start.
  The WebSocket handler returned `500 terminal: exec: "mariadb":
  executable file not found in $PATH` **before** accepting the upgrade, so
  the browser socket never left CONNECTING and the UI sat on "connecting"
  indefinitely (no prompt, no error, just an empty panel). The terminal
  backend now resolves the child executable against the Sabdopalon PATH
  (`lookPathInEnv`) and passes the absolute path to `exec.Command`, so the
  DB client is found in `<bin>/<engine>/bin` and the prompt appears.
  Regression tests cover `lookPathInEnv` (bin-dir resolution + not-found)
  and the `envFor` PATH ordering invariant.

## [0.9.1] — 2026-08-25

### Fixed
- **Desktop installers were stamped 0.8.3 in the v0.9.0 release** — the
  Tauri/Cargo/npm version pins (`tauri.conf.json`, `Cargo.toml`,
  `package.json`, `package-lock.json`) still read `0.8.3`, so the release
  produced `Sabdopalon_0.8.3_*.{AppImage,deb,dmg,exe}` assets under the
  v0.9.0 tag while the CLI bundles were correct. All four desktop version
  pins are bumped to `0.9.1`.

## [0.9.0] — 2026-08-25

### Added
- **Per-site detail page** — every site now has a dedicated detail page at
  `/sites/:name` with tabbed views: Overview (framework, PHP, database, URLs,
  running dev-tools), Config (inline `.sabdopalon.yml` editor), Logs
  (multi-source tailer: php/vite/artisan), Dev Tools (start/stop Vite, artisan,
  npm, composer), and an inline Terminal. Clicking a site name in the sites
  list navigates here; the row menu gets a "View details" entry.
- **Dev-tools supervisor (`internal/devtools`)** — a new package that manages
  per-site development tool processes (Vite, Artisan serve, npm run dev,
  npm run build, composer install/update). Mirrors the `internal/services`
  pattern but is scoped per site: each site gets its own Vite on its own port,
  and tools are killed when the site is stopped or Sabdopalon shuts down.
  Available tools are auto-detected from the project dir (vite.config,
  artisan, package.json, composer.json).
- **Framework-aware PHP router** — the proxy now detects the site's
  framework (Laravel, WordPress, CodeIgniter, Symfony) and writes the right
  `.sabdopalon-router.php` content. Laravel gets a dedicated router that sets
  `SCRIPT_NAME`/`PATH_INFO` correctly so the front controller and route
  matching work under `php -S` without `.htaccess`.
- **Vite reverse-proxy** — when a Vite dev server is started from the Dev Tools
  tab, the proxy intercepts Vite HMR/asset paths (`/@vite/`,
  `/node_modules/.vite/`) and reverses them to the running Vite dev server.
  This makes `name.localhost` serve Vite HMR directly — no need to run
  `composer run dev` in a separate terminal or visit `localhost:8000`. The
  Vite port is also injected into PHP env (`SABDOPALON_VITE_PORT/HOST`) so
  `vite.config.js` can wire HMR to the Sabdopalon-managed Vite server.
- **Site detail API** — `GET /api/sites/<name>` returns a full aggregate
  (framework, PHP binary, config, dev-tools status, available logs);
  `GET /api/sites/<name>/logs?log=<source>` tails a specific log;
  `POST /api/sites/<name>/devtools {tool, action}` starts/stops a dev-tool.
- **Inline database controls on the dashboard** — MariaDB and PostgreSQL now
  have dedicated cards directly on the dashboard (status badge, error hint,
  Start/Stop buttons) so the engines can be started/stopped without navigating
  to the Database page. They reuse the same `api.databaseControl` endpoint as
  the Database page; the Database page remains the place for port config,
  restart, and backups. This resolves the confusion where the dashboard's
  Start All / Stop All only touched optional services (MinIO, Meilisearch,
  …) while the DBMS engines — prominently shown in the Databases stat card —
  had no controls on the dashboard itself.

### Fixed
- **phpMyAdmin: "The mysqli extension is missing"** — the pinned static-php
  builds were too small: the unix "common" combo compiles pdo_mysql but not
  mysqli (phpMyAdmin requires mysqli), and the Windows spc-min build ships
  only 7 extensions. The default PHP ([php]/[php85]) now pins the "bulk"
  combo (~55 extensions: mysqli, pdo_mysql, gd, zip, intl, imagick, redis,
  opcache, …) on unix and the "spc-max" combo (mysqli, gd, zip, curl, …) on
  Windows (8.5.5, the newest spc-max asset). All five SHA-256 pins
  re-computed from full downloads; versioned extras (php81–php83) still use
  "common".
- **CLI release bundle shipped PHP without mysqli (Windows + Linux/macOS)**
  — the release workflow's CLI bundle step downloaded the "common" combo on
  unix and "spc-min" on Windows, while the desktop build and the package
  registry already used "bulk"/"spc-max". The CLI bundle was therefore the
  one distribution path where phpMyAdmin still failed with "mysqli extension
  is missing". The CLI bundle now downloads the same "bulk" (unix) /
  "spc-max" (Windows, 8.5.5) builds as the desktop app and registry, so all
  three distribution paths ship a PHP with mysqli.
- **Database page showed "Port aktif di: 0" while the daemon ran fine** —
  the setup wizard never wrote `mariadb_port` (only the legacy `port`
  field), the config API reported that raw 0, and the UI's `?? 3306`
  fallback does not catch a literal 0. The API now reports the *effective*
  port (config → legacy fallback → default), the wizard writes
  `mariadb_port = 3306` explicitly, and the UI treats 0 as unset.
- **Ghost MariaDB locked every launch out of its own port** — a daemon can
  outlive its sidecar (crash, SIGKILL, data dir deleted while running) and
  lose its pid file with it; from then on every start reported "port 3306
  is already in use by another process" against the app's own orphan.
  `checkPortOwner` now falls back to process-table identity: a mariadbd/
  postgres whose command line pins OUR data dir (`--datadir=…`/`-D …`) is
  adopted even without a pid file, and `StopAll` sweeps such ghosts so quit
  reclaims their ports.
- **Quit could SIGKILL the sidecar mid-cleanup** — the shell waited only
  10s for the graceful Go shutdown (sites → databases → services, each with
  its own budget) before SIGKILL, exactly how daemons get orphaned. The
  grace period is now 30s.
- **Setup-mode SIGTERM skipped cleanup** — the handler was a bare
  `os.Exit(0)`; it now stops site children first, mirroring the full
  server's shutdown path.
- **First launch stalled with no feedback** — the sidecar extracted the
  bundled core (100+ MB archive) BEFORE starting the dashboard, so the
  native window stayed hidden for up to a minute and the webview showed a
  stale "Connection refused" page. The dashboard now binds within ~1s of
  double-click; the bundled core is extracted during the wizard's install
  step instead, with progress in its log ("📦 memeriksa core bundle…").
- **Setup wizard froze at "Memulai…" (10%)** — the desktop setup watcher
  restarted the sidecar the moment config/engine.toml appeared, but the
  install job writes that config first and keeps working (phpMyAdmin
  deploy, sample site, completion marker). The restart killed the job
  mid-flight; the watcher now waits for the completion marker, and the
  wizard recovers from an orphaned job by navigating to the dashboard.
- **Install log panel collapsed to ~80px** — shadcn CardHeader ships
  items-start which survived cn() and shrank flex children to content
  width; items-stretch restores full-width logs.
- **AppImage vs CLI port conflict** — the Tauri single-instance plugin only
  knows about Tauri processes, so launching the CLI (`sabdopalon`) while the
  desktop app ran (or vice-versa) started a second instance that failed to
  bind 9900/8080/3306 and left the user staring at a generic error or
  half-started daemons. Both launch paths now acquire a shared advisory lock
  (flock on Unix, LockFileEx on Windows) on `<data>/.sabdopalon.lock` before
  binding any port; a second instance refuses to start with a clear message
  naming the holder PID.
- **OS shutdown/logout orphaned the sidecar** — only tray Quit called
  `sidecar::stop()`; an OS-initiated exit (logout, shutdown, SIGTERM to the
  Tauri process) left the Go sidecar and its database/service daemons
  running, holding ports so the next launch failed to bind. The desktop
  shell now hooks `RunEvent::Exit` so `sidecar::stop()` runs on every
  termination path.
- **Shutdown left the HTTPS listener dangling** — the SIGTERM handler called
  `srv.StopAll()` (PHP sites only) instead of `srv.Stop()`, so the HTTPS
  listener and its cert watcher were never closed; the next start could trip
  on the leftover socket. Both the full server and setup-mode handlers now
  call `srv.Stop()`, and the handler is installed before the dashboard
  starts so a signal during startup still cleans up.
- **Tray Restart raced the freed port** — `restart()` spawned the
  replacement sidecar immediately after `stop()`, but the OS could still
  hold the dashboard port in TIME_WAIT or a daemon child could lag a beat.
  The new instance would then fail to bind. `restart()` now polls the port
  until it clears (bounded) before starting the replacement.
- **Blank white window after Quit → relaunch (AppImage)** — `wait_ready`
  polled TCP connectability (layer 4) but the dashboard's HTTP handler
  (layer 7) was not ready yet; the reload then landed on a stale error
  page that WebKitGTK does not re-fetch, leaving the window permanently
  white while the server itself came up fine seconds later (verifiable in
  a browser). The readiness probe now sends a real HTTP HEAD request and
  waits for an HTTP response, and the reload is a `location.replace()`
  (which forces a fresh navigation even from an error page) instead of
  `location.reload()`.
- **Package install hung at "starting…" (web + desktop)** — the install
  handler piped the package manager's progress through an unbuffered
  `io.Pipe` and read it with `io.ReadAll` *after* `Download` returned.
  `io.Pipe` has no internal buffer, so the very first progress `Write`
  blocked waiting for a reader that would never run until the download
  finished — a classic deadlock. The install never completed and the UI
  sat on "Memasang … starting…" forever, with no success/failure
  feedback. Progress is now written straight into the job's output buffer
  via a mutex-guarded writer, so the UI poll sees lines live; the progress
  card now shows an explicit running/success/failure state (spinner → ✓/✗)
  with a dismissible result, and the log auto-scrolls.
- **PHP package cards contradicted the status header** — `/api/packages`
  reported `installed` based solely on the bundled copy in `bin/`, so when
  the host system supplied the PHP Sabdopalon was actively running (the
  header said "php 8.5.8"), every PHP card still showed "not installed" — a
  direct contradiction that left users unsure whether anything worked. The
  registry also defines both a generic `[php]` and a versioned `[php85]`
  pointing at the identical artifact, which rendered two near-duplicate
  cards ("PHP 8.5" and "Default PHP") for the same download. The packages
  API now exposes an `active` flag set when a PHP package's version matches
  the PHP Sabdopalon is currently using (bundled or system); the UI
  de-duplicates PHP cards by short version (keeping one) and shows a green
  "aktif" badge for the PHP in use, distinct from "installed" (bundled) and
  "not installed" (installable).
- **Windows built-in terminal opened an external console window instead of
  the embedded one** — `startProcess` in `conpty_windows.go` allocated a
  `ProcThreadAttributeList` for the pseudo console but never filled it with
  the `PSEUDOCONSOLE` attribute (the `attrs.Update(…)` call from the upstream
  go-pty reference was dropped during adaptation). `CreateProcess` therefore
  received `EXTENDED_STARTUPINFO_PRESENT` with an empty attribute list, did
  not know about our `HPSEUDOCONSOLE`, and fell back to allocating a fresh
  visible console window for the PowerShell child — so the desktop app's
  embedded terminal stayed dead while a separate PowerShell window popped
  up. The missing `attrs.Update(_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, &hpc,
  sizeof(hpc))` is restored (passing `&hpc` rather than casting the handle to
  a pointer to satisfy `go vet`'s unsafe.Pointer rules), which attaches the
  child to the pseudo console and routes its I/O through our pipes.

## [0.8.3] — 2026-08-25

### Added
- **Linux launcher integration (AppImage)** — the desktop shell now writes
  a user-level launcher + the camel icon into
  `~/.local/share/{applications,icons}` (idempotent, no root), so the app
  shows up in the GNOME/KDE dash with the right icon instead of a generic
  one. Exec points at the AppImage path ($APPIMAGE) when available. Naming
  matches Tauri's own bundle exactly (`sabdopalon-desktop` for the icon and
  StartupWMClass) — the WM_CLASS comes from the Cargo binary name, and a
  mismatched value makes the running window impossible to associate with
  the launcher.
- **Unified surface colors** — the sidebar token now matches the background
  (title-bar surface) in both themes, so title bar, sidebar and pages read
  as one continuous surface; the setup wizard follows it and adds explicit
  seams (border lines) between header, content and footer.

### Fixed
- **Quitting left databases and services running** — the desktop shell
  killed the sidecar with SIGKILL, so the Go shutdown path (which stops
  sites, databases and services) never ran and MariaDB/PostgreSQL kept
  living as orphans. `sidecar::stop()` now sends SIGTERM first and waits
  (bounded) before falling back to SIGKILL; on Windows taskkill /T takes
  the whole tree. Tray Quit and the setup-restart both go through this.
- **Title bar scrolled away and the sidebar sat under it** — the custom
  title bar was in the normal document flow (it scrolled off with long
  pages) and the fixed sidebar container (`data-slot="sidebar-container"`,
  not `…-sidebar`) was never offset, so it hid under the bar. The bar is
  now fixed at the top, the frame is padded by its height, and the
  container starts below it.
- **Desktop app trapped in setup mode after the wizard** — the setup-mode
  sidecar never exited: the dashboard reloaded into full chrome while the
  server was still the config-less instance (toast "database manager not
  available (setup mode)", Proxy: 0, DB Start buttons failing). The shell
  now watches for the wizard's config and restarts the sidecar in full
  mode; the wizard's reload waits until the real server (proxy bound)
  answers before navigating.
- **Desktop window controls were dead** — the dashboard is served from the
  remote origin http://localhost:9900, and Tauri capabilities only covered
  the local context, so every IPC call from the custom title bar
  (minimize/maximize/close/start-dragging) was silently denied. The
  capability now declares the dashboard's remote URLs.
- **Wizard fits the screen** — the setup wizard was redesigned to fit one
  viewport with no page scroll: compact horizontal header, tighter cards,
  and the CTA pinned to a bottom bar (log panel scrolls internally).

## [0.8.2] — 2026-08-25

### Added
- **New app icon everywhere** — the camel desktop icon now drives the full
  Tauri icon set (Windows .ico incl. Square logos, macOS .icns, Linux
  PNGs) across all platforms.
- **CLI on the built-in terminal's PATH (AppImage)** — the desktop shell now
  seeds a runnable copy of the `sabdopalon` CLI into `<data>/bin` (version-
  probed, refreshed on app update, atomic copy), so Linux users can drive
  the same install from the built-in terminal: `sabdopalon doctor`,
  `sabdopalon add …`, `sabdopalon sites`… A real copy rather than a symlink
  so it survives outside the read-only squashfs mount and can even take
  file capabilities (enable-ports).
- **Custom desktop title bar** — the Tauri shell now draws its own window
  bar (drag region, brand, minimize/maximize/close) matching the dashboard
  design system instead of the native OS chrome; macOS keeps the native
  traffic lights via the overlay title-bar style. The bar renders only
  inside the desktop app — the browser dashboard is untouched.
- **Persistent terminal sessions** — named sessions (`?session=<key>`) now
  survive disconnects: the Sites dock and Terminal page reattach to the same
  live shell after a route change or reload and replay the buffered output
  (ring buffer, 256 KB) instead of spawning a fresh shell. One permanent
  pump per session reads the PTY; clients are swappable sinks, so reattach
  can never steal bytes or interleave writers. Sessions are single-client
  (reattach kicks), LRU-capped at 12, and reaped after 30 minutes detached;
  `?fresh=1` still spawns a brand-new shell (Restart button).
- **Terminal page tabs persist** — the tab list (and each tab's session key)
  is saved to localStorage, so navigating away and back restores every shell
  with its scrollback instead of resetting to a single "Shell 1". Session
  keys are per-tab-lifetime nonces (a new tab can never reattach to a closed
  tab's shell), and closing a tab kills its server session immediately via
  the new `?kill=1` control on the terminal endpoint.

### Changed
- **Sites page: terminal dock starts closed** on every screen size — the
  header button opens it and the choice is remembered in localStorage
  (previously it forced itself open on desktops ≥1024 px).
- **Desktop installs land in a user-visible folder** — the Tauri shell now
  points the sidecar's install root at `<home>/Sabdopalon`
  (`/home/<user>/Sabdopalon` on Linux, `C:\Users\<user>\Sabdopalon` on
  Windows) instead of the hidden OS app-data dir
  (`~/.local/share/com.sabdopalon.app`). Existing installs in the legacy
  location keep running there — only fresh setups use the friendly root.

## [0.8.1] — 2026-08-25

### Fixed
- **CLI crashed on a fresh, config-less install** — `sabdopalon sites`,
  `doctor`, `vhost` and even `--help` dereferenced the nil config and
  segfaulted when run before the setup wizard completed (exactly what a
  user typing into the AppImage's built-in terminal hits first). All four
  now fall back to the default-shaped bare config.
- **Database daemon start: conflicts, double-starts and zombies** — the
  shared MariaDB/PostgreSQL `Start()` path never checked whether the port
  was taken or a daemon for the same data dir was already alive, so a second
  install (e.g. AppImage alongside CLI) or a plain double-click on "Start"
  spawned a doomed daemon that died 30 seconds later with a generic "did not
  start". Now: port held by a foreign process fails fast with a message
  naming the port; an alive daemon for this data dir (pidfile + binary
  match) is adopted instead of re-spawned; readiness additionally verifies
  the pidfile records OUR pid — closing Windows' false-ready hole where any
  listener on the TCP port counted as success. The monitor goroutine now
  Waits() every child from birth (readiness-timeout starts included) and
  Stop() waits for it, eliminating `[mariadbd] <defunct>` zombies; Windows
  Stop falls back to `taskkill /T /F` for orphaned trees. Failure messages
  embed the last log lines so `logs/mariadb.log` explains itself in the UI.

## [0.8.0] — 2026-08-24

### Changed
- **Toolchain & CI modernization** — GitHub Actions bumped to current majors:
  checkout v7, setup-node v7 (Node 24), setup-go v7, upload-artifact v7,
  download-artifact v8, action-gh-release v3. Kills the "Node.js 20 is
  deprecated" warnings that forced artifact actions onto Node 24.
- **Go 1.27** — `go.mod` language version raised from 1.23 to 1.27
  (latest stable); CI already builds on `stable`.
- **App version 0.8.0** across desktop package, Tauri config, Rust crate,
  sidecar, and CLI (`internal/app.Version`).
- **pkgmgr hardening** — extraction is staged and verified before promotion;
  an archive yielding zero files fails loudly instead of marking an empty
  tree as installed; URL overrides use the exact `platformKey()` vocabulary.
- **Setup wizard revamp** — full-screen single-page wizard (no sidebar/header)
  with live inventory: **"Termasuk dalam paket"** lists PHP/MariaDB/phpMyAdmin
  with real installed-state badges (not static text), **"Tools tambahan"** on
  the right shows only not-yet-installed tools as checkboxes (Redis hidden on
  Linux/macOS; MinIO version hidden when too long), **"Pengaturan lanjutan"**
  collapsed by default. Gate no longer leaks to the dashboard on refresh/
  restart mid-setup — a completion marker is written only on success; legacy
  installs remain bootstrapped via real data detection.

### Fixed
- **Cross-platform package installs** — audited every registry entry against
  real upstream assets: PHP/Mailpit Windows zips were extracted as tar.gz
  (hard failure), php82–85 `url_windows` contained an unsupported
  `{version_windows}` placeholder (guaranteed 404), php81 had no Windows URL
  at all, MinIO's Windows URL added an `.exe` suffix upstream never ships,
  Meilisearch macOS/Linux-arm asset names never matched, PostgreSQL on
  Apple Silicon hit a nonexistent `darwin-arm64` classifier, and Redis'
  flat Windows zip + `strip_root=true` produced an empty install that was
  still marked "installed". All fixed; Adminer no longer renamed to
  `adminer.php.exe` on Windows. Registry templates and checksum keys are now
  validated by a unit test (`TestRegistryTemplatesWellFormed`).
- **SHA-256 pins for every platform** — all artifacts now carry checksums
  for linux x86_64/aarch64, macOS Intel/Apple Silicon, and Windows x64
  (phpMyAdmin uses one platform-neutral pin). Only the floating
  `meilisearch = "latest"` stays trust-on-first-use by design.

## [0.7.5] — 2026-08-23

### Fixed
- **Desktop installers were never uploaded** — every release to date: builds
  use `--target <triple>` so bundles land under `target/<triple>/release/
  bundle`, but the upload glob read `target/release/bundle/**` (zero
  matches, silently "successful"). Globs now span all triples, and a
  fail-loud verification step asserts the expected installer exists before
  uploading.

## [0.7.4] — 2026-08-23

### Fixed
- **CI: linux desktop download step** — the core archive was written into
  the very tree tar was walking ("file changed as we read it"); staged via
  /tmp now.
- **React #310 crash** — `useNavigate` ran after DashboardPage's setup-mode
  early return; all page hooks audited to be unconditional.
- **Embedded terminal could not be typed into** (input wiring lost in a
  refactor) and Settings' database card confusion (moved wholesale to the
  Database page).

### Changed
- **Every database runs at once** — MariaDB AND PostgreSQL are independent
  daemons, each with its own card (enable switch, port, start/stop/restart,
  failure reason), both default ON; live toggle without restart. Sites and
  the embedded terminal receive connection info for BOTH engines.

## [0.7.3] — 2026-08-23

### Fixed
- **Desktop shell: "Could not connect to localhost"** — three stacked bugs:
  resource resolution read `<res>/core` while Tauri ships
  `<res>/resources/core` (bundled PHP invisible → auto-download tried the
  read-only AppImage mount → sidecar died before :9900 listened); second
  launches fought the first over ports (close hides to tray) —
  `tauri-plugin-single-instance` now focuses the running window; bin root is
  always a writable `<data>/bin` with bundled entries symlinked in.
- **CI: desktop linux AppImage, root cause this time** — linuxdeploy scans
  every ELF under AppDir usr/lib *including resources*, and any bundled
  binary linking a lib the runner lacks (libaio…) fails the build regardless
  of Galera removal. Linux now ships the core as ONE archive
  (`resources/core/core.tar.gz`, no ELFs to scan); the sidecar extracts it
  into the writable bin dir on first run and deletes the ~400 MB archive.

## [0.7.2] — 2026-08-23

### Terminal that just works + Sites page revamp

### Added
- **Sites page revamp** — app-frame layout: content column and terminal dock
  scroll internally and span exactly header→bottom (no dead gap under the
  fold); persistent right-hand **terminal dock** (collapsible, width
  persisted) with site-context dropdown, connection indicator, clear &
  restart-shell buttons; hybrid responsive site list (table on desktop,
  cards on mobile), site filter box, dismissible clean-URL banner.
- **Terminal tabs** — `/terminal` hosts multiple concurrent shell sessions;
  switching tabs keeps background sessions alive.
- **Terminal UX** — bundled JetBrains Mono font, clipboard integration
  (`Ctrl+Shift+C/V`, OSC52), auto-reconnect with status indicator.

### Fixed
- **`mariadb`/`psql` now connect from the embedded terminal without flags**
  — sessions get DB client env (`MYSQL_UNIX_PORT`,
  `MARIADB_UNIX_PORT`, `MYSQL_TCP_PORT`, `MYSQL_HOST`, PG* equivalents)
  pointing at Sabdopalon's own daemon; previously clients died with
  `ERROR 2002 … /tmp/mysql.sock`.
- **`TERM environment variable not set`** (`clear`, vim, htop…) — sessions
  default to `TERM=xterm-256color` + `COLORTERM=truecolor`; the desktop
  sidecar has no TERM of its own.
- **Windows terminal is a real terminal** — pipe fallback replaced by
  ConPTY (`internal/terminal/conpty_windows.go`): colors, resize and
  interactive programs work; resize frames now go out instantly via xterm's
  `onResize` instead of a 2 s poll.
- **CI: desktop linux job** — AppImage bundling failed twice: linuxdeploy
  needs FUSE missing on ubuntu-24.04 runners (install `libfuse2`(t64) +
  `APPIMAGE_EXTRACT_AND_RUN=1`) and MariaDB's Galera/engine-plugin ELFs
  link OpenSSL 1.0 that no runner ships (dropped at download time; never
  loaded by local dev).

## [0.7.1] — 2026-08-23

### Fixed
- **Windows: no console flashes** — child processes (php, MariaDB,
  mariadb-install-db, certutil, tar, composer, php version probes) are now
  spawned with `CREATE_NO_WINDOW` + `HideWindow`, so the desktop app never
  pops up a console window; sidecar stays `-H windowsgui`. New shared helper
  `internal/winproc`. README now presents the NSIS desktop installer as the
  primary terminal-free Windows install path.
- **CI: AppImage bundling on ubuntu-latest (24.04)** — the linuxdeploy
  AppImage needs FUSE which GH runners no longer preinstall; install
  `libfuse2`(t64) and set `APPIMAGE_EXTRACT_AND_RUN=1` so bundling works
  headless.

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
