fn main() {
    println!(
        "cargo:rustc-env=TUNEHARBOR_TARGET={}",
        std::env::var("TARGET").unwrap()
    );
    tauri_build::build()
}
