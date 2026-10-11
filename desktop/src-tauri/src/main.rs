// Sabdopalon desktop shell: spawns the Go sidecar (which serves the
// dashboard on 127.0.0.1:9900) in the user-data dir, shows the native
// window, and wires up tray + autostart.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::Manager;

mod sidecar;
mod tray;

/// Replace the running sidecar with a freshly verified download, then restart it.
///
/// The dashboard downloads the file and verifies its checksum in Go, then asks for
/// this: only the shell can replace the process it is currently running. Returns the
/// path that was replaced, or a sentence explaining what went wrong — including, on
/// failure, that the previous sidecar was put back.
#[tauri::command]
fn apply_sidecar_update(app: tauri::AppHandle, path: String) -> Result<String, String> {
    sidecar::apply_update(&app, std::path::Path::new(&path))
}

/// Open a folder from the dashboard with the OS file manager.
///
/// A small command rather than the opener plugin's openPath: that one is scoped to a
/// static set of paths, and this program's data root is whatever the user configured —
/// `C:\sabdopalon`, or anywhere else via SABDOPALON_HOME. The path always comes from the
/// dashboard's own about payload, never from user input.
#[tauri::command]
fn open_path(path: String) -> Result<(), String> {
    let target = std::path::PathBuf::from(&path);
    if !target.exists() {
        return Err(format!("{path} does not exist"));
    }
    #[cfg(target_os = "windows")]
    let mut cmd = {
        let mut c = std::process::Command::new("explorer");
        c.arg(&target);
        c
    };
    #[cfg(target_os = "macos")]
    let mut cmd = {
        let mut c = std::process::Command::new("open");
        c.arg(&target);
        c
    };
    #[cfg(all(unix, not(target_os = "macos")))]
    let mut cmd = {
        let mut c = std::process::Command::new("xdg-open");
        c.arg(&target);
        c
    };
    cmd.spawn()
        .map(|_| ())
        .map_err(|e| format!("could not open {path}: {e}"))
}

/// Run the downloaded installer and step aside so it can replace the program.
///
/// An installer cannot replace files while Sabdopalon runs, so something has to
/// outlive this process: a detached shell waits for the installer to finish and then
/// starts the app again. The installer is per-user, so none of this needs elevation.
///
/// The installer window stays visible. A silent NSIS run is a flag nobody has verified
/// here, and a progress bar the user can see beats a silent failure they cannot.
#[tauri::command]
fn apply_installer_update(app: tauri::AppHandle, path: String) -> Result<(), String> {
    let installer = std::path::PathBuf::from(&path);
    if !installer.is_file() {
        return Err(format!("the downloaded installer is not there: {path}"));
    }
    let exe = std::env::current_exe().map_err(|e| format!("cannot find my own path: {e}"))?;

    let script = format!(
        "start \"\" /wait \"{}\" && start \"\" \"{}\"",
        installer.display(),
        exe.display()
    );
    std::process::Command::new("cmd")
        .args(["/C", &script])
        .spawn()
        .map_err(|e| format!("could not start the installer: {e}"))?;

    // Let the spawn take hold, then exit so the installer can write over the program
    // files. The detached shell above brings the app back when it is done.
    std::thread::spawn(move || {
        std::thread::sleep(std::time::Duration::from_millis(900));
        app.exit(0);
    });
    Ok(())
}

fn main() {
    tauri::Builder::default()
        // Exactly one instance: a second launch focuses the running window
        // instead of fighting over the dashboard port (the source of
        // "connection refused" confusion after close-to-tray).
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            use tauri::Manager;
            if let Some(win) = app.get_webview_window("main") {
                let _ = win.show();
                let _ = win.set_focus();
            }
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec![]),
        ))
        .plugin(tauri_plugin_opener::init())
        // Two commands, because exactly two actions cannot be done from Go: replacing
        // the sidecar binary while it runs, and running an installer that has to
        // outlive this process. Everything else the dashboard needs, it does over HTTP.
        .invoke_handler(tauri::generate_handler![
            apply_sidecar_update,
            apply_installer_update,
            open_path
        ])
        .setup(|app| {
            // Start the Go sidecar right away (it boots in setup mode when
            // the install has no config yet, so the GUI wizard works first-run).
            let handle = app.handle().clone();
            sidecar::start(&handle)?;

            // Build the tray menu: Open Dashboard / Open Sites / Start-Stop /
            // Autostart toggle / Quit.
            tray::setup(app)?;

            // Show the window once the sidecar answers on :9900.
            let win = app.get_webview_window("main").expect("main window");
            sidecar::wait_ready(handle, win);

            Ok(())
        })
        .on_window_event(|window, event| {
            // Hide to tray instead of quitting when the window closes.
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                let _ = window.hide();
                api.prevent_close();
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building Sabdopalon desktop")
        // RunEvent::Exit fires on ALL termination paths — tray Quit (which
        // calls sidecar::stop() itself, making this a harmless no-op) AND
        // OS-initiated exit: logout, shutdown, SIGTERM/SIGINT to the Tauri
        // process. Without this hook, an OS-initiated exit would leave the Go
        // sidecar and its database/service daemons orphaned, holding ports
        // 9900/8080/3306 so the next launch fails to bind.
        .run(|_app_handle, event| {
            if let tauri::RunEvent::Exit = event {
                sidecar::stop();
            }
        });
}
