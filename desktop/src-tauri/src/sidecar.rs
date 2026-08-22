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
fn sidecar_path(app: &AppHandle) -> Option<PathBuf> {
    let exe_dir = std::env::current_exe().ok()?.parent()?.to_path_buf();
    let candidates = [
        Some(exe_dir.join("sabdopalon")),
        app.path()
            .resolve("binaries/sabdopalon", tauri::path::BaseDirectory::Resource)
            .ok(),
        Some(exe_dir.join("binaries/sabdopalon")),
        Some(exe_dir.join("../binaries/sabdopalon")),
    ];
    candidates.into_iter().flatten().find(|p| p.is_file())
}

/// Spawn the sidecar (sabdopalon binary from the bundle).
///
/// First run (no engine.toml yet) boots in setup mode so the GUI wizard
/// shows; afterwards the sidecar serves the normal dashboard.
pub fn start(app: &AppHandle) -> Result<(), Box<dyn std::error::Error>> {
    let dir = data_dir(app);
    std::fs::create_dir_all(&dir)?;

    let bin = sidecar_path(app)
        .ok_or_else(|| "sidecar binary not found (binaries/sabdopalon)".to_string())?;

    let bootstrapped = dir.join("config/engine.toml").is_file();

    let mut cmd = std::process::Command::new(bin);
    // The sidecar owns the data dir; never opens the browser (the
    // native window IS the dashboard).
    cmd.env("SABDOPALON_DIR", &dir).arg("--no-open");
    if !bootstrapped {
        cmd.arg("--setup-mode");
    }
    let child = cmd.spawn()?;

    let mut guard = SIDECAR.lock().unwrap();
    *guard = Some(child);
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
