// Sabdopalon desktop shell: spawns the Go sidecar (which serves the
// dashboard on 127.0.0.1:9900) in the user-data dir, shows the native
// window, and wires up tray + autostart.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::Manager;

mod sidecar;
mod tray;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec![]),
        ))
        .plugin(tauri_plugin_opener::init())
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
        .run(tauri::generate_context!())
        .expect("error while running Sabdopalon desktop");
}
