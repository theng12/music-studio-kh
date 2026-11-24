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
use tauri_plugin_autostart::ManagerExt;
use tauri_plugin_dialog::{DialogExt, MessageDialogKind};
use music_studio_kh::bridge::{validate_profile_url, Bridge};

/// Sparkle through the shared shim: one C call, one state flag, no Tauri types in between.
#[cfg(target_os = "macos")]
mod sparkle {
    use std::net::{SocketAddr, TcpStream};
    use std::sync::atomic::{AtomicU8, Ordering};
    use std::time::{Duration, Instant};
    use tauri::Manager;

    extern "C" {
        fn kh_sparkle_start() -> i32;
        fn kh_sparkle_last_error() -> *const std::ffi::c_char;
        fn kh_sparkle_set_state_callback(cb: extern "C" fn(i32));
    }

    const HOLD_ADDR: ([u8; 4], u16) = ([127, 0, 0, 1], 8645);
    const HOLD_WAIT_TIMEOUT: Duration = Duration::from_secs(5);
    const HOLD_WAIT_POLL: Duration = Duration::from_millis(50);

    static STATE: AtomicU8 = AtomicU8::new(0);

    extern "C" fn on_state(state: i32) {
        STATE.store(state.clamp(0, 2) as u8, Ordering::SeqCst);
    }

    /// True while Sparkle is installing or relaunching: the app must not refuse its own quit then.
    pub fn updating() -> bool {
        STATE.load(Ordering::SeqCst) >= 1
    }

    /// Wait until the Go helper's hold server accepts a TCP connection (bounded), avoiding
    /// a cold-start ECONNREFUSED race between Bridge::spawn() and kh_sparkle_start().
    pub fn wait_for_hold_port(addr: SocketAddr, timeout: Duration, poll: Duration) -> bool {
        let deadline = Instant::now() + timeout;
        loop {
            if TcpStream::connect_timeout(&addr, Duration::from_millis(200)).is_ok() {
                return true;
            }
            if Instant::now() >= deadline {
                return false;
            }
            std::thread::sleep(poll);
        }
    }

    /// Called once on the main thread, after the hold server is listening. A refusal lands in the log with its reason.
    pub fn start() {
        unsafe {
            kh_sparkle_set_state_callback(on_state);
            let rc = kh_sparkle_start();
            if rc != 0 {
                let reason = std::ffi::CStr::from_ptr(kh_sparkle_last_error()).to_string_lossy();
                eprintln!("kh-sparkle: refused to start ({rc}): {reason}");
            }
        }
    }

    /// Wait off the main thread for :8645 to bind, then dispatch start() on the main thread.
    pub fn start_when_ready(app: tauri::AppHandle) {
        std::thread::spawn(move || {
            let addr = SocketAddr::from(HOLD_ADDR);
            if !wait_for_hold_port(addr, HOLD_WAIT_TIMEOUT, HOLD_WAIT_POLL) {
                eprintln!(
                    "kh-sparkle: hold server at {addr} not ready after {}s; attempting start anyway",
                    HOLD_WAIT_TIMEOUT.as_secs()
                );
            }
            if app.state::<super::Closing>().0.load(Ordering::SeqCst) {
                return;
            }
            let _ = app.run_on_main_thread(start);
        });
    }
}

#[cfg(not(target_os = "macos"))]
mod sparkle {
    pub fn updating() -> bool {
        false
    }
    pub fn start_when_ready(_app: tauri::AppHandle) {}
}

struct Backend(Mutex<Result<Bridge, String>>);
struct Closing(AtomicBool);

const AUTOSTART_MOVE_ERR: &str =
    "Move Music Studio KH to Applications before turning on start at login.";

fn path_is_inside(candidate: &std::path::Path, dir: &std::path::Path) -> bool {
    candidate != dir && candidate.starts_with(dir)
}

fn is_in_applications_dir(exe: &std::path::Path, home: Option<&std::path::Path>) -> bool {
    let system_apps = std::path::Path::new("/Applications");
    if path_is_inside(exe, system_apps) {
        return true;
    }
    if let Ok(canon) = std::fs::canonicalize(system_apps) {
        if path_is_inside(exe, &canon) {
            return true;
        }
    }
    if let Some(home_dir) = home {
        let user_apps = home_dir.join("Applications");
        if path_is_inside(exe, &user_apps) {
            return true;
        }
        if let Ok(canon_home) = std::fs::canonicalize(home_dir) {
            if path_is_inside(exe, &canon_home.join("Applications")) {
                return true;
            }
        }
        if let Ok(canon_user_apps) = std::fs::canonicalize(&user_apps) {
            if path_is_inside(exe, &canon_user_apps) {
                return true;
            }
        }
    }
    false
}

fn unescape_xml_entities(raw: &str) -> String {
    raw.replace("&amp;", "&")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", "\"")
        .replace("&apos;", "'")
}

fn parse_plist_xml_program_arg0(xml: &str) -> Option<String> {
    let key_idx = xml.find("<key>ProgramArguments</key>")?;
    let after_key = &xml[key_idx + "<key>ProgramArguments</key>".len()..];
    let arr_start = after_key.find("<array>")?;
    let after_arr = &after_key[arr_start + "<array>".len()..];
    let arr_end = after_arr.find("</array>")?;
    let array_body = &after_arr[..arr_end];
    let str_start = array_body.find("<string>")?;
    let after_str = &array_body[str_start + "<string>".len()..];
    let str_end = after_str.find("</string>")?;
    let value = unescape_xml_entities(after_str[..str_end].trim());
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn read_plist_program_arg0(plist_path: &std::path::Path) -> Option<String> {
    if let Ok(content) = std::fs::read_to_string(plist_path) {
        if let Some(arg0) = parse_plist_xml_program_arg0(&content) {
            return Some(arg0);
        }
    }
    #[cfg(target_os = "macos")]
    {
        let out = std::process::Command::new("/usr/libexec/PlistBuddy")
            .args(["-c", "Print :ProgramArguments:0"])
            .arg(plist_path)
            .output()
            .ok()?;
        if out.status.success() {
            let value = String::from_utf8_lossy(&out.stdout).trim().to_string();
            if !value.is_empty() {
                return Some(value);
            }
        }
    }
    None
}

fn should_heal_launch_agent(
    exe: &std::path::Path,
    home: Option<&std::path::Path>,
    plist_path: &std::path::Path,
) -> bool {
    if !is_in_applications_dir(exe, home) || !plist_path.is_file() {
        return false;
    }
    let Some(arg0) = read_plist_program_arg0(plist_path) else {
        return false;
    };
    let arg0_path = std::path::Path::new(&arg0);
    if arg0_path == exe {
        return false;
    }
    if let Ok(canon_arg0) = std::fs::canonicalize(arg0_path) {
        if canon_arg0 == exe {
            return false;
        }
    }
    true
}

#[cfg(target_os = "macos")]
fn current_exe_canonical() -> Result<PathBuf, String> {
    let exe = std::env::current_exe().map_err(|_| AUTOSTART_MOVE_ERR.to_string())?;
    Ok(std::fs::canonicalize(&exe).unwrap_or(exe))
}

#[cfg(target_os = "macos")]
fn heal_launch_agent_if_stale(app: &tauri::AppHandle) {
    let Ok(exe) = current_exe_canonical() else {
        return;
    };
    let Some(home) = std::env::var_os("HOME").map(PathBuf::from) else {
        return;
    };
    let plist_path = home
        .join("Library")
        .join("LaunchAgents")
        .join("Music Studio KH.plist");
    if should_heal_launch_agent(&exe, Some(&home), &plist_path) {
        eprintln!(
            "autostart: healing stale LaunchAgent plist to {}",
            exe.display()
        );
        if let Err(e) = app.autolaunch().enable() {
            eprintln!("autostart: failed to heal LaunchAgent plist: {e}");
        }
    }
}

#[tauri::command]
fn set_autostart(
    window: WebviewWindow,
    app: tauri::AppHandle,
    enabled: bool,
) -> Result<(), String> {
    only_manager(&window)?;
    if enabled {
        #[cfg(target_os = "macos")]
        {
            let exe = current_exe_canonical()?;
            let home = std::env::var_os("HOME").map(PathBuf::from);
            if !is_in_applications_dir(&exe, home.as_deref()) {
                return Err(AUTOSTART_MOVE_ERR.to_string());
            }
        }
        app.autolaunch().enable().map_err(|e| e.to_string())
    } else {
        app.autolaunch().disable().map_err(|e| e.to_string())
    }
}

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
    #[serde(default)]
    name: String,
}

// Profile windows join the manager window as macOS tabs instead of adding windows.
const TAB_GROUP: &str = "music-studio-kh";

#[cfg(target_os = "macos")]
fn prefer_tabs(window: &WebviewWindow) {
    let handle = window.clone();
    let window = window.clone();
    // AppKit window state is main-thread only; command handlers need not be.
    let _ = handle.run_on_main_thread(move || {
        use objc2_app_kit::{NSWindow, NSWindowTabbingMode};
        if let Ok(pointer) = window.ns_window() {
            // The system tab preference alone leaves windows unmerged, so ask for tabs explicitly.
            let ns_window = unsafe { &*(pointer as *const NSWindow) };
            ns_window.setTabbingMode(NSWindowTabbingMode::Preferred);
        }
    });
}

#[cfg(target_os = "macos")]
fn merge_into_tabs(app: &tauri::AppHandle, profile: &WebviewWindow) {
    let Some(main) = app.get_webview_window("main") else {
        return;
    };
    let handle = app.clone();
    let profile = profile.clone();
    let main = main.clone();
    let _ = handle.run_on_main_thread(move || {
        use objc2_app_kit::{NSWindow, NSWindowOrderingMode, NSWindowTabbingMode};
        let (Ok(profile_ptr), Ok(main_ptr)) = (profile.ns_window(), main.ns_window()) else {
            return;
        };
        let profile_window = unsafe { &*(profile_ptr as *const NSWindow) };
        let main_window = unsafe { &*(main_ptr as *const NSWindow) };
        // Automatic tabbing only merges at creation time, so group them explicitly.
        if profile_window.tabbingIdentifier().to_string()
            == main_window.tabbingIdentifier().to_string()
        {
            profile_window.setTabbingMode(NSWindowTabbingMode::Preferred);
            main_window.setTabbingMode(NSWindowTabbingMode::Preferred);
            main_window.addTabbedWindow_ordered(profile_window, NSWindowOrderingMode::Above);
            // Adding a tab does not select it, so the new window would stay hidden.
            profile_window.makeKeyAndOrderFront(None);
            if !profile_window.isVisible() {
                // Tabbing refused: keep the player usable as its own window.
                profile_window.orderFront(None);
            }
        }
    });
}

#[cfg(not(target_os = "macos"))]
fn prefer_tabs(_: &WebviewWindow) {}

#[cfg(not(target_os = "macos"))]
fn merge_into_tabs(_: &tauri::AppHandle, _: &WebviewWindow) {}

#[tauri::command]
fn open_support_page() -> Result<(), String> {
    // One allow-listed destination: the shared donation page. Nothing else opens externally.
    const SUPPORT_URL: &str = "https://nowpayments.io/donation/studiokh";
    #[cfg(target_os = "macos")]
    let mut command = std::process::Command::new("open");
    #[cfg(target_os = "windows")]
    let mut command = {
        let mut command = std::process::Command::new("cmd");
        command.args(["/C", "start", ""]);
        command
    };
    #[cfg(all(unix, not(target_os = "macos")))]
    let mut command = std::process::Command::new("xdg-open");
    command
        .arg(SUPPORT_URL)
        .spawn()
        .map(|_| ())
        .map_err(|_| "Could not open the support page".to_string())
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
        #[cfg(target_os = "macos")]
        let _ = existing.show();
        let _ = existing.unminimize();
        let _ = existing.set_focus();
        if existing.is_visible().unwrap_or(false) {
            // The profile server may have restarted (new folder, port), dropping the old sign-in.
            let _ = existing.eval(&format!(
                "window.__TUNEHARBOR_BOOTSTRAP__ = {}; window.dispatchEvent(new Event('desktop-bootstrap'));",
                serde_json::to_string(&opened.bootstrap).map_err(|_| "Invalid bootstrap")?
            ));
            return Ok(());
        }
        // A window left hidden by an earlier attempt cannot be shown again; replace it.
        let _ = existing.destroy();
    }
    let script = format!(
        "window.__TUNEHARBOR_DESKTOP__ = true; window.__TUNEHARBOR_BOOTSTRAP__ = {};",
        serde_json::to_string(&opened.bootstrap).map_err(|_| "Invalid bootstrap")?
    );
    let port = url.port();
    let title = if opened.name.is_empty() {
        "Music".to_string()
    } else {
        opened.name
    };
    let profile_window = WebviewWindowBuilder::new(&app, label, WebviewUrl::External(url))
        .title(title)
        .tabbing_identifier(TAB_GROUP)
        .inner_size(1160.0, 850.0)
        .min_inner_size(340.0, 140.0)
        .disable_drag_drop_handler()
        .initialization_script(&script)
        .on_navigation(move |url| url.host_str() == Some("127.0.0.1") && url.port() == port)
        .on_document_title_changed(|window, title| {
            if let Some(cmd) = title.strip_prefix("__TH_CMD__:") {
                match cmd {
                    "toggle-fullscreen" => {
                        let fullscreen = !window.is_fullscreen().unwrap_or(false);
                        if window.set_fullscreen(fullscreen).is_ok() {
                            let _ = window.eval(&format!(
                                "window.dispatchEvent(new CustomEvent('player-fullscreen', {{detail: {}}}));",
                                fullscreen
                            ));
                        }
                    }
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
                    "manage-profile" | "relocate-profile" | "add-library-root" => {
                        if let Some(main) = window.app_handle().get_webview_window("main") {
                            let _ = main.show();
                            let _ = main.unminimize();
                            let _ = main.set_focus();
                            let id = window.label().strip_prefix("profile-").unwrap_or("");
                            let _ = main.eval(&format!(
                                "window.dispatchEvent(new CustomEvent({:?}, {{detail: {:?}}}));", cmd, id
                            ));
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
    prefer_tabs(&profile_window);
    merge_into_tabs(&app, &profile_window);
    Ok(())
}

fn executable(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    let own = std::env::current_exe().map_err(|_| "App location unavailable")?;
    // Signed Mac releases ship the server as a helper app so it shows under the app's own name.
    #[cfg(target_os = "macos")]
    if let Some(helper) = own.parent().and_then(|macos| macos.parent()).map(|contents| {
        contents.join("Helpers/Music Studio KH Helper.app/Contents/MacOS/Music Studio KH Helper")
    }) {
        if helper.is_file() {
            return Ok(helper);
        }
    }
    let name = if cfg!(windows) {
        "music-studio-kh-server.exe"
    } else {
        "music-studio-kh-server"
    };
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
    // cfg, not cfg!: a release binary must not even carry the build machine's path.
    #[cfg(debug_assertions)]
    {
        let suffix = if cfg!(windows) { ".exe" } else { "" };
        let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("binaries")
            .join(format!(
                "music-studio-kh-server-{}{}",
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
    match app.get_webview_window("main") {
        Some(window) => {
            let _ = window.show();
            let _ = window.unminimize();
            let _ = window.set_focus();
        }
        None => {
            // Reopening must never no-op: rebuild the main window if it is gone.
            if let Some(config) = app.config().app.windows.first() {
                if let Ok(builder) = tauri::WebviewWindowBuilder::from_config(app, config) {
                    if let Ok(window) = builder.build() {
                        let _ = window.show();
                        let _ = window.set_focus();
                    }
                }
            }
        }
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
             browser and phone. To stop all servers, choose “Quit Music Studio KH” from the \
             tray icon or press ⌘Q.",
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
        .invoke_handler(tauri::generate_handler![
            request,
            open_profile,
            open_support_page,
            set_autostart
        ])
        .setup(|app| {
            let data = app.path().config_dir()?.join("MusicStudioKH").join("profiles");
            let resources = app.path().resource_dir()?.join("runtime-tools");
            // Signed Mac releases package runtime executables as Tauri sidecars.
            let mac_tools = std::env::current_exe()?.parent().map(PathBuf::from);
            let tools = if cfg!(target_os = "macos")
                && mac_tools.as_ref().is_some_and(|dir| {
                    ["ffmpeg", "ffprobe", "yt-dlp", "deno"]
                        .iter()
                        .all(|tool| dir.join(tool).is_file())
                })
            {
                mac_tools.ok_or("Mac runtime directory missing")?
            } else if resources.is_dir() {
                resources
            } else {
                // cfg, not cfg!: a release binary must not even carry the build machine's path.
                #[cfg(debug_assertions)]
                let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("runtime-tools");
                #[cfg(not(debug_assertions))]
                let dev = resources;
                dev
            };
            let bridge = executable(app.handle()).and_then(|exe| Bridge::spawn(exe, tools, data));
            app.manage(Backend(Mutex::new(bridge)));
            if let Some(window) = app.get_webview_window("main") {
                prefer_tabs(&window);
            }
            // Test hook: start closed to the tray so reopen can be exercised without
            // synthetic window closes, which macOS refuses.
            if std::env::args().any(|a| a == "--hide-window") {
                let handle = app.handle().clone();
                std::thread::spawn(move || {
                    std::thread::sleep(std::time::Duration::from_millis(1500));
                    if let Some(window) = handle.get_webview_window("main") {
                        let _ = window.hide();
                    }
                });
            }

            // The tray is a quick menu only; the app window carries everything else.
            let tray_menu = Menu::with_items(
                app,
                &[
                    &MenuItem::with_id(app, "show", "Open Music Studio KH", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "play_pause", "Play / Pause", true, None::<&str>)?,
                    &MenuItem::with_id(app, "next_track", "Next", true, None::<&str>)?,
                    &MenuItem::with_id(app, "prev_track", "Previous", true, None::<&str>)?,
                    &PredefinedMenuItem::separator(app)?,
                    &MenuItem::with_id(app, "quit", "Quit Music Studio KH", true, None::<&str>)?,
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

            let app_menu_music_studio_kh = Submenu::with_items(
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
                    &MenuItem::with_id(app, "quit", "Quit Music Studio KH", true, Some("CmdOrCtrl+Q"))?,
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
                    &app_menu_music_studio_kh,
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
            #[cfg(target_os = "macos")]
            heal_launch_agent_if_stale(app.handle());

            // Started last, once the tray and window exist; wait off the main thread for
            // the helper's hold listener (:8645) before invoking kh_sparkle_start().
            sparkle::start_when_ready(app.handle().clone());
            Ok(())
        })
        .on_menu_event(|app, event| handle_menu_action(app, event.id.as_ref()))
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::Resized(_) = event {
                if let Some(win) = window.app_handle().get_webview_window(window.label()) {
                    let fullscreen = win.is_fullscreen().unwrap_or(false);
                    let _ = win.eval(&format!(
                        "window.dispatchEvent(new CustomEvent('player-fullscreen', {{detail: {}}}));",
                        fullscreen
                    ));
                }
            }
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
        .run(|app, event| match event {
            // Clicking the app icon while it is hidden is an activation, not a new
            // process, so the window has to be brought back here.
            tauri::RunEvent::Reopen { .. } => {
                show(app);
                // Activation is dropped while the window is still becoming visible.
                let handle = app.clone();
                std::thread::spawn(move || {
                    std::thread::sleep(std::time::Duration::from_millis(150));
                    if let Some(window) = handle.get_webview_window("main") {
                        let _ = window.show();
                        let _ = window.unminimize();
                        let _ = window.set_focus();
                    }
                });
            }
            tauri::RunEvent::ExitRequested { api, .. } => {
                // While Sparkle installs, it terminates the app itself: never refuse that.
                if !sparkle::updating() && !app.state::<Closing>().0.load(Ordering::SeqCst) {
                    api.prevent_exit();
                    quit(app);
                }
            }
            _ => {}
        });
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

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

    #[test]
    fn autostart_allows_applications_dirs_and_refuses_others() {
        let home = Path::new("/Users/tester");
        // Allowed: /Applications and $HOME/Applications
        assert!(is_in_applications_dir(
            Path::new("/Applications/Music Studio KH.app/Contents/MacOS/Music Studio KH"),
            Some(home)
        ));
        assert!(is_in_applications_dir(
            Path::new("/Users/tester/Applications/Music Studio KH.app/Contents/MacOS/Music Studio KH"),
            Some(home)
        ));
        // Refused: build, artifacts, target, Volumes (mounted DMG), temp, sibling prefix, bare dir
        for bad in [
            "/Users/tester/dev/music-studio-kh/app/artifacts/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/Users/tester/dev/music-studio-kh/app/src-tauri/target/release/bundle/macos/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/Users/tester/dev/music-studio-kh/app/build/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/Volumes/Music Studio KH/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/tmp/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/Applications-staging/Music Studio KH.app/Contents/MacOS/Music Studio KH",
            "/Applications",
            "/Users/tester/Applications",
        ] {
            assert!(
                !is_in_applications_dir(Path::new(bad), Some(home)),
                "expected {bad} to be refused"
            );
        }
    }

    #[test]
    fn stale_launch_agent_heal_predicate_handles_absent_matching_and_stale() {
        let tmp = std::env::temp_dir().join(format!(
            "music-studio-kh-autostart-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&tmp);
        std::fs::create_dir_all(&tmp).unwrap();

        let home = tmp.join("home");
        let app_exe = Path::new("/Applications/Music Studio KH.app/Contents/MacOS/Music Studio KH");
        let dev_exe = Path::new(
            "/Users/tester/app/artifacts/Music Studio KH.app/Contents/MacOS/Music Studio KH",
        );
        let plist_path = tmp.join("Music Studio KH.plist");

        // 1. Absent plist -> never create one
        assert!(!should_heal_launch_agent(app_exe, Some(&home), &plist_path));

        // 2. Existing plist matching current exe -> no-op
        let matching_xml = format!(
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
             <plist version=\"1.0\"><dict>\n\
             <key>Label</key><string>Music Studio KH</string>\n\
             <key>ProgramArguments</key><array>\n\
             <string>{}</string>\n\
             <string>--autostart</string>\n\
             </array></dict></plist>\n",
            app_exe.display()
        );
        std::fs::write(&plist_path, &matching_xml).unwrap();
        assert!(!should_heal_launch_agent(app_exe, Some(&home), &plist_path));

        // 3. Existing plist pointing at stale artifacts path, running from /Applications -> heal
        let stale_xml = format!(
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
             <plist version=\"1.0\"><dict>\n\
             <key>Label</key><string>Music Studio KH</string>\n\
             <key>ProgramArguments</key><array>\n\
             <string>{}</string>\n\
             <string>--autostart</string>\n\
             </array></dict></plist>\n",
            dev_exe.display()
        );
        std::fs::write(&plist_path, &stale_xml).unwrap();
        assert!(should_heal_launch_agent(app_exe, Some(&home), &plist_path));

        // 4. Running from a non-Applications path -> never heal
        assert!(!should_heal_launch_agent(dev_exe, Some(&home), &plist_path));

        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[cfg(target_os = "macos")]
    #[test]
    fn sparkle_wait_for_hold_port_waits_for_delayed_listener_and_times_out() {
        // Reserve an ephemeral port, drop it, then bind it after a delay on a background thread.
        let probe = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = probe.local_addr().unwrap();
        drop(probe);

        // Unbound port times out cleanly.
        assert!(!sparkle::wait_for_hold_port(
            addr,
            std::time::Duration::from_millis(120),
            std::time::Duration::from_millis(20),
        ));

        // Delayed bind succeeds once the listener comes up.
        let handle = std::thread::spawn(move || {
            std::thread::sleep(std::time::Duration::from_millis(100));
            let listener = std::net::TcpListener::bind(addr).unwrap();
            let _ = listener.accept();
        });
        assert!(sparkle::wait_for_hold_port(
            addr,
            std::time::Duration::from_secs(2),
            std::time::Duration::from_millis(25),
        ));
        let _ = handle.join();
    }
}

