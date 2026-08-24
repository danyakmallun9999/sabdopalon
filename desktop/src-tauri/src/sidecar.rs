// Sidecar management: locate the bundled Go binary, launch it with
// SABDOPALON_DIR pointing at the user-facing install root (~/Sabdopalon),
// wait for the dashboard HTTP endpoint, then point the window at it.
use std::io::Write;
use std::path::PathBuf;
use std::process::Child;
use std::sync::Mutex;
use std::time::Duration;

use tauri::{AppHandle, Emitter, Manager, WebviewWindow};

// The sidecar process (owned here so it is killed on quit).
static SIDECAR: Mutex<Option<Child>> = Mutex::new(None);

/// The install root where the sidecar keeps engine.toml, sites/, data/…
///
/// Friendly, user-visible location: `<home>/Sabdopalon` — Linux
/// `/home/<user>/Sabdopalon`, Windows `C:\Users\<user>\Sabdopalon`.
/// Herd-style: the app itself is installed read-only elsewhere; only this
/// folder holds user data.
///
/// Back-compat: installs created before the friendly default lived under the
/// OS app-data dir (`~/.local/share/com.sabdopalon.app` and friends). When
/// that legacy dir is bootstrapped (engine.toml present) and the friendly
/// one is not yet, keep using it — an upgrade must never strand user data.
pub fn data_dir(app: &AppHandle) -> PathBuf {
    let friendly = home_dir().join("Sabdopalon");
    if let Ok(legacy) = app.path().app_data_dir() {
        if legacy.join("config").join("engine.toml").is_file()
            && !friendly.join("config").join("engine.toml").is_file()
        {
            return legacy;
        }
    }
    friendly
}

/// The user's home directory (HOME on Unix, USERPROFILE on Windows).
fn home_dir() -> PathBuf {
    let key = if cfg!(windows) { "USERPROFILE" } else { "HOME" };
    std::env::var_os(key)
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."))
}

/// Locate the sidecar binary candidates, best first:
/// 1. next to the running executable — `target/release/sabdopalon` (dev/build)
/// 2. bundle resource dir (installed app) — `binaries/sabdopalon`
/// 3. repo layout (dev) — `../binaries/sabdopalon` next to src-tauri
///
/// On Windows the binaries carry a `.exe` suffix; try both spellings.
/// The ORDER alone is not trusted: `choose_sidecar` probes each candidate's
/// reported version so a stale binary left beside the installed exe can never
/// silently override the freshly-bundled one (this is how pre-ConPTY sidecars
/// kept opening external Windows Terminal windows after upgrades).
fn sidecar_candidates(app: &AppHandle) -> Vec<PathBuf> {
    let exe_dir = match std::env::current_exe().ok().as_ref().and_then(|p| p.parent()) {
        Some(p) => p.to_path_buf(),
        None => return Vec::new(),
    };
    let names: &[&str] = if cfg!(windows) {
        &["sabdopalon.exe", "sabdopalon"]
    } else {
        &["sabdopalon"]
    };
    let mut candidates: Vec<PathBuf> = Vec::new();
    for n in names {
        candidates.push(exe_dir.join(n));
        candidates.push(exe_dir.join("binaries").join(n));
        candidates.push(exe_dir.join("../binaries").join(n));
    }
    if let Ok(p) = app.path().resolve("binaries/sabdopalon", tauri::path::BaseDirectory::Resource) {
        candidates.push(p);
    }
    if cfg!(windows) {
        if let Ok(p) = app.path().resolve("binaries/sabdopalon.exe", tauri::path::BaseDirectory::Resource) {
            candidates.push(p);
        }
    }
    candidates.into_iter().filter(|p| p.is_file()).collect()
}

/// Run `<bin> version` and return its trimmed stdout ("sabdopalon 0.8.3").
fn probe_version(bin: &PathBuf) -> Option<String> {
    let out = std::process::Command::new(bin)
        .arg("version")
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
    if s.is_empty() {
        None
    } else {
        Some(s)
    }
}

/// Copy src → dst via a temp file + rename, so a crash mid-copy can never
/// leave a half-written binary at the destination. Unix permissions
/// (including the exec bit) are preserved by std::fs::copy.
fn copy_atomic(src: &PathBuf, dst: &PathBuf) -> std::io::Result<()> {
    let tmp = dst.with_extension("tmp");
    std::fs::copy(src, &tmp)?;
    std::fs::rename(&tmp, dst)
}

/// Pick the sidecar binary whose self-reported version matches the app's own
/// version. Falls back to the first available candidate (old behaviour) while
/// recording a warning so stale copies show up in logs/sidecar.log.
fn choose_sidecar(app: &AppHandle, issues: &mut Vec<String>) -> Option<PathBuf> {
    let expected = app.package_info().version.to_string();
    let mut fallback: Option<PathBuf> = None;
    for cand in sidecar_candidates(app) {
        match probe_version(&cand) {
            Some(v) if v.contains(&expected) => return Some(cand),
            Some(v) => issues.push(format!(
                "stale sidecar skipped: {} reports \"{}\" (expected {})",
                cand.display(),
                v,
                expected
            )),
            None => issues.push(format!(
                "sidecar probe failed: {} (no/broken version output)",
                cand.display()
            )),
        }
        if fallback.is_none() {
            fallback = Some(cand.clone());
        }
    }
    fallback
}

/// Spawn the sidecar (sabdopalon binary from the bundle).
///
/// First run (no engine.toml yet) boots in setup mode so the GUI wizard
/// shows; afterwards the sidecar serves the normal dashboard.
/// stdout/stderr are redirected to `<data>/logs/sidecar.log` so crashes and
/// panics leave a trace (invisible on Windows thanks to -H windowsgui).
pub fn start(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    let dir = data_dir(app);
    std::fs::create_dir_all(&dir)?;

    let bootstrapped = dir.join("config/engine.toml").is_file();

    // Crash/console logging for the sidecar. Rotate first — an append-only
    // log would grow unbounded over weeks of daily use.
    let logs_dir = dir.join("logs");
    std::fs::create_dir_all(&logs_dir)?;
    let sidecar_log = logs_dir.join("sidecar.log");
    if let Ok(meta) = std::fs::metadata(&sidecar_log) {
        const ROTATE_AT: u64 = 10 * 1024 * 1024; // 10 MB
        if meta.len() > ROTATE_AT {
            let old = logs_dir.join("sidecar.log.old");
            let _ = std::fs::remove_file(&old);
            let _ = std::fs::rename(&sidecar_log, &old);
        }
    }
    let mut log_file = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&sidecar_log)?;

    // Everything noteworthy found while preparing the launch (stale
    // sidecars, failed bundle copies) lands in the same log — stderr is
    // invisible under -H windowsgui.
    let mut issues: Vec<String> = Vec::new();

    // Version handshake: never blindly run whatever binary happens to sit
    // next to the installed exe.
    let bin = choose_sidecar(app, &mut issues)
        .ok_or_else(|| "sidecar binary not found (binaries/sabdopalon)".to_string())?;

    // Core stack (PHP/MariaDB/phpMyAdmin) ships in the app's resource dir on
    // full-bundle builds. Tauri copies resources preserving their
    // src-tauri-relative path, so inside an AppImage it lives under
    // <resource>/resources/core (dev layouts may expose <resource>/core).
    // The sidecar's bin root is ALWAYS a writable dir (<data>/bin): bundled
    // entries are symlinked in, and future downloads land beside them —
    // installs must never target the read-only AppImage mount.
    let core = ["resources/core", "core"].iter().find_map(|rel| {
        app.path()
            .resolve(rel, tauri::path::BaseDirectory::Resource)
            .ok()
            .filter(|p| p.is_dir())
    });

    let bin_dir = dir.join("bin");
    std::fs::create_dir_all(&bin_dir)?;
    let mut core_archive: Option<PathBuf> = None;
    if let Some(core) = &core {
        if let Ok(entries) = std::fs::read_dir(core) {
            for e in entries.flatten() {
                // The linux bundle ships the core as ONE archive (no ELFs for
                // linuxdeploy to scan); the Go side extracts it on first run.
                if e.file_name() == *"core.tar.gz" {
                    core_archive = Some(e.path());
                    continue;
                }
                let dst = bin_dir.join(e.file_name());
                if dst.exists() || dst.is_symlink() {
                    if entry_looks_complete(&e.path(), &dst) {
                        continue;
                    }
                    issues.push(format!(
                        "repairing incomplete bundled entry '{}' in bin/ (re-copying)",
                        e.file_name().to_string_lossy()
                    ));
                    if dst.is_dir() && !dst.is_symlink() {
                        let _ = std::fs::remove_dir_all(&dst);
                    } else {
                        let _ = std::fs::remove_file(&dst);
                    }
                }
                if let Err(err) = link_or_copy(&e.path(), &dst) {
                    issues.push(format!(
                        "bundling '{}' into bin/ failed: {} — this component will be missing or partial",
                        e.file_name().to_string_lossy(),
                        err
                    ));
                }
            }
        }
    }

    // Seed a runnable copy of the CLI itself into <data>/bin — the built-in
    // terminal has that dir on PATH, so users can drive the same install
    // from the shell: `sabdopalon doctor|add|sites|…`. A real copy, not a
    // symlink: it survives outside the read-only AppImage mount and can
    // even take file capabilities (enable-ports). Refreshed on every app
    // update via the version probe; failures are non-fatal.
    let cli_dst = bin_dir.join(if cfg!(windows) { "sabdopalon.exe" } else { "sabdopalon" });
    let expected = app.package_info().version.to_string();
    match probe_version(&cli_dst) {
        Some(v) if v.contains(&expected) => {}
        stale => {
            if let Some(v) = stale {
                issues.push(format!(
                    "refreshing stale CLI copy in bin/: reports \"{v}\" (expected {expected})"
                ));
            }
            if let Err(err) = copy_atomic(&bin, &cli_dst) {
                issues.push(format!(
                    "seeding CLI into bin/ failed: {err} — 'sabdopalon' won't be on the terminal PATH"
                ));
            }
        }
    }

    for line in &issues {
        let _ = writeln!(&mut log_file, "[sidecar] {line}");
    }
    let _ = log_file.flush();

    // Ship the default package registry into <data>/packages. Only files
    // that are MISSING are seeded — a user's edited packages.toml always
    // wins, and the Go binary additionally embeds the default registry.
    let pkg_res = ["resources/packages", "packages"].iter().find_map(|rel| {
        app.path()
            .resolve(rel, tauri::path::BaseDirectory::Resource)
            .ok()
            .filter(|p| p.is_dir())
    });
    if let Some(pkg_res) = pkg_res {
        let data_pkgs = dir.join("packages");
        if std::fs::create_dir_all(&data_pkgs).is_ok() {
            if let Ok(entries) = std::fs::read_dir(&pkg_res) {
                for e in entries.flatten() {
                    let dst = data_pkgs.join(e.file_name());
                    if !dst.exists() {
                        if let Err(err) = std::fs::copy(e.path(), &dst) {
                            let _ = writeln!(
                                &mut log_file,
                                "[sidecar] seeding packages/{} failed: {}",
                                e.file_name().to_string_lossy(),
                                err
                            );
                        }
                    }
                }
            }
        }
    }

    let mut cmd = std::process::Command::new(bin);
    // The sidecar owns the data dir; never opens the browser (the
    // native window IS the dashboard).
    cmd.env("SABDOPALON_DIR", &dir)
        .env("SABDOPALON_BIN_DIR", &bin_dir)
        .arg("--no-open");
    if let Some(archive) = core_archive {
        cmd.env("SABDOPALON_CORE_ARCHIVE", archive);
    }
    cmd.stdout(std::process::Stdio::from(log_file.try_clone()?))
        .stderr(std::process::Stdio::from(log_file));
    if !bootstrapped {
        cmd.arg("--setup-mode");
    }
    let child = cmd.spawn()?;

    {
        let mut guard = SIDECAR.lock().unwrap();
        *guard = Some(child);
    }

    // Setup-mode watcher: the wizard writes config/engine.toml when it
    // finishes, but the RUNNING sidecar is the config-less setup instance —
    // no proxy, no DB manager — so it can never serve the real dashboard
    // (the dashboard meanwhile reloads into full chrome: "Proxy: 0", toast
    // "database manager not available (setup mode)"). Restart the sidecar
    // once the config appears; the next spawn skips --setup-mode.
    if !bootstrapped {
        let app = app.clone();
        let cfg_path = dir.join("config").join("engine.toml");
        let log_path = sidecar_log.clone();
        std::thread::spawn(move || {
            for _ in 0..2400 {
                std::thread::sleep(Duration::from_millis(750));
                if !cfg_path.is_file() {
                    continue;
                }
                let mut log = std::fs::OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(&log_path);
                match &mut log {
                    Ok(f) => {
                        let _ = writeln!(f, "[sidecar] setup finished — restarting sidecar in full mode");
                    }
                    Err(_) => eprintln!("[sidecar] setup finished — restarting sidecar in full mode"),
                }
                stop();
                std::thread::sleep(Duration::from_millis(300));
                if let Err(err) = start(&app) {
                    eprintln!("[sidecar] restart after setup failed: {err}");
                }
                return;
            }
            eprintln!("[sidecar] setup watcher gave up (no config after 30 min)");
        });
    }

    #[cfg(target_os = "linux")]
    integrate_desktop_entry(&mut issues);

    Ok(())
}

/// Linux only: register a user-level launcher so the AppImage shows up in
/// the GNOME/KDE dash with the camel icon (a bare AppImage run has no
/// .desktop entry, so the shell shows a generic icon). Idempotent — files
/// are rewritten only when their content changes — and entirely contained
/// in the user's own ~/.local/share (no root, no system dirs).
///
/// Naming MUST match what Tauri's bundler ships inside the AppImage:
/// the binary (and therefore the GTK WM_CLASS) and the bundled icon are
/// both named after the Cargo package — `sabdopalon-desktop`. A
/// StartupWMClass that doesn't match means the running window can never be
/// associated with this launcher (generic icon in the dock).
#[cfg(target_os = "linux")]
fn integrate_desktop_entry(issues: &mut Vec<String>) {
    const APP_ID: &str = "sabdopalon-desktop"; // = Cargo package name = WM_CLASS
    let home = match std::env::var("HOME") {
        Ok(h) if !h.is_empty() => h,
        _ => return,
    };
    // Exec: the AppImage path (stable across the /tmp/.mount_* lifetime —
    // the runtime exposes it via $APPIMAGE); fall back to the real binary
    // for deb/dev layouts.
    let exec = std::env::var("APPIMAGE")
        .ok()
        .filter(|s| !s.is_empty())
        .or_else(|| std::env::current_exe().ok().map(|p| p.display().to_string()));
    let Some(exec) = exec else { return };

    let icon_dir =
        std::path::PathBuf::from(format!("{home}/.local/share/icons/hicolor/512x512/apps"));
    let app_dir = std::path::PathBuf::from(format!("{home}/.local/share/applications"));
    if std::fs::create_dir_all(&icon_dir).is_err() || std::fs::create_dir_all(&app_dir).is_err() {
        issues.push("desktop integration: could not create ~/.local/share dirs".into());
        return;
    }

    let icon_dst = icon_dir.join(format!("{APP_ID}.png"));
    let icon_bytes: &[u8] = include_bytes!("../icons/icon.png");
    if std::fs::read(&icon_dst).map(|b| b != icon_bytes).unwrap_or(true) {
        if let Err(err) = std::fs::write(&icon_dst, icon_bytes) {
            issues.push(format!("desktop integration: icon write failed: {err}"));
            return;
        }
    }

    let entry = format!(
        "[Desktop Entry]\n\
         Type=Application\n\
         Name=Sabdopalon\n\
         GenericName=Local PHP dev server\n\
         Comment=PHP + MariaDB + phpMyAdmin — portabel dalam satu folder\n\
         Exec=\"{exec}\"\n\
         Icon={APP_ID}\n\
         StartupWMClass={APP_ID}\n\
         Categories=Development;\n\
         Terminal=false\n\
         StartupNotify=true\n"
    );
    let dst = app_dir.join(format!("{APP_ID}.desktop"));
    if std::fs::read_to_string(&dst).map(|c| c != entry).unwrap_or(true) {
        if let Err(err) = std::fs::write(&dst, entry) {
            issues.push(format!("desktop integration: .desktop write failed: {err}"));
            return;
        }
        issues.push(format!(
            "desktop launcher installed/updated (~/.local/share/applications/{APP_ID}.desktop)"
        ));
    }
}

/// entry_looks_complete decides whether an already-present bundled entry in
/// bin/ can be trusted. Only known-fragile entries are verified: a copied
/// phpmyadmin tree without libraries/constants.php is the classic symptom of
/// an interrupted copy (it produces "failed opening required
/// .../libraries/constants.php" fatals on phpmyadmin.localhost).
fn entry_looks_complete(src: &std::path::Path, dst: &std::path::Path) -> bool {
    if src.is_dir() && src.file_name().map(|n| n == std::ffi::OsStr::new("phpmyadmin")).unwrap_or(false) {
        let canary = src.join("libraries").join("constants.php");
        if canary.is_file() && !dst.join("libraries").join("constants.php").is_file() {
            return false;
        }
    }
    true
}

/// Symlink src → dst, falling back to a copy when symlinks are unavailable
/// (Windows without developer mode). Directories are copied recursively as
/// a last resort only. Errors are RETURNED — silently discarding them is how
/// partial trees used to end up looking installed.
fn link_or_copy(src: &PathBuf, dst: &PathBuf) -> std::io::Result<()> {
    #[cfg(unix)]
    {
        if std::os::unix::fs::symlink(src, dst).is_ok() {
            return Ok(());
        }
    }
    #[cfg(windows)]
    {
        use std::os::windows::fs as wfs;
        let ok = if src.is_dir() {
            wfs::symlink_dir(src, dst).is_ok()
        } else {
            wfs::symlink_file(src, dst).is_ok()
        };
        if ok {
            return Ok(());
        }
    }
    if src.is_dir() {
        copy_dir_recursive(src, dst)
    } else {
        std::fs::copy(src, dst).map(|_| ())
    }
}

fn copy_dir_recursive(src: &PathBuf, dst: &PathBuf) -> std::io::Result<()> {
    std::fs::create_dir_all(dst)?;
    for e in std::fs::read_dir(src)?.flatten() {
        let d = dst.join(e.file_name());
        if e.path().is_dir() {
            copy_dir_recursive(&e.path(), &d)?;
        } else {
            std::fs::copy(e.path(), d)?;
        }
    }
    Ok(())
}

/// Stop the sidecar GRACEFULLY. The Go side handles SIGTERM by running its
/// full shutdown path (stops sites, databases and services — those live in
/// their own process groups and would be ORPHANED by a hard kill: MariaDB
/// kept running after Quit). So: SIGTERM first, wait bounded, SIGKILL as
/// the last resort. Windows has no signals — taskkill /T takes the whole
/// process tree down instead.
pub fn stop() {
    let mut guard = SIDECAR.lock().unwrap();
    if let Some(mut child) = guard.take() {
        #[cfg(unix)]
        {
            let _ = std::process::Command::new("kill")
                .arg(child.id().to_string())
                .status();
            for _ in 0..40 {
                match child.try_wait() {
                    Ok(Some(_)) => return, // graceful exit confirmed
                    _ => std::thread::sleep(Duration::from_millis(250)),
                }
            }
        }
        #[cfg(windows)]
        {
            let _ = std::process::Command::new("taskkill")
                .args(["/T", "/F", "/PID", &child.id().to_string()])
                .status();
        }
        let _ = child.kill();
        let _ = child.wait();
    }
}

/// Poll 127.0.0.1:9900 until the dashboard answers, then show the window.
pub fn wait_ready(app: AppHandle, win: WebviewWindow) {
    std::thread::spawn(move || {
        for _ in 0..120 {
            if std::net::TcpStream::connect("127.0.0.1:9900").is_ok() {
                let _ = win.show();
                let _ = win.set_focus();
                return;
            }
            std::thread::sleep(Duration::from_millis(500));
        }
        // Timeout: still show the window so the user sees an error page
        // rather than nothing.
        let _ = win.show();
        let _ = app.emit("sidecar-timeout", ());
    });
}
