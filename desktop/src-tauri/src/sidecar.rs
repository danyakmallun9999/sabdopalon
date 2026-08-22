// Sidecar management: locate the bundled Go binary, launch it with
// SABDOPALON_DIR pointing at the OS user-data dir, wait for the dashboard
// HTTP endpoint, then point the window at it.
use std::path::PathBuf;
use std::process::Child;
use std::sync::Mutex;
use std::time::Duration;

use tauri::{AppHandle, Emitter, Manager, WebviewWindow};

// The sidecar process (owned here so it is killed on quit).
static SIDECAR: Mutex<Option<Child>> = Mutex::new(None);

/// The OS user-data dir where the sidecar keeps engine.toml, sites/, data/…
/// (Herd-style: the app itself is installed read-only elsewhere).
pub fn data_dir(app: &AppHandle) -> PathBuf {
    app.path()
        .app_data_dir()
        .expect("app data dir")
}

/// Locate the sidecar binary. Resolution order:
/// 1. next to the running executable — `target/release/sabdopalon` (dev/build)
/// 2. bundle resource dir (installed app) — `binaries/sabdopalon`
/// 3. repo layout (dev) — `../binaries/sabdopalon` next to src-tauri
///
/// On Windows the binaries carry a `.exe` suffix; try both spellings.
fn sidecar_path(app: &AppHandle) -> Option<PathBuf> {
    let exe_dir = std::env::current_exe().ok()?.parent()?.to_path_buf();
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
    candidates.into_iter().find(|p| p.is_file())
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

    let bin = sidecar_path(app)
        .ok_or_else(|| "sidecar binary not found (binaries/sabdopalon)".to_string())?;

    let bootstrapped = dir.join("config/engine.toml").is_file();

    // Crash/console logging for the sidecar.
    let logs_dir = dir.join("logs");
    std::fs::create_dir_all(&logs_dir)?;
    let log_file = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(logs_dir.join("sidecar.log"))?;

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
    if let Some(core) = &core {
        if let Ok(entries) = std::fs::read_dir(core) {
            for e in entries.flatten() {
                let dst = bin_dir.join(e.file_name());
                if dst.exists() || dst.is_symlink() {
                    continue;
                }
                link_or_copy(&e.path(), &dst);
            }
        }
    } else if cfg!(windows) {
        // No bundled resources at all — plain data/bin it is.
    }

    let mut cmd = std::process::Command::new(bin);
    // The sidecar owns the data dir; never opens the browser (the
    // native window IS the dashboard).
    cmd.env("SABDOPALON_DIR", &dir)
        .env("SABDOPALON_BIN_DIR", &bin_dir)
        .arg("--no-open")
        .stdout(std::process::Stdio::from(log_file.try_clone()?))
        .stderr(std::process::Stdio::from(log_file));
    if !bootstrapped {
        cmd.arg("--setup-mode");
    }
    let child = cmd.spawn()?;

    let mut guard = SIDECAR.lock().unwrap();
    *guard = Some(child);
    Ok(())
}

/// Symlink src → dst, falling back to a copy when symlinks are unavailable
/// (Windows without developer mode). Directories are copied recursively as
/// a last resort only.
fn link_or_copy(src: &PathBuf, dst: &PathBuf) {
    #[cfg(unix)]
    {
        if std::os::unix::fs::symlink(src, dst).is_ok() {
            return;
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
            return;
        }
    }
    if src.is_dir() {
        let _ = copy_dir_recursive(src, dst);
    } else {
        let _ = std::fs::copy(src, dst);
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

/// Stop the sidecar (used by tray Quit and app exit).
pub fn stop() {
    let mut guard = SIDECAR.lock().unwrap();
    if let Some(mut child) = guard.take() {
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
