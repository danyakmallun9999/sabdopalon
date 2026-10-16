# Auto-update on Windows — design note

Status: **implemented in 0.11.20; the pipeline that decides which path applies was
fixed in 0.11.21** (see "What 0.11.21 fixed"). Checking and applying are always
separate: the app may tell you a release exists, and never installs one on its own.

## The problem it solves

Updating Sabdopalon used to mean: open GitHub, download a 59 MB installer, run it,
reopen the app. Reinstalling is already non-destructive — since 0.11.15 the program
lives in `%LOCALAPPDATA%\Sabdopalon` and **data lives in `C:\sabdopalon`**, so sites,
databases, config and certificates are untouched and the setup wizard does not run
again. What remained was the manual download.

## What the history says

Measured across the nine releases from v0.11.9 to v0.11.17:

| Release | Changed | Update could be |
|---|---|---|
| 0.11.9 … 0.11.13 | Go only | sidecar only |
| **0.11.14** | Go + Rust (`SABDOPALON_HOME`) | installer required |
| **0.11.15** | Go + Rust (`C:\sabdopalon` default) | installer required |
| 0.11.16 (starter kits) | Go + UI | sidecar only |
| 0.11.17 (new site) | Go + UI | sidecar only |

**Seven of nine releases could have been sidecar-only.** Both exceptions were
changes to the desktop shell. The dashboard UI needs no separate channel: it is
compiled into the Go binary (`//go:embed all:ui/dist`), so UI work rides along
with the sidecar.

## One button, two paths

Both paths are chosen automatically, from what the release contains. The About page
says which one applies before anything is downloaded.

**Path A — sidecar only (fast, no installer, ~10 seconds)**

1. Dashboard/tray checks `GET /releases/latest`.
2. If the release carries a sidecar asset, download just that binary.
3. Verify the sha256 from the release asset's `digest` field.
4. Stop sidecar → replace the file → start again. The shell already has this
   route: tray → Restart.

**Path B — installer (when the shell changed)**

1. Same detection, but no sidecar asset is present → the installer is needed.
2. One click: download the 59 MB installer with a progress modal.
3. Verify the sha256 before running anything.
4. Launch the installer and **quit the app** (program files are locked while it
   runs), then relaunch.

## The contract between CI and the updater

The asset **name** is the signal, and it is the only metadata that has to stay in
step. `internal/dashboard/handlers_update.go` treats a release asset whose name
contains `sidecar` as "only the Go program changed", and the installer path
otherwise. The installer is then picked by name (`setup`) rather than by position,
so a bare sidecar can never be run as an installer.

The **other** half of the contract is Tauri's ACL. The dashboard is a remote
origin (the sidecar serves it over HTTP), and Tauri 2.11 enforces capabilities for
remote origins; an application that declares no permissions of its own has no
manifest to allow its own commands against. Applying an update therefore needs
both `desktop/src-tauri/permissions/dashboard.toml` (which commands are
callable) and a capability that grants them to the main window. Missing either
half fails at the last step — after the download and the checksum — with
`Command apply_sidecar_update not allowed by ACL`, which is exactly what 0.11.21
shipped and 0.11.22 fixed.

CI decides with a git diff against the previous release tag:

```
git diff --name-only <previous-tag> HEAD -- desktop/src-tauri
```

- **No changed file** → publish `sabdopalon-sidecar-x86_64-pc-windows-msvc.exe`
  → clients offer the fast path.
- **Any unchanged-from-version change** → publish only the installer → clients
  offer the installer path.
- **No previous tag visible** → installer only. Not being able to tell is not
  permission to promise that only Go changed.
- A changed line that is only a `version` line in `Cargo.toml`, `Cargo.lock` or
  `tauri.conf.json` does **not** count: every release carries a new number, and
  counting it made the fast path unreachable. `src/`, `capabilities/`, `icons/`
  and `resources/` always count.

## What 0.11.21 fixed

The fast path had never been reachable:

- The workflow published the bare sidecar under its Tauri external-binary name
  (`sabdopalon-x86_64-pc-windows-msvc.exe`), which does not contain the word the
  updater looks for, so every release was offered as an installer update.
- The version bump every release needs touches `Cargo.toml`, `Cargo.lock` and
  `tauri.conf.json`, which the gate read as "the desktop shell changed".
- The installer was selected as "the first `.exe` asset", which was correct only
  because CI happened to upload the installer first.

After a fast update the shell binary is still the older version while the sidecar
is the new one: that is the intended shape, and the version handshake in
`sidecar.rs` falls back to the only candidate — logging a `stale sidecar skipped`
line for the previous number in `logs/sidecar.log`. It is expected, not a fault.
The next release that touches Rust publishes the installer again, which restores
shell and sidecar to the same version.

## Windows details that make it practical

- **Per-user install** (`%LOCALAPPDATA%\Sabdopalon`) → **no UAC prompt** for an
  update. This is what usually blocks unattended updates.
- **Separate data root** → the installer only replaces the program; sites,
  databases and config survive, and no setup runs.
- **Brief interruption**: the app must exit, so MariaDB and the PHP workers stop
  too. A relaunch brings them back (sites start on demand). Roughly 30–60 seconds.
- **Relaunch**: a detached `cmd /c start /wait <installer> … && start <app>` so
  something outlives the app and starts it again.

## Honest limitations

- **Unsigned installer.** A freshly downloaded installer can raise SmartScreen
  once, because Windows tags downloads with Mark of the Web. The real fix is a
  code-signing certificate.
- **sha256 from the same source as the download** is TLS-bound. It protects
  against a truncated or corrupted download, not against a compromised origin.
  The cryptographically stronger option is Tauri's official updater
  (`tauri-plugin-updater`, minisign signatures + `latest.json`); the signing key
  is already in place, and the plugin was left out in 0.11.20 only because its
  `ring` dependency does not build on the development machine. Adding it needs a
  clean build first.
- **Version skew**: a new sidecar must keep working with an older shell. The
  protocol is small (argv, environment, HTTP), so the risk is low — but shell
  changes should stay rare and additive, and the sidecar must not require a newer
  shell.
