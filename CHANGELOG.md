# Changelog

All notable changes to Sabdopalon are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/); versioning is semver-ish.

## [Unreleased]

## [0.11.21] - 2026-10-16

### Fixed
- **A per-site PHP version now sticks.** Picking one in a site's Config tab wrote the
  value into `.sabdopalon.yml` quoted and escaped, but the reader only stripped the
  quotes, so a Windows binary path came back as `C:\\php84\\php.exe` instead of
  `C:\php84\php.exe`. It then matched none of the PHP choices the dashboard offers, and
  the site looked like it had gone back to the default PHP. Writing and reading now
  agree; files already on disk from the old build are read correctly; and a value the
  parser cannot round-trip is refused before it reaches the disk, where it used to be
  written first and reported afterwards (leaving the damaged file in place).
- **The Config tab says what is actually stored.** A saved value that is not in the
  list - a version that is not downloaded, a path that moved, a hand-edited file - is
  now offered as its own "as configured" entry. The select used to fall back to its
  placeholder, which read as "it reverted to Default".
- **Saving reports a PHP that cannot be resolved.** A version that is neither bundled
  nor on PATH saves fine but makes the site refuse to start, so the reply carries that
  warning instead of a bare success.
- **`node:` and `database:` overrides in `.sabdopalon.yml` survive a save** from the
  Config tab, which used to replace the whole file. A file this build cannot parse is
  not overwritten at all: it is left alone with an explanation.
- **The site page no longer breaks when there are no dev-tools or log files.** The API
  sends empty lists rather than `null`, and the page tolerates a `null` from an older
  sidecar. A JSON `null` there reached `.filter()` and took the whole page down.
- The PHP card on a site's Overview shows the binary name rather than the whole Windows
  path, together with the version that binary actually reports.
- Stray Windows-1252 bytes left in this file by earlier edits are repaired, so it is
  valid UTF-8 again.

### Fixed (updates)
- **The in-place update path can be offered again.** The updater reads the presence of an
  asset whose name says *sidecar* as "only the Go program changed, replace it in about
  ten seconds". The workflow published that binary under its Tauri external-binary name
  (`sabdopalon-x86_64-pc-windows-msvc.exe`), which never says it, so every release was
  offered as a full installer update and the fast path could not trigger at all. It is
  now published as `sabdopalon-sidecar-x86_64-pc-windows-msvc.exe`.
- **A version bump is no longer mistaken for a shell change.** Every release carries the
  new number in `Cargo.toml`, `Cargo.lock` and `tauri.conf.json`; counting those files as
  "the desktop shell changed" is what made every release installer-only. A file now only
  counts when some changed line is not a version line — `src/`, `capabilities/`, `icons/`
  and `resources/` still count unconditionally.
- **The installer is chosen by name, not by upload order.** The updater used to take the
  first `.exe` asset, which was correct only because CI happened to upload the installer
  first; a release listing a bare sidecar ahead of it would have run an 11 MB program as
  a 59 MB installer. A name containing `setup` wins, a name containing `sidecar` is never
  an installer, and anything else is a last resort.

## [0.11.20] - 2026-10-11


### Added
- **An About page** that answers what this installation is, what it does to your machine,
  and what it talks to: identity and version, the components it is made of, the paths it
  keeps (each copyable and openable), the credits, a diagnostics block made to be pasted
  into a bug report, and the complete list of hosts it may contact.

  Built by danyakmallun - https://github.com/danyakmallun9999/sabdopalon

- **Updating from inside the app.** About checks for a new release, says which of the two
  paths it will take - about ten seconds in place, or the installer with roughly a minute
  of downtime - downloads it, verifies the checksum against what the release published,
  applies it, and keeps the previous sidecar so a failed start can be rolled back.
  Checking and applying stay separate steps, and it never updates itself.

- **A beta channel** that can see pre-releases, remembered per browser, with the highest
  version winning rather than the first entry on the list.

- **Signed releases** when a signing key is present: the installer's .sig is published
  beside it. Without the key a release is unsigned and still complete, so a missing secret
  can never break a release.

### Fixed
- The PHP router no longer lives in your projects: one shared script in the data root,
  nothing at all for plain PHP, and files older versions wrote inside site folders are
  removed on the next start.
- Linked projects are served: a project whose PHP lives only in a subfolder was treated as
  static and shown as a directory listing.
- `sabdopalon link` reports the port actually in use rather than the configured one.
- PostgreSQL gains its client tools (psql, pg_dump) as a self-contained package, which also
  gives PostgreSQL backups the pg_dump they were missing.

### Notes
- The About page and the in-app updater exist from this version on; earlier versions have to
  be updated manually once.
- The updater is Windows-only, like this release workflow.

## [0.11.19] - 2026-10-11

### Fixed
- **The PostgreSQL terminal no longer hangs.** psql was launched without the
  directory holding libpq on its PATH, and Windows reports a missing DLL as a modal
  dialog that leaves the process waiting forever — a session that never showed a
  prompt. The client tools are now self-contained: the archive's DLLs are installed
  beside the executables (14 DLLs, 42.8 MB in total), so nothing depends on how the
  child's PATH was assembled. Verified with the server's directory removed from PATH
  entirely: `psql (PostgreSQL) 17.10`, connecting as sabdopalon.

  Reinstalling the package is enough to pick this up; the registry entry itself
  decides which files are kept.

## [0.11.18] - 2026-10-11

### Added
- **The PHP router no longer lives in your projects.** Every PHP site used to get
  its own `.sabdopalon-router.php`, written inside the site folder - and for a
  linked project that folder is your own repository, so a tool left a foreign file
  in your code, even for projects that need no router at all. There is now one
  shared script in the data root (`<root>/routes/router.php`), plain PHP sites get
  none, and a router this program previously wrote inside a site folder is removed
  on the next start. A router that is not ours is never touched.

- **PostgreSQL client tools** - `psql` and `pg_dump` - installable from the
  Packages page. They were never bundled: the PostgreSQL Sabdopalon ships is
  server-only (zonky's embedded build), which left the Database tab's PostgreSQL
  terminal with nothing to launch and PostgreSQL backups with no `pg_dump`. The
  executables come from EnterpriseDB, the same 17.10 version as the server, and a
  new registry key (`files`) keeps nine executables (2.3 MB) out of the 318 MB
  archive, because every DLL they link against is already installed beside them.

  The Database tab no longer hides the PostgreSQL terminal, and `psql` connects
  with no flags: the terminal already injects PGHOST/PGPORT/PGUSER=sabdopalon/
  PGDATABASE.

### Fixed
- **Linked projects are served.** `IsStaticOnly` did not follow a junction, so a
  project linked from outside the data root whose PHP lives only in a subfolder
  (`public/index.php`) was served as a directory listing instead of through PHP.

- **`sabdopalon link` reports the port actually in use.** It printed `:8080` while
  the proxy was answering on port 80, because the configured port is only a
  preference. Port 80 is no longer named at all.

## [0.11.17] - 2026-10-11

### Added
- **Choosing the database and the testing framework when creating a site.** Both
  are what `laravel new` asks about, available in the New site dialog and on the
  command line:

      sabdopalon new <template> <name> [--db=sqlite|mariadb] [--pest|--phpunit]

  - **SQLite** stays the default, as it is for `laravel new`.
  - **MariaDB** is where Sabdopalon can do what the Laravel installer cannot: it
    creates the database, writes DB_CONNECTION/DB_HOST/DB_PORT/DB_DATABASE/
    DB_USERNAME/DB_PASSWORD into both .env and .env.example, and then migrates.
    Nothing is asked of the user, because Sabdopalon already knows the port its
    server bound and the credentials. Verified on a real project: the schema was
    created and nine tables appeared in it (cache, cache_locks, failed_jobs,
    job_batches, jobs, migrations, password_reset_tokens, sessions, users).
  - **Pest** is the default testing framework, exactly as `laravel new` treats it,
    installed with the installer's own command sequence (remove phpunit, require
    pest + pest-plugin-laravel, update, `pest --init`). Verified by running the
    suite in the created project: two tests passed. `--phpunit` leaves the
    skeleton's testing setup alone.

  The database and testing choices are offered for Laravel templates only, and
  are skipped with a note for a project that has no artisan to run.

### Fixed
- **Scaffolding resolves its tools itself instead of trusting PATH.** Three
  separate failures came out of verifying this feature on Windows, all of the same
  kind: a GUI app cannot assume the environment it was launched with.
  - `composer create-project` shells out to `php`, so bin/ is prepended to the
    child's PATH ("'php' is not recognized as an internal or external command").
  - PHP fell back to C:\WINDOWS as its temp directory when TMP/TEMP were absent
    and Composer refused to run there; missing or empty temp variables are now
    filled from a folder Sabdopalon owns.
  - `exec.Command` searches the CURRENT process's PATH, not cmd.Env, so
    `php artisan migrate` failed with `exec: "php": executable file not found in
    %PATH%` even with the child's PATH fixed. Every tool is now resolved
    explicitly, with bin/ first — which also means a Composer installed through
    the Packages page takes precedence over an older system one.

## [0.11.16] - 2026-10-11

### Added
- **Laravel starter kits, the same choices `laravel new` offers.** Creating a site
  now offers the four first-party kits - React, Vue, Svelte and Livewire - each
  with and without authentication scaffolding (laravel/react-starter-kit,
  laravel/blank-react-starter-kit, ...), installed exactly the way the Laravel
  installer does it: `composer create-project <kit> <dir> --stability=dev`.

  A kit performs its own setup through composer.json's
  `extra.laravel.installer["post-create-project"]` hook, so nothing imitates what
  a kit does internally: verified end to end, a fresh React kit generated its key,
  migrated SQLite, installed npm dependencies and built its assets by itself.

  One thing is Sabdopalon's own: APP_URL is written as `http://<name>.localhost`
  in both .env and .env.example. `laravel new` writes `http://localhost:8000`,
  which names a port nothing listens on behind the proxy - the difference between
  a page that loads its assets and a blank one.

### Fixed
- **Scaffolding no longer depends on how Sabdopalon was launched.** Both failures
  below were found by verifying starter-kit creation on a real machine, and they
  are the same mistake: trusting an inherited environment.

  - `composer create-project` shells out to `php`, and the child inherited a PATH
    without the install's bin/, so the download died with
    "'php' is not recognized as an internal or external command". bin/ is now
    prepended for every scaffolding command.
  - PHP fell back to C:\WINDOWS as its temp directory when TMP/TEMP were absent,
    and Composer refused to run there: "PHP temp directory (C:\WINDOWS) does not
    exist or is not writable to Composer". Missing or empty temp variables are now
    filled in from a folder Sabdopalon owns (`<root>/tmp`), falling back to the OS
    temp - deliberately not os.TempDir() alone, which returns empty in exactly the
    stripped environment that produced this failure.

## [0.11.15] - 2026-10-11

### Changed
- **Fresh installs on Windows now live in `C:\sabdopalon`.** The data root was
  always `~/sabdopalon`, which is one level deeper than the convention this is
  modelled on (`C:\laragon`, `C:\xampp`) and spends ~33 characters of the
  260-character path limit a process gets before the project name is even
  appended — the limit `node_modules` trees run into. A new install therefore
  creates `C:\sabdopalon` when that volume root accepts a new folder and a new
  file, and falls back to `~/sabdopalon` when it does not (a hardened machine can
  withhold even the default "create folders" grant), so the app never fails to
  start over a path it cannot write.

  Nothing moves by itself. An existing install in `~/sabdopalon`,
  `~/Sabdopalon` or the old OS app-data folder is still used as-is, and
  `SABDOPALON_HOME` still overrides everything, so upgrading never strands a
  user's sites. To move an existing install, quit Sabdopalon and move the folder
  to `C:\sabdopalon` — it is picked up there on the next start, with no setting
  to change.

## [0.11.14] - 2026-10-11

### Added
- **The data root can live outside the home directory.** Set `SABDOPALON_HOME`
  (for example `C:\sabdopalon`) and the desktop shell, the sidecar and every CLI
  command use it: sites, bin, config, logs, data, certs, packages and backups all
  follow. This matters most on Windows, where a deep
  `<home>\sabdopalon\sites\<project>` path eats into the 260-character limit that
  `npm install` runs into. Quotes and stray whitespace around the value are
  trimmed, because `setx "SABDOPALON_HOME=C:\sabdopalon"` is how it gets set in
  practice.

  Set it before the app starts, and note that a Windows GUI inherits its
  environment from the Explorer session that launched it - so sign out and back in
  (or restart Explorer) once after setting it. Existing data is moved by hand;
  nothing is migrated automatically, and the shell keeps finding the old
  `~/sabdopalon` when no override is given.

## [0.11.13] - 2026-10-11

### Fixed
- **A stopped dev server no longer leaves the site blank.** `composer run dev` (or
  `npm run dev`) writes `public/hot`, and Ctrl+C leaves it behind. Laravel then
  points every asset at the address inside it, so the page returns its HTML while
  the browser gets no JavaScript: on a fresh Laravel 13 project all assets pointed
  at `http://[::1]:5173` with nothing listening, and the Inertia root stayed
  empty. Sabdopalon now removes a `public/hot` whose server is no longer
  listening - when it stops a dev-tool itself, when a site stops, and on a slow
  background sweep so a dev server it never started heals too. A hot file naming
  a live server is left untouched, because that is a user at work.

## [0.11.12] — 2026-10-10

### Fixed
- **`yac` is switched off explicitly, so it can never print a startup fatal error
  again.** The bundled PHP is a static build that includes `yac`, an optional
  shared-memory cache whose allocator is fragile on Windows. A fresh Laravel 13
  project running `composer run dev` printed this once per server start:

  ```
  Fatal error: Shared memory allocator startup failed at 'fopen': No such file or directory
  Fatal error: Unable to start yac module
  ```

  It reads as a broken PHP, and tooling that inspects a child's output treats it
  as one. The bundled binary could not be made to reproduce it here — `php -v`,
  `-m`, `-r` and `-S` all start clean, with or without the flag — so the failure
  depends on the environment a child is given, which is exactly why it is now
  turned off instead of left to chance: passed as `-d yac.enable=0
  -d yac.enable_cli=0` both to every site worker Sabdopalon starts and through the
  `php` shim on PATH. Nothing is lost — every request is a fresh process, so there
  is no long-lived process for a shared-memory cache to serve — and a later `-d`
  on the command line still wins for anyone who wants it back.

## [0.11.11] — 2026-10-10

### Fixed
- **A dead PHP worker no longer swallows every Nth request.** Each site runs
  several `php -S` children behind the proxy, and round-robin handed requests to
  them without asking whether they were still alive. A worker that died — memory
  pressure, a fatal error, or a parent killed without cleanup — took one request
  in `len(workers)` with it, which the browser shows as a blank page until a
  refresh happens to land on a live one. Worker selection now skips ports that are
  not accepting connections; the round-robin is otherwise unchanged, so the
  concurrency several workers exist for is not reduced.

- **Workers left behind by a killed Sabdopalon are cleaned up on startup.** A
  clean exit stops every worker, but a killed process (an installer replacing the
  app does exactly that, and the desktop shell hides to the tray rather than
  quitting) leaves them running: invisible in the app, still holding their ports,
  still holding their memory. Measured on one machine before the fix: **32 php
  processes and 1785 MB**, of which 24 workers had a dead parent. The reap is
  deliberately narrow — the command line must contain `-S 127.0.0.1:`, must name
  a directory under your sites root, and the parent process must be gone — so a
  second running instance, an `artisan queue:listen`, or a server you started
  yourself is never touched. Verified: it stopped those 24 and freed 1314 MB,
  leaving the running instance's own workers alone.

### Changed
- **Laravel's dev tool on Windows no longer runs `composer run dev`.** That script
  ends with `php artisan pail`, pail needs the POSIX-only `pcntl` extension, and
  the whole `concurrently` tree dies with it — the queue worker and Vite it had
  just started are killed a second later, which is what made `composer run dev`
  fail with "The [pcntl] extension is required to run Pail." The other thing that
  script starts is redundant here anyway: Sabdopalon serves the site through its
  own proxy, so `artisan serve` only opened a second server nothing outside the
  machine could reach. On Windows the tool is now "Laravel Dev (queue + Vite)" and
  starts exactly the two things that need supervising. macOS and Linux keep the
  project's own script, where pail works.

## [0.11.10] — 2026-10-10

### Fixed
- **The command line ignored `config/php.ini`.** Sites receive that file through
  PHPRC; the `php` shim in `bin/` passed nothing at all, so `php -i` reported
  `Loaded Configuration File => (none)` while the site happily read the file. A
  setting — or an `extension=` line — added from the dashboard therefore looked
  like it worked for the site and was ignored in the terminal, and every
  deprecation notice from every tool running on PHP was printed raw. The shim
  sets PHPRC now, guarded so a missing ini still leaves PHP on its defaults: one
  PHP install, one php.ini, whichever way you reach it.

- **`sabdopalon doctor` never mentioned Composer.** It reports the tools whose
  shim loses the PATH to another installation, and it skipped Composer, Node and
  npm as "system tools" — true until Sabdopalon installs them, misleading
  afterwards. A machine-wide Composer 2.8.9 from Composer-Setup sits in the
  MACHINE PATH ahead of everything per-user, wins over the managed 2.10.2, and
  prints deprecation warnings of its own on a modern PHP, which reads as "the PHP
  Sabdopalon ships is broken". The check was also blind to `.bat` launchers, so
  the managed `composer.bat` was invisible to it; both are fixed, and doctor now
  names the Composer it manages plus the one command that updates the winner.

## [0.11.9] — 2026-10-10

### Fixed
- **A publicly shared Laravel site came out blank.** The proxy rewrote every
  request's Host to the site's canonical `*.localhost` name — right for local
  traffic, wrong for a tunnel: the visitor's browser was told to fetch
  `http://point-of-sales.localhost/build/assets/app-*.js`, a host that exists on
  nobody else's machine. The HTML arrived, the JavaScript never did, and a
  client-rendered page stayed empty. Requests arriving on a tunnel hostname now
  keep the host the visitor actually used; local traffic is untouched.

  This also corrects a claim that shipped in 0.11.5: "the origin sees the real
  public Host" was never true. The first end-to-end test used a plain PHP page,
  which does not care what Host it was asked for, so the rewrite went unnoticed.

- **The request also still looked like plain HTTP to the framework.** Cloudflare
  terminates TLS, so PHP generated `http://` asset URLs inside an `https://` page
  and the browser blocked them as mixed content — a blank page again. The
  generated router now marks the request secure when the proxy says it arrived
  over HTTPS. It has to be the router: the PHP built-in server never sets
  `$_SERVER['HTTPS']` for a plain request.

- **Existing sites now actually receive router fixes.** `ensureRouters`
  deliberately never overwrote a router that was already there, so a fix like the
  one above would only ever have reached sites created after the upgrade. The
  generated routers carry a version marker, and a router *we* wrote that lacks it
  (or has an older one) is refreshed; a router a project ships itself is still
  left alone.

## [0.11.8] — 2026-10-10

### Fixed
- **The dashboard is canonical at `/dashboard` now.** The desktop shell navigates
  the window to the bare origin, so the address bar sat on `/` while the app was
  showing the dashboard — and the URL is what anyone quotes when something looks
  wrong. `/` now redirects to `/dashboard`.
- **A path that is not a route no longer renders an empty page.** Anything else —
  a stale bookmark, a mistyped URL, a link from an older build — used to match no
  route at all, which rendered the sidebar and header around a blank content area
  and read as "the dashboard is broken". There is a catch-all redirect to the
  dashboard now, so a stray URL lands somewhere useful.

  The links on the Go-served start page (the one shown for a host with no site)
  and the desktop shell's own navigation point at `/dashboard` too, so all three
  entry points agree on the canonical URL.

## [0.11.7] — 2026-10-10

### Added
- **cloudflared can be installed from the dashboard.** Sharing a site used to
  answer with a `winget install Cloudflare.cloudflared` command and a terminal —
  workable for you, hopeless for anyone else. `cloudflared` is now a package like
  PHP or Mailpit: it appears on the Packages page, and the Public access card
  offers an **Install** button whenever it is missing. It installs to
  `bin/cloudflared/`, the copy that takes precedence over anything on PATH, so
  the version this app manages is the version it runs.
  Pinned to 2026.10.0 per platform with GitHub's own release digests — each one
  re-verified by downloading the artifact and hashing it locally before being
  written into the registry — and the install verifies the checksum before it is
  used. The installer does not bundle it: the Windows download is 53 MB, and only
  people who actually share a site need it. On macOS and Linux the same package
  installs through `sabdopalon add cloudflared`.

### Fixed
- **The managed copy was ignored until restart.** The binary lookup cached the
  first path it resolved, so installing cloudflared from the dashboard left the
  app pointing at the system copy (or at nothing at all) until Sabdopalon was
  restarted — the install appeared to do nothing. A cached path that is not the
  managed one is no longer trusted, and two tests pin both the precedence
  (managed beats PATH) and the caching rule.
- **`type` can now be overridden per platform.** The registry could only say "the
  Windows artifact is packaged differently" (`type_windows`). cloudflared needs
  the general case: a bare executable on Windows/Linux and a `.tgz` on macOS, so
  `type_<os>_<arch>` exists now — and the registry guard test validates those
  keys against the same platform vocabulary as the checksum and URL overrides,
  which is what caught the first draft of the entry.

## [0.11.6] — 2026-10-10

### Fixed
- **Sharing a site opened an external console window that had to stay open.** The
  desktop sidecar has no console of its own (`-H windowsgui`), so Windows handed
  the console-subsystem `cloudflared` a brand-new window — and that window *is*
  cloudflared's console, so closing it killed the tunnel. cloudflared is now
  spawned console-free, the way every other child process already was
  (`HideWindow` + `CREATE_NO_WINDOW` through the `winproc.Quiet` helper), and a
  Windows-only test pins the flag so the regression cannot come back silently.
  The bug was invisible to a normal local `go build` (a console-linked sidecar
  lets the child inherit its console), which is why it slipped past the first pass.
- **The URLs in the Sites table did nothing when clicked.** The URL cell stops
  propagation so the row does not navigate, and React's synthetic
  `stopPropagation()` also calls the native one — so the click died before either
  the app's external-link handler or Tauri's own opener could see it. The handler
  now listens in the **capture** phase, which runs before any bubble-phase
  `stopPropagation`, so the link opens in the browser regardless; it calls
  `preventDefault()` first, so the opener plugin stands down and nothing opens
  twice.

## [0.11.5] — 2026-10-10

### Added
- **Share a site publicly, through a Cloudflare Quick Tunnel.** "Share publicly…"
  in the Sites menu (and a Public access card on the site page) puts a site on a
  temporary `https://<random>.trycloudflare.com` URL, optionally restricted with
  `--allowed-mail` so only named email addresses can open it. The same thing from
  the CLI: `sabdopalon share <site>`, `sabdopalon share list`,
  `sabdopalon share stop <site>`.

  How it is wired is why this is more than a spawned process:
  - The tunnel points at **Sabdopalon's proxy**, not at PHP, so a shared site keeps
    its document root, its per-site env and its PHP version.
  - The random public hostname is registered with the proxy **at runtime**, so the
    origin sees the real public Host header. Forcing the internal hostname through
    cloudflared's `httpHostHeader` would have made the app emit
    `http://myapp.localhost/...` links — dead for anyone outside this machine.
  - Sharing a stopped site starts it first; a tunnel to a stopped site only
    answers 503.
  - The URL is taken from cloudflared's own log by matching the *hostname*, not the
    sentence around it, so a reworded banner cannot break it. Failures retry where a
    different flag would actually help: an unknown `--no-autoupdate` is dropped, and
    an unreachable edge is retried over http/2 (QUIC on UDP 7844 is blocked on
    plenty of networks). A tunnel that keeps dropping is restarted three times and
    then reports it.
  - cloudflared is detected, never installed behind your back: PATH, winget's Links
    shim and the winget package directory, plus `<root>/bin/cloudflared` for a copy
    you manage yourself. When it is missing, the message quotes the command that
    fixes it on your platform (`winget install Cloudflare.cloudflared`,
    `brew install cloudflared`, or the release URL on Linux).
  - Nothing shares itself. A tunnel is never started because an earlier run had
    one: publishing a site is an explicit act, and a quick tunnel gets a new
    hostname every time, so an automatic restart would only produce a fresh public
    URL nobody asked for. Tunnels die with Sabdopalon, and `Stop`/shutdown removes
    the hostname from the proxy so a stale URL cannot resolve to a site again.

  Limits are shown in the UI instead of hidden: quick tunnels are for testing (no
  uptime guarantee, 200 concurrent requests and then 429, and **no SSE**), the
  hostname changes on every share, and anyone with the link can reach the site —
  turn `APP_DEBUG` off first. The dashboard itself is deliberately not shareable.

### Changed
- The sidebar's brand block is gone. The desktop title bar already draws the logo
  and the name two rows above it, so one screen showed both twice. Version and
  uptime moved to the app bar beside the DB/PHP badges.

### Fixed
- Setup announced success twice, in the same words: "Sabdopalon Is Ready! / All
  core components and selected tools were configured successfully." in the status
  card, and "Everything is installed and configured. / Your local server is ready
  for app development." in a panel below the log with a "Go to Dashboard" button.
  The panel is gone and the card now reads "Setup complete / Opening the
  dashboard…", which is what is actually happening while the sidecar restarts.
- The handover after setup replaces the location with `/dashboard` instead of
  reloading whatever URL the wizard happened to be on, so a finished setup always
  lands on the dashboard.

## [0.11.4] — 2026-10-10

A design-system pass, because the dashboard had grown a status colour per page,
five different corner radii among its own form controls, and three ways to name
an icon-only button. Two of the entries below are plain bugs that the pass
turned up.

### Fixed
- **Links in the desktop app did nothing.** Every site URL looked like dead text.
  The Tauri opener plugin does intercept those clicks — it even calls
  `preventDefault()` — and then invokes `plugin:opener|open_url`, which
  `capabilities/default.json` did not grant, so the invoke was rejected after the
  default had already been prevented. The permission is granted now, and the
  click interceptor was rewritten: it tested `window.__TAURI__.shell.open`, which
  does not exist in Tauri v2 (the global carries the CORE api; plugin apis arrive
  with their own injected script), so that check had quietly matched nothing.
- **A second Sabdopalon could not start at all.** The low-port preference asked
  `canBind(80)` — a *wildcard* bind — while the listeners that actually serve bind
  127.0.0.1 and ::1 one address at a time. Where 127.0.0.1:80 was already taken the
  probe passed, the real bind failed, and that error shut the whole app down
  instead of falling back to the configured port, which is what the code promises.
  The port is now chosen by binding it (`netutil.ServeListeners`).
- **The Sites table's right-hand side did not line up.** The Actions header was
  48px wide while its cell holds a button plus a menu (~126px), and the PHP and
  Status columns rendered a Badge in one case and a plain `<span>` in the other,
  giving them different font sizes and heights. Columns have explicit widths now,
  every state is a Badge, and the table's right edge matches the toolbar above it
  (the content column carried `lg:pr-4` that the toolbar did not).
- **Corner radius had three sources of truth:** `--radius` was 0, the code chips
  used a hard-coded 4px, and the sidebar set a self-referential
  `[--radius:var(--radius-xl)]` that resolved to nothing. One token now, with a
  linear scale, and the form controls that disagreed about their own corners
  (input 10px, select 14px, textarea 12px, checkbox 6px) share it.

### Changed
- **One status vocabulary.** `--success`/`--warning` with `StatusDot`/`ToneBadge`
  replace forty hard-coded palette classes: the same state used to be
  emerald-500 on one page and emerald-400 behind a `dark:` variant on another, at
  three different dot sizes. Two meanings came out clearer — a chip saying
  "default"/"installed"/"active" no longer borrows the primary *button* colour,
  and an alternative PHP version is muted rather than amber (a warning).
- **Icon-only buttons always have a label and a tooltip** (`IconButton`), instead
  of a raw `title=` on some and nothing at all on others.
- **Validation belongs to the field.** The wizard answered an invalid port with a
  toast that floats far from the input and expires on its own; errors are inline
  now, with `aria-invalid` and `aria-describedby`.
- **One empty state, one spinner**, instead of four shapes of "nothing here" and
  a mix of plain "Loading…" text and skeletons.
- **Layout by route, not by pathname string.** Which pages scroll internally was
  decided by a hard-coded array of route names in `App.tsx`; it is a layout route
  now, and the frame height is defined once (`--app-h`). The wizard's loading
  state is a skeleton instead of a blank window.
- **Fewer words.** Sentences that only restated the heading, the label or the
  table beside them are gone from every page; text that carries a constraint, a
  consequence or a format (the LAN exposure warning, backup retention, php.ini
  scope, alias/env formats, "needs composer") stayed.
- Five unused shadcn primitives deleted (avatar, breadcrumb, drawer, toggle,
  toggle-group); arbitrary 10/11px type sizes replaced with the scale; the sidebar
  groups its pages by what you came to do (Manage / Tools / System); and the site
  name in the Sites table is a real link, so the row is reachable by keyboard.

## [0.11.3] — 2026-10-10

Both fixes come from a fresh Windows laptop with PostgreSQL selected, and both
are about the same kind of silence: work happening with nothing on screen to say
so, and a failure message that names the wrong file.

### Fixed
- **The wizard looked frozen at 95% while it was still working.** Its log ended
  at "postgresql installed" because the step after the downloads — deploying
  phpMyAdmin into `sites/` — copied thousands of small files without a word, and
  a fresh machine's antivirus inspects every one of them. The step now announces
  itself and reports the running file count, and the same copy at startup (a
  bundled phpMyAdmin that is not deployed yet) moved behind the dashboard bind,
  together with the databases: nothing depends on it being finished, and it was
  another multi-minute wait in front of a window that could not explain itself.
- **"mariadb did not start: no output captured (see logs/mariadb.log)"** pointed
  at the wrong file. MariaDB and MySQL write their startup errors to
  `<datadir>/<host>.err`, not to the console, so the captured log is legitimately
  empty in exactly the case a user needs it most — the real error was sitting
  unread in the data directory. The failure now reports both logs, the engine's
  own error file and its last lines, whether the daemon is still alive or already
  exited (with its status), and whether this was a first start.
- **A first start gets 3 minutes instead of 30s.** It has to create the
  InnoDB/aria system tables and write them through, and antivirus inspects all of
  it. Killing the daemon at 30s was both wrong and destructive: the
  half-initialised data directory it left behind then failed every later start.
- **A half-initialised data directory is rebuilt instead of failing forever.**
  When a first initialization is interrupted — the app closed while it ran, which
  is what a stuck setup leads to — the directory is non-empty but has no system
  schema, and the old "has files, so skip init" rule left the database
  permanently unable to start. Rebuilding requires the *absence* of that schema
  (`mysql/`, `base/`) and of our own init marker, so a real database is never
  touched.

## [0.11.2] — 2026-10-10

Everything here was found by using 0.11.1 on a real Windows machine rather than
by reading the code, and two of the numbers below come from measurements taken
on that machine.

### Fixed
- **The first launch after setup could not answer the UI for minutes.** The
  dashboard bound only after every enabled database and service had started, and
  on a fresh install that means `initdb`/`mariadb-install-db` initialising empty
  data directories, each engine's 30s TCP probe plus 10s of pidfile waiting, and
  every optional service's own readiness probe — all in series. The wizard polls
  that same server, and its progress bar is capped at 95% by design, so a
  finished install looked frozen at 95% with no "enter dashboard" button
  (reported on a fresh Windows laptop with PostgreSQL selected; the next launch
  was instant because the data directories existed by then). The dashboard now
  binds first and the daemons follow in the background. Measured with MariaDB
  pointed at a daemon that never answers: reachable after **36.6s** before,
  **0.7s** after. Databases can therefore be seen still starting for a few seconds
  after the UI appears — that state is live on the Database page, not hidden.
- **`composer run dev` was probed on a port Laravel never uses.** `laravel-dev`
  exported `APP_PORT`, which Laravel does not read; `artisan serve` takes its
  bind address from `SERVER_HOST`/`SERVER_PORT`. The readiness check therefore
  timed out after 15s and killed a healthy dev tree. Measured: `SERVER_PORT=8127`
  makes Laravel bind 8127 and leaves 8000 free, and a start/stop/start cycle now
  reports 8000 then 8001.
- **Laravel generated URLs for a dead port.** Nothing exported `APP_URL`, so
  `url()`/`asset()`/redirects followed .env's `http://localhost:8000` — a port
  alive only while `composer run dev` happens to run. The proxied PHP process now
  receives `APP_URL` (and `SABDOPALON_URL`) for the site's real address; a
  per-site `env: APP_URL=…` still wins.
- **A transient `SABDOPALON_BIN_DIR` was written to the user's persistent PATH.**
  Found while taking the measurement above: the harness left its temp bin folder
  at position 1 of the real user PATH, where it shadowed the real install.
  `userBinDir`'s own contract forbids exactly that; `prepareBinDir` now skips the
  persistent write for a transient override.

### Added
- **Projects outside `sites/` can be served.** `sabdopalon link <name> <path>`
  links an existing project (a junction on Windows — no admin or Developer Mode —
  a symlink elsewhere) into `sites/`, and the scanner follows links. `os.ReadDir`
  reports a *link's* own type and `DirEntry.IsDir()` is false for symlinks and
  junctions alike, so such a project previously vanished without a word.
  `bin/php/<X.Y>` links are detected the same way, so a hand-supplied PHP build is
  no longer reported as "not installed".
- **`sabdopalon doctor` reports who really owns `php`/`mysql`.** Windows composes
  the process PATH as machine entries first, then per-user ones, so a tool in the
  machine PATH cannot be outranked from HKCU. The report names the winner and,
  when it is beyond reach, prints the elevated fix. Commands Sabdopalon does not
  provide (composer, node, npm) are never reported.
- **npm/pnpm/yarn/bun install tools** in the Dev Tools tab, chosen by lockfile
  exactly the way Laravel's own `NodePackageManager` chooses them, plus a
  preflight that turns a missing `node_modules` into "run `npm install` first"
  instead of a tool that reports "ready" while every asset 404s.

### Changed
- The `bin/` entry is now moved to the **front** of the user PATH instead of
  being appended. On the machine this was measured on it sat last, so `php` and
  `mysql` resolved to Herd/Laragon/MySQL builds while Sabdopalon's own shims went
  unused. (A machine-wide PATH entry still wins over any per-user one — `doctor`
  says so explicitly.)
- A Laravel project is no longer offered a second, standalone Vite when its
  composer `dev` script already starts one: two servers both write `public/hot`.
- The generated `engine.toml` comment now describes the real `[php]` resolution
  order — explicit binary → `prefer` (default `system`, i.e. the first `php` on
  PATH) → bundled — which it previously stated backwards.

## [0.11.1] — 2026-09-21

Builds on the Windows compatibility work in `5ff9337` and fixes three defects
found while verifying it against a real Windows install.

### Fixed
- **The setup wizard failed on every Windows desktop install** — it stopped at
  the phpMyAdmin step with `deploy phpMyAdmin: copy phpMyAdmin: symlink …: A
  required privilege is not held by the client`. Desktop installs expose the
  bundled stack through a link (`bin/phpmyadmin` → `resources/core/phpmyadmin`),
  and `copyTree` treated that linked source root as a link to recreate rather
  than a directory to copy: it skipped the branch that would have created
  `sites/phpmyadmin/`, then called `os.Symlink` at a path whose parent did not
  exist — which Windows reports as that privilege error, not as a missing
  directory. The deploy now follows the link, creates the destination, and
  copies the contents, so `sites/phpmyadmin/public` is a real tree and
  `resources/` is never written to.
- **`sabdopalon add <pkg>` replaced a component provided by the bundled stack** —
  when a package target is itself a link into `resources/` (e.g.
  `bin/phpmyadmin`), the installer treated the healthy bundle as an
  "incomplete/unverified installation", renamed it aside and substituted a fresh
  download, all while reporting success. It now recognises the link and leaves it
  in place. Note: installing a *different version* of a component whose target
  sits **inside** a linked directory (`bin/php/8.5`) still writes through the link
  into the bundle — tracked separately.
- **Black console windows flashed on a loop in the Packages tab** —
  `/api/sys-tools` probes every registered tool with `<bin> --version`, and the
  tab polls that endpoint every 8s. The probe did not mark its child, and the
  desktop sidecar is linked with `-H windowsgui` (so it owns no console): Windows
  handed each probe a brand-new console window, giving three flashes every 8s
  with no error text anywhere to explain them. The probe now goes through
  `winproc.Quiet` (`CREATE_NO_WINDOW` + `HideWindow`), the same helper every other
  Windows spawn site uses — it was the last unmarked one of 55.

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
  with live inventory: **"Included"** lists PHP/MariaDB/phpMyAdmin
  with real installed-state badges (not static text), **"Additional Tools"** on
  the right shows only not-yet-installed tools as checkboxes (Redis hidden on
  Linux/macOS; MinIO version hidden when too long), **"Advanced settings"**
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
