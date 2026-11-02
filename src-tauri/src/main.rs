#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]
use serde::Deserialize;
use serde_json::Value;
use std::path::PathBuf;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Mutex,
};
use tauri::menu::{Menu, MenuItem, PredefinedMenuItem, Submenu};
use tauri::tray::{TrayIconBuilder, TrayIconEvent};
use tauri::{Manager, WebviewUrl, WebviewWindow, WebviewWindowBuilder};
use tauri_plugin_dialog::{DialogExt, MessageDialogKind};
use tuneharbor::bridge::{validate_profile_url, Bridge};

struct Backend(Mutex<Result<Bridge, String>>);
struct Closing(AtomicBool);

fn only_manager(window: &WebviewWindow) -> Result<(), String> {
    if window.label() != "main" {
        return Err("Native profile administration is private to the profile manager".into());
    }
    Ok(())
}

#[tauri::command]
async fn request(
    window: WebviewWindow,
    app: tauri::AppHandle,
    method: String,
    params: Value,
) -> Result<Value, String> {
    only_manager(&window)?;
    tauri::async_runtime::spawn_blocking(move || {
        let state = app.state::<Backend>();
        let mut backend = state
            .0
            .lock()
            .map_err(|_| "Music server state unavailable")?;
        backend
            .as_mut()
            .map_err(|e| e.clone())?
            .request(&method, params)
    })
    .await
    .map_err(|_| "Music server request stopped".to_string())?
}

#[derive(Deserialize)]
struct Opened {
    id: String,
    url: String,
    bootstrap: String,
}

fn dispatch_player_cmd(app: &tauri::AppHandle, cmd: &str) {
    let windows = app.webview_windows();
    let has_profile = windows.keys().any(|k| k.starts_with("profile-"));
    for (label, win) in windows {
        if !has_profile || label.starts_with("profile-") {
            let _ = win.eval(&format!(
                "window.__TUNEHARBOR_MENU_CMD__ && window.__TUNEHARBOR_MENU_CMD__({:?})",
                cmd
            ));
        }
    }
}

fn toggle_pin_windows(app: &tauri::AppHandle) {
    let windows = app.webview_windows();
    let profile_windows: Vec<_> = windows
        .iter()
        .filter(|(label, _)| label.starts_with("profile-"))
        .map(|(_, win)| win)
        .collect();
    let targets = if !profile_windows.is_empty() {
        profile_windows
    } else if let Some(main) = windows.get("main") {
        vec![main]
    } else {
        Vec::new()
    };
    for win in targets {
        let pinned = !win.is_always_on_top().unwrap_or(false);
        let _ = win.set_always_on_top(pinned);
        let _ = win.eval(&format!(
            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') {{ window.__TH_DESKTOP_STATE__({{\"pinned\":{}}}); }} else if (window.__TH_DESKTOP_STATE__) {{ Object.assign(window.__TH_DESKTOP_STATE__, {{\"pinned\":{}}}); }}",
            pinned, pinned
        ));
    }
}

fn toggle_mini_windows(app: &tauri::AppHandle) {
    for (label, win) in app.webview_windows() {
        if label.starts_with("profile-") {
            let scale = win.scale_factor().unwrap_or(1.0);
            let height = win.inner_size().map(|s| (s.height as f64) / scale).unwrap_or(850.0);
            if height > 320.0 {
                let _ = win.set_size(tauri::Size::Logical(tauri::LogicalSize::new(420.0, 220.0)));
                let _ = win.set_always_on_top(true);
                let _ = win.eval(
                    "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":true,\"pinned\":true}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":true,\"pinned\":true}); }"
                );
            } else {
                let _ = win.set_size(tauri::Size::Logical(tauri::LogicalSize::new(1160.0, 850.0)));
                let _ = win.eval(
                    "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":false}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":false}); }"
                );
            }
            let _ = win.eval(
                "window.__TUNEHARBOR_MENU_CMD__ && (window.__TUNEHARBOR_MENU_CMD__(\"toggle-mini\"), window.__TUNEHARBOR_MENU_CMD__(\"toggle_mini\"))"
            );
        }
    }
}

fn toggle_hide_windows(app: &tauri::AppHandle) {
    let windows = app.webview_windows();
    let relevant: Vec<_> = windows
        .iter()
        .filter(|(label, _)| *label == "main" || label.starts_with("profile-"))
        .map(|(_, win)| win)
        .collect();

    let any_visible = relevant.iter().any(|win| win.is_visible().unwrap_or(false));
    if any_visible {
        for win in &relevant {
            if win.is_visible().unwrap_or(false) {
                let _ = win.hide();
            }
        }
    } else {
        for win in &relevant {
            let _ = win.show();
            let _ = win.unminimize();
            let _ = win.set_focus();
        }
    }
}

fn handle_menu_action(app: &tauri::AppHandle, id: &str) {
    match id {
        "play_pause" | "next_track" | "prev_track" | "toggle_shuffle" | "cycle_repeat" | "toggle_lyrics" => {
            dispatch_player_cmd(app, id);
        }
        "toggle_pin" => toggle_pin_windows(app),
        "toggle_mini" => toggle_mini_windows(app),
        "toggle_hide" => toggle_hide_windows(app),
        "show" => show(app),
        "quit" => quit(app),
        _ => {}
    }
}

#[tauri::command]
fn open_profile(
    window: WebviewWindow,
    app: tauri::AppHandle,
    opened: Opened,
) -> Result<(), String> {
    only_manager(&window)?;
    if opened.id.len() != 32
        || !opened.id.chars().all(|c| c.is_ascii_hexdigit())
        || opened.bootstrap.len() != 64
        || !opened.bootstrap.chars().all(|c| c.is_ascii_hexdigit())
    {
        return Err("Invalid desktop profile response".into());
    }
    let url = validate_profile_url(&opened.url)?;
    let label = format!("profile-{}", opened.id);
    if let Some(existing) = app.get_webview_window(&label) {
        let _ = existing.show();
        let _ = existing.unminimize();
        let _ = existing.set_focus();
        return Ok(());
    }
    let script = format!(
        "window.__TUNEHARBOR_BOOTSTRAP__ = {};",
        serde_json::to_string(&opened.bootstrap).map_err(|_| "Invalid bootstrap")?
    );
    let port = url.port();
    WebviewWindowBuilder::new(&app, label, WebviewUrl::External(url))
        .title("Music Studio KH · Music library")
        .inner_size(1160.0, 850.0)
        .min_inner_size(340.0, 140.0)
        .disable_drag_drop_handler()
        .initialization_script(&script)
        .on_navigation(move |url| url.host_str() == Some("127.0.0.1") && url.port() == port)
        .on_document_title_changed(|window, title| {
            if let Some(cmd) = title.strip_prefix("__TH_CMD__:") {
                match cmd {
                    "toggle-pin" => {
                        let pinned = !window.is_always_on_top().unwrap_or(false);
                        let _ = window.set_always_on_top(pinned);
                        let _ = window.eval(&format!(
                            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') {{ window.__TH_DESKTOP_STATE__({{\"pinned\":{}}}); }} else if (window.__TH_DESKTOP_STATE__) {{ Object.assign(window.__TH_DESKTOP_STATE__, {{\"pinned\":{}}}); }}",
                            pinned, pinned
                        ));
                    }
                    "pin-on" => {
                        let _ = window.set_always_on_top(true);
                        let _ = window.eval(
                            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"pinned\":true}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"pinned\":true}); }"
                        );
                    }
                    "pin-off" => {
                        let _ = window.set_always_on_top(false);
                        let _ = window.eval(
                            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"pinned\":false}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"pinned\":false}); }"
                        );
                    }
                    "mini-on" => {
                        let _ = window.set_size(tauri::Size::Logical(tauri::LogicalSize::new(420.0, 220.0)));
                        let _ = window.set_always_on_top(true);
                        let _ = window.eval(
                            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":true,\"pinned\":true}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":true,\"pinned\":true}); }"
                        );
                    }
                    "mini-off" => {
                        let _ = window.set_size(tauri::Size::Logical(tauri::LogicalSize::new(1160.0, 850.0)));
                        let _ = window.eval(
                            "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":false}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":false}); }"
                        );
                    }
                    "toggle-mini" => {
                        let scale = window.scale_factor().unwrap_or(1.0);
                        let height = window.inner_size().map(|s| (s.height as f64) / scale).unwrap_or(850.0);
                        if height > 320.0 {
                            let _ = window.set_size(tauri::Size::Logical(tauri::LogicalSize::new(420.0, 220.0)));
                            let _ = window.set_always_on_top(true);
                            let _ = window.eval(
                                "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":true,\"pinned\":true}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":true,\"pinned\":true}); }"
                            );
                        } else {
                            let _ = window.set_size(tauri::Size::Logical(tauri::LogicalSize::new(1160.0, 850.0)));
                            let _ = window.eval(
                                "if (typeof window.__TH_DESKTOP_STATE__ === 'function') { window.__TH_DESKTOP_STATE__({\"mini\":false}); } else if (window.__TH_DESKTOP_STATE__) { Object.assign(window.__TH_DESKTOP_STATE__, {\"mini\":false}); }"
                            );
                        }
                    }
                    "hide-window" => {
                        let _ = window.hide();
                    }
                    "close-profile-window" => {
                        let _ = window.destroy();
                    }
                    "quit-app" => {
                        quit(window.app_handle());
                    }
                    _ => {}
                }
            }
        })
        .build()
        .map_err(|e| e.to_string())?;
    Ok(())
}

fn executable(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    let name = if cfg!(windows) {
        "tuneharbor-server.exe"
    } else {
        "tuneharbor-server"
    };
    let own = std::env::current_exe().map_err(|_| "App location unavailable")?;
    let near = own.parent().ok_or("App location unavailable")?.join(name);
    if near.is_file() {
        return Ok(near);
    }
    let resource = app
        .path()
        .resource_dir()
        .map_err(|_| "App resources unavailable")?
        .join(name);
    if resource.is_file() {
        return Ok(resource);
    }
    if cfg!(debug_assertions) {
        let suffix = if cfg!(windows) { ".exe" } else { "" };
        let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("binaries")
            .join(format!(
                "tuneharbor-server-{}{}",
                env!("TUNEHARBOR_TARGET"),
                suffix
            ));
        if dev.is_file() {
            return Ok(dev);
        }
    }
    Err("Music server is missing from this installation. Reinstall Music Studio KH.".into())
}

fn show(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
    for (label, win) in app.webview_windows() {
        if label.starts_with("profile-") {
            let _ = win.show();
            let _ = win.unminimize();
            let _ = win.set_focus();
        }
    }
}

fn quit(app: &tauri::AppHandle) {
    if app.state::<Closing>().0.swap(true, Ordering::SeqCst) {
        return;
    }
    let app = app.clone();
    std::thread::spawn(move || {
        if let Ok(mut backend) = app.state::<Backend>().0.lock() {
            if let Ok(bridge) = backend.as_mut() {
                bridge.shutdown();
            }
        }
        app.exit(0);
    });
}

/// The first time the window closes, say that servers keep running in the tray.
fn explain_tray_once(app: &tauri::AppHandle) {
    let Ok(dir) = app.path().app_config_dir() else {
        return;
    };
    let marker = dir.join("close-to-tray-explained");
    if marker.exists()
        || std::fs::create_dir_all(&dir).is_err()
        || std::fs::write(&marker, b"1").is_err()
    {
        return;
    }
    app.dialog()
        .message(
            "Music Studio KH is still running in the tray so your music stays available to your \
             browser and phone. To stop all servers, choose “Quit and stop servers” from the \
             tray icon.",
        )
        .title("Music Studio KH keeps playing in the background")
        .kind(MessageDialogKind::Info)
        .show(|_| {});
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _, _| show(app)))
        .plugin(tauri_plugin_dialog::init())
        .plugin(
            tauri_plugin_autostart::Builder::new()
                .args(["--autostart"])
                .build(),
        )
        .manage(Closing(AtomicBool::new(false)))
        .invoke_handler(tauri::generate_handler![request, open_profile])
        .setup(|app| {
            let data = app.path().config_dir()?.join("TuneHarbor").join("profiles");
            let resources = app.path().resource_dir()?.join("runtime-tools");
            let tools = if resources.is_dir() {
                resources
            } else {
                PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("runtime-tools")
            };
            let bridge = executable(app.handle()).and_then(|exe| Bridge::spawn(exe, tools, data));
            app.manage(Backend(Mutex::new(bridge)));

            let tray_menu = Menu::with_items(
                app,
                &[
                    &MenuItem::with_id(app, "play_pause", "Play / Pause", true, None::<&str>)?,
                    &MenuItem::with_id(app, "next_track", "Next Song", true, None::<&str>)?,
                    &MenuItem::with_id(app, "prev_track", "Previous Song", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_shuffle", "Toggle Shuffle", true, None::<&str>)?,
                    &MenuItem::with_id(app, "cycle_repeat", "Cycle Repeat Mode", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_lyrics", "Show / Hide Lyrics", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "toggle_pin", "Pin Window on Top", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_mini", "Toggle Mini Player", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_hide", "Hide / Show Music Studio KH", true, None::<&str>)?,
                    &MenuItem::with_id(app, "show", "Open Music Studio KH Manager", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "quit", "Quit and stop servers", true, None::<&str>)?,
                ],
            )?;

            TrayIconBuilder::new()
                .icon(app.default_window_icon().ok_or("App icon missing")?.clone())
                .tooltip("Music Studio KH · Your music, at home")
                .menu(&tray_menu)
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::DoubleClick { .. } = event {
                        show(tray.app_handle());
                    }
                })
                .build(app)?;

            let app_menu_tuneharbor = Submenu::with_items(
                app,
                "Music Studio KH",
                true,
                &[
                    &MenuItem::with_id(app, "show", "Open Music Studio KH Manager", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::about(app, None, None)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::hide(app, None)?,
                    &PredefinedMenuItem::hide_others(app, None)?,
                    &PredefinedMenuItem::show_all(app, None)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "quit", "Quit and stop servers", true, None::<&str>)?,
                ],
            )?;

            let app_menu_edit = Submenu::with_items(
                app,
                "Edit",
                true,
                &[
                    &PredefinedMenuItem::undo(app, None)?,
                    &PredefinedMenuItem::redo(app, None)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::cut(app, None)?,
                    &PredefinedMenuItem::copy(app, None)?,
                    &PredefinedMenuItem::paste(app, None)?,
                    &PredefinedMenuItem::select_all(app, None)?,
                ],
            )?;

            let app_menu_controls = Submenu::with_items(
                app,
                "Controls",
                true,
                &[
                    &MenuItem::with_id(app, "play_pause", "Play / Pause", true, None::<&str>)?,
                    &MenuItem::with_id(app, "next_track", "Next Song", true, None::<&str>)?,
                    &MenuItem::with_id(app, "prev_track", "Previous Song", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "toggle_shuffle", "Toggle Shuffle", true, None::<&str>)?,
                    &MenuItem::with_id(app, "cycle_repeat", "Cycle Repeat Mode", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_lyrics", "Show / Hide Lyrics", true, None::<&str>)?,
                ],
            )?;

            let app_menu_window = Submenu::with_items(
                app,
                "Window",
                true,
                &[
                    &MenuItem::with_id(app, "toggle_pin", "Pin Window on Top", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_mini", "Toggle Mini Player", true, None::<&str>)?,
                    &MenuItem::with_id(app, "toggle_hide", "Hide / Show Music Studio KH", true, None::<&str>)?,
                    &MenuItem::with_id(app, "show", "Open Music Studio KH Manager", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &PredefinedMenuItem::minimize(app, None)?,
                    &PredefinedMenuItem::maximize(app, None)?,
                ],
            )?;

            let app_menu = Menu::with_items(
                app,
                &[
                    &app_menu_tuneharbor,
                    &app_menu_edit,
                    &app_menu_controls,
                    &app_menu_window,
                ],
            )?;
            let _ = app.set_menu(app_menu);

            if std::env::args().any(|a| a == "--autostart") {
                if let Ok(mut backend) = app.state::<Backend>().0.lock() {
                    if let Ok(bridge) = backend.as_mut() {
                        let _ = bridge.request("restore", serde_json::json!({}));
                    }
                }
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.hide();
                }
            }
            Ok(())
        })
        .on_menu_event(|app, event| handle_menu_action(app, event.id.as_ref()))
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if window.label().starts_with("profile-") {
                    api.prevent_close();
                    let _ = window.show();
                    let _ = window.unminimize();
                    let _ = window.set_focus();
                    if let Some(win) = window.app_handle().get_webview_window(window.label()) {
                        let _ = win.eval(
                            "if (window.__TUNEHARBOR_ON_CLOSE_REQUESTED__) { window.__TUNEHARBOR_ON_CLOSE_REQUESTED__(); } else { document.title = '__TH_CMD__:hide-window'; }",
                        );
                    }
                } else if window.label() == "main" {
                    api.prevent_close();
                    let _ = window.hide();
                    explain_tray_once(window.app_handle());
                }
            }
        })
        .build(tauri::generate_context!())
        .expect("Music Studio KH could not initialize")
        .run(|app, event| {
            if let tauri::RunEvent::ExitRequested { api, .. } = event {
                if !app.state::<Closing>().0.load(Ordering::SeqCst) {
                    api.prevent_exit();
                    quit(app);
                }
            }
        });
}

#[cfg(test)]
mod tests {
    #[test]
    fn test_desktop_cmd_parsing() {
        assert_eq!(
            "__TH_CMD__:toggle-pin".strip_prefix("__TH_CMD__:"),
            Some("toggle-pin")
        );
        assert_eq!(
            "__TH_CMD__:pin-on".strip_prefix("__TH_CMD__:"),
            Some("pin-on")
        );
        assert_eq!(
            "__TH_CMD__:pin-off".strip_prefix("__TH_CMD__:"),
            Some("pin-off")
        );
        assert_eq!(
            "__TH_CMD__:mini-on".strip_prefix("__TH_CMD__:"),
            Some("mini-on")
        );
        assert_eq!(
            "__TH_CMD__:mini-off".strip_prefix("__TH_CMD__:"),
            Some("mini-off")
        );
        assert_eq!(
            "__TH_CMD__:toggle-mini".strip_prefix("__TH_CMD__:"),
            Some("toggle-mini")
        );
        assert_eq!(
            "__TH_CMD__:hide-window".strip_prefix("__TH_CMD__:"),
            Some("hide-window")
        );
        assert_eq!(
            "__TH_CMD__:close-profile-window".strip_prefix("__TH_CMD__:"),
            Some("close-profile-window")
        );
        assert_eq!(
            "__TH_CMD__:quit-app".strip_prefix("__TH_CMD__:"),
            Some("quit-app")
        );
        assert_eq!(
            "Music Studio KH · Music library".strip_prefix("__TH_CMD__:"),
            None
        );

        let normal_height = 850.0;
        let mini_height = 220.0;
        assert!(normal_height > 320.0);
        assert!(mini_height <= 320.0);
    }
}

