// Tray menu: Open Dashboard, Open Sites Folder, Start/Stop (restart the
// sidecar), Autostart toggle, and Quit.
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem};
use tauri::tray::TrayIconBuilder;
use tauri::{AppHandle, Manager};
use tauri_plugin_opener::OpenerExt;

use crate::sidecar;

pub fn setup(app: &tauri::App) -> tauri::Result<()> {
    let open_dash_item = MenuItem::with_id(app, "open-dashboard", "Open Dashboard", true, None::<&str>)?;
    let open_sites_item = MenuItem::with_id(app, "open-sites", "Open Sites Folder", true, None::<&str>)?;
    let restart_item = MenuItem::with_id(app, "restart", "Restart Server", true, None::<&str>)?;
    let autostart_item = MenuItem::with_id(app, "toggle-autostart", "Start at Login", true, None::<&str>)?;
    let quit_item = MenuItem::with_id(app, "quit", "Quit Sabdopalon", true, None::<&str>)?;

    let menu = Menu::with_items(
        app,
        &[
            &open_dash_item,
            &open_sites_item,
            &restart_item,
            &autostart_item,
            &PredefinedMenuItem::separator(app)?,
            &quit_item,
        ],
    )?;

    // Use the app icon when available; never panic on headless/resource
    // edge cases (a panic here would force-close the whole app on some
    // platforms — e.g. Windows NSIS builds).
    let mut tray = TrayIconBuilder::with_id("sabdopalon-tray")
        .menu(&menu)
        .show_menu_on_left_click(false);
    if let Some(icon) = app.default_window_icon().cloned() {
        tray = tray.icon(icon);
    }
    let _tray = tray
        .on_menu_event(|app, event| match event.id().as_ref() {
            "open-dashboard" => open_dashboard(app),
            "open-sites" => open_sites(app),
            "restart" => restart(app),
            "toggle-autostart" => toggle_autostart(app),
            "quit" => quit(app),
            _ => {}
        })
        .build(app)?;

    Ok(())
}

fn open_dashboard(app: &AppHandle) {
    if let Some(win) = app.get_webview_window("main") {
        let _ = win.show();
        let _ = win.set_focus();
    }
}

fn open_sites(app: &AppHandle) {
    let dir = sidecar::data_dir(app).join("sites");
    let _ = app.opener().open_path(dir.to_string_lossy().to_string(), None::<&str>);
}

fn restart(app: &AppHandle) {
    sidecar::stop();
    let _ = sidecar::start(app);
}

fn toggle_autostart(app: &AppHandle) {
    use tauri_plugin_autostart::ManagerExt;
    let autostart = app.autolaunch();
    let enabled = autostart.is_enabled().unwrap_or(false);
    if enabled {
        let _ = autostart.disable();
    } else {
        let _ = autostart.enable();
    }
}

fn quit(app: &AppHandle) {
    sidecar::stop();
    app.exit(0);
}
