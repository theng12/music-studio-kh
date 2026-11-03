fn main() {
    println!(
        "cargo:rustc-env=TUNEHARBOR_TARGET={}",
        std::env::var("TARGET").unwrap()
    );
    println!("cargo:rerun-if-env-changed=MUSIC_ANDROID_HTTP_HOSTS");
    println!(
        "cargo:rustc-env=MUSIC_ANDROID_HTTP_HOSTS={}",
        std::env::var("MUSIC_ANDROID_HTTP_HOSTS").unwrap_or_else(|_| "[]".into())
    );
    tauri_build::build()
}
