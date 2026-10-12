#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]
use serde::Deserialize;
use serde_json::Value;
use std::path::PathBuf;
use std::sync::{atomic::{AtomicBool, Ordering}, Mutex};
use tauri::{Manager, WebviewUrl, WebviewWindow, WebviewWindowBuilder};
use tauri::menu::{Menu, MenuItem};
use tauri::tray::{TrayIconBuilder, TrayIconEvent};
use tuneharbor::bridge::{Bridge, validate_profile_url};
struct Backend(Mutex<Result<Bridge, String>>);
struct Closing(AtomicBool);
fn only_manager(window: &WebviewWindow) -> Result<(), String> { if window.label() != "main" { return Err("Native profile administration is private to the profile manager".into()); } Ok(()) }
#[tauri::command]
async fn request(window: WebviewWindow, app: tauri::AppHandle, method: String, params: Value) -> Result<Value, String> {
    only_manager(&window)?;
    tauri::async_runtime::spawn_blocking(move || { let state = app.state::<Backend>(); let mut backend = state.0.lock().map_err(|_| "Music server state unavailable")?; backend.as_mut().map_err(|e| e.clone())?.request(&method, params) }).await.map_err(|_| "Music server request stopped".to_string())?
}
#[derive(Deserialize)] struct Opened { id: String, url: String, bootstrap: String }
#[tauri::command]
fn open_profile(window: WebviewWindow, app: tauri::AppHandle, opened: Opened) -> Result<(), String> {
    only_manager(&window)?;
    if opened.id.len() != 32 || !opened.id.chars().all(|c| c.is_ascii_hexdigit()) || opened.bootstrap.len() != 64 || !opened.bootstrap.chars().all(|c| c.is_ascii_hexdigit()) { return Err("Invalid desktop profile response".into()); }
    let url = validate_profile_url(&opened.url)?;
    let label = format!("profile-{}", opened.id);
    if let Some(existing) = app.get_webview_window(&label) { existing.close().map_err(|e| e.to_string())?; }
    let script = format!("window.__TUNEHARBOR_BOOTSTRAP__ = {};", serde_json::to_string(&opened.bootstrap).map_err(|_| "Invalid bootstrap")?);
    let port = url.port();
    WebviewWindowBuilder::new(&app, label, WebviewUrl::External(url)).title("TuneHarbor · Music library").inner_size(1160.0, 850.0).min_inner_size(380.0, 600.0).initialization_script(&script).on_navigation(move |url| url.host_str() == Some("127.0.0.1") && url.port() == port).build().map_err(|e| e.to_string())?;
    Ok(())
}
fn executable(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    let name = if cfg!(windows) { "tuneharbor-server.exe" } else { "tuneharbor-server" };
    let own = std::env::current_exe().map_err(|_| "App location unavailable")?;
    let near = own.parent().ok_or("App location unavailable")?.join(name);
    if near.is_file() { return Ok(near); }
    let resource = app.path().resource_dir().map_err(|_| "App resources unavailable")?.join(name);
    if resource.is_file() { return Ok(resource); }
    if cfg!(debug_assertions) { let suffix = if cfg!(windows) { ".exe" } else { "" }; let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("binaries").join(format!("tuneharbor-server-{}{}", env!("TUNEHARBOR_TARGET"), suffix)); if dev.is_file() { return Ok(dev); } }
    Err("Music server is missing from this installation. Reinstall TuneHarbor.".into())
}
fn show(app: &tauri::AppHandle) { if let Some(window) = app.get_webview_window("main") { let _ = window.show(); let _ = window.set_focus(); } }
fn quit(app: &tauri::AppHandle) {
    if app.state::<Closing>().0.swap(true, Ordering::SeqCst) { return; }
    let app = app.clone();
    std::thread::spawn(move || { if let Ok(mut backend) = app.state::<Backend>().0.lock() { if let Ok(bridge) = backend.as_mut() { bridge.shutdown(); } } app.exit(0); });
}
fn main() {
    tauri::Builder::default().plugin(tauri_plugin_single_instance::init(|app, _, _| show(app))).plugin(tauri_plugin_dialog::init()).plugin(tauri_plugin_autostart::Builder::new().args(["--autostart"]).build()).manage(Closing(AtomicBool::new(false))).invoke_handler(tauri::generate_handler![request, open_profile]).setup(|app| {
        let data = app.path().config_dir()?.join("TuneHarbor").join("profiles");
        let resources = app.path().resource_dir()?.join("runtime-tools");
        let tools = if resources.is_dir() { resources } else { PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("runtime-tools") };
        let bridge = executable(app.handle()).and_then(|exe| Bridge::spawn(exe, tools, data));
        app.manage(Backend(Mutex::new(bridge)));
        let show_item = MenuItem::with_id(app, "show", "Open TuneHarbor", true, None::<&str>)?;
        let quit_item = MenuItem::with_id(app, "quit", "Quit and stop servers", true, None::<&str>)?;
        let menu = Menu::with_items(app, &[&show_item, &quit_item])?;
        TrayIconBuilder::new().icon(app.default_window_icon().ok_or("App icon missing")?.clone()).tooltip("TuneHarbor · Your music, at home").menu(&menu).on_menu_event(|app, event| match event.id.as_ref() { "show" => show(app), "quit" => quit(app), _ => {} }).on_tray_icon_event(|tray,event| if let TrayIconEvent::DoubleClick {..}=event {show(tray.app_handle())}).build(app)?;
        if std::env::args().any(|a| a == "--autostart") { if let Ok(mut backend) = app.state::<Backend>().0.lock() { if let Ok(bridge) = backend.as_mut() { let _ = bridge.request("restore", serde_json::json!({})); } } if let Some(window) = app.get_webview_window("main") { let _ = window.hide(); } }
        Ok(())
    }).on_window_event(|window, event| { if window.label() == "main" { if let tauri::WindowEvent::CloseRequested { api, .. } = event { api.prevent_close(); let _ = window.hide(); } } }).build(tauri::generate_context!()).expect("TuneHarbor could not initialize").run(|app,event| { if let tauri::RunEvent::ExitRequested { api, .. } = event { if !app.state::<Closing>().0.load(Ordering::SeqCst) { api.prevent_exit(); quit(app); } } });
}
