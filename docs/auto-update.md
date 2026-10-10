# Auto-update on Windows — design note (future work)

Status: **not implemented.** This records the analysis so the decision does not
have to be redone. Nothing here changes behaviour today.

## The problem it solves

Updating Sabdopalon currently means: open GitHub, download a 59 MB installer, run
it, reopen the app. Reinstalling is already non-destructive — since 0.11.15 the
program lives in `%LOCALAPPDATA%\Sabdopalon` and **data lives in
`C:\sabdopalon`**, so sites, databases, config and certificates are untouched and
the setup wizard does not run again. What remains is the manual download.

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

## Recommended shape: one button, two paths

Both paths are chosen automatically, from what the release contains.

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

**How CI decides, without anyone tagging anything**

```
git diff --name-only <previous-tag> HEAD -- desktop/src-tauri
```

Empty → publish the sidecar asset → clients offer the fast path.
Non-empty → publish only the installer → clients offer the installer path.

The presence of the sidecar asset *is* the signal; no separate manifest is
needed, and the hash is already available from GitHub's `digest` field.

## Windows details that make it practical

- **Per-user install** (`%LOCALAPPDATA%\Sabdopalon`) → **no UAC prompt** for a
  silent update. This is what usually blocks unattended updates.
- **Separate data root** → the installer only replaces the program; sites,
  databases and config survive, and no setup runs.
- **Brief interruption**: the app must exit, so MariaDB and the PHP workers stop
  too. A relaunch brings them back (sites start on demand). Roughly 30–60
  seconds.
- **Relaunch**: the reliable trick is to spawn a detached
  `cmd /c start /wait <installer> … && start <app>` so something outlives the app
  and starts it again. The installer's own relaunch flag, if it has one, needs
  verifying at implementation time.
- **Silent/passive install**: the exact NSIS flags need verifying. Even a normal
  installer window is far less work than the manual flow.
- **Prefer a sidecar copy in the data root** (`<root>/bin/sabdopalon.exe`; the
  shell already seeds the CLI there) so a swap never writes into the installed
  program directory.

## Honest limitations

- **Unsigned installer.** A freshly downloaded installer can raise SmartScreen
  once, because Windows tags downloads with Mark of the Web. The app can clear
  that tag after the hash matches, but the real fix is a code-signing
  certificate — which is also what makes this feel seamless for anyone else.
- **sha256 from the same source as the download** is TLS-bound. It protects
  against a truncated or corrupted download, not against a compromised origin.
  The cryptographically stronger option is Tauri's official updater
  (`tauri-plugin-updater`, minisign signatures + `latest.json`), which is the
  path to take if Sabdopalon is ever distributed to other people.
- **Version skew**: a new sidecar must keep working with an older shell. The
  protocol is small (argv, environment, HTTP), so the risk is low — but shell
  changes should stay rare and additive, and the sidecar must not require a newer
  shell.
