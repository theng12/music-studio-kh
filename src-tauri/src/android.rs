use crate::android_url::validate_server;
use tauri::{Manager, WebviewUrl, WebviewWindow, WebviewWindowBuilder};
use url::Url;

fn http_hosts() -> Vec<String> {
    serde_json::from_str(env!("MUSIC_ANDROID_HTTP_HOSTS")).expect("build-time HTTP hosts")
}

fn is_local(url: &Url) -> bool {
    url.scheme() == "http" && url.host_str() == Some("tauri.localhost") && url.port().is_none()
}

#[tauri::command]
fn client_config(window: WebviewWindow) -> Result<serde_json::Value, String> {
    if !is_local(&window.url().map_err(|e| e.to_string())?) {
        return Err("Client setup is only available on the connection screen".into());
    }
    Ok(serde_json::json!({ "httpHosts": http_hosts() }))
}

#[tauri::command]
fn connect(window: WebviewWindow, server: String) -> Result<String, String> {
    if !is_local(&window.url().map_err(|e| e.to_string())?) {
        return Err("Return to the connection screen to choose a server".into());
    }
    let url = validate_server(&server, &http_hosts())?;
    // The navigation handler permits this origin only after a local user chose it.
    window
        .state::<Connection>()
        .0
        .lock()
        .map_err(|_| "Connection state unavailable")?
        .replace(url.origin().ascii_serialization());
    Ok(url.to_string())
}

struct Connection(std::sync::Mutex<Option<String>>);

#[tauri::mobile_entry_point]
pub fn run() {
    tauri::Builder::default()
        .manage(Connection(std::sync::Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![client_config, connect])
        .setup(|app| {
            let handle = app.handle().clone();
            WebviewWindowBuilder::new(app, "main", WebviewUrl::App("android.html".into()))
                .title("Music Studio KH")
                .initialization_script(format!(
                    "window.__MUSIC_ANDROID_VERSION__ = {:?};\n{}",
                    env!("CARGO_PKG_VERSION"),
                    include_str!("android-navigation.js")
                ))
                .on_navigation(move |url| {
                    is_local(url)
                        || handle
                            .state::<Connection>()
                            .0
                            .lock()
                            .ok()
                            .and_then(|origin| origin.clone())
                            .is_some_and(|origin| url.origin().ascii_serialization() == origin)
                })
                .build()?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("Android client failed to start");
}
