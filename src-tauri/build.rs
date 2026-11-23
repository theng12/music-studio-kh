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
    // Sparkle through the shared shim, macOS only: a vendored framework and a
    // universal static library. The rpath is mandatory — without it the app
    // links and dies at launch with `Library not loaded: @rpath/Sparkle.framework/...`.
    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("macos") {
        let vendor = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("vendor/khsparkle");
        println!("cargo:rerun-if-changed={}", vendor.join("libkhsparkle.a").display());
        println!("cargo:rustc-link-search=native={}", vendor.display());
        println!("cargo:rustc-link-lib=static=khsparkle");
        println!("cargo:rustc-link-search=framework={}", vendor.display());
        println!("cargo:rustc-link-lib=framework=Sparkle");
        println!("cargo:rustc-link-lib=framework=Foundation");
        println!("cargo:rustc-link-arg=-Wl,-rpath,@executable_path/../Frameworks");
        // Test binaries live under target/, not in a bundle: point them at the vendored
        // framework. Tests only — never the shipped app, which must carry no owner path.
        println!("cargo:rustc-link-arg-tests=-Wl,-rpath,{}", vendor.display());
    }
    tauri_build::build()
}
