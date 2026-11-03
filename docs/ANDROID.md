# Android streaming client

The Android client uses the existing Tauri + React app. Its bundled connection screen opens the Mac's normal authenticated React library. Music, library management, downloads and conversion stay on the Mac. The desktop supervisor and desktop-only plugins are excluded from the Android library.

## Build locally

Install the free Tauri Android prerequisites: Java, Android SDK/NDK and Rust Android targets. Set `JAVA_HOME`, `ANDROID_HOME`, and `NDK_HOME` as described in [Tauri prerequisites](https://v2.tauri.app/start/prerequisites/). No account or paid builder is required.

Name every exact HTTP host this APK may use. These are hosts only, without scheme, path or port. For example, to test against the host Mac from the standard Android emulator:

```bash
npm run build:android -- --host 10.0.2.2
```

Repeat `--host` for the Mac's actual LAN address, tailnet address or hostname. The default targets are ARM64 and ARMv7; `--target aarch64` selects just ARM64. The build generates a debug APK and a JSON build summary in `~/Desktop/All YouTube Stuff/Music Studio KH/Android/`.

The generated Android project remains under ignored `src-tauri/gen/android/`. The build script reapplies the tracked activity template, optional touchscreen/TV launcher declarations and exact-host network configuration every time. Do not ship an APK produced by calling the raw Tauri build command without these preparations.

## Connection and security

The APK shows its permitted HTTP hosts on the connection screen. Android's network security configuration denies cleartext by default, allows only these exact hosts plus Tauri's two local transport hosts, and excludes subdomains. Trusted HTTPS uses the platform's normal certificate checks. A changed Mac host requires rebuilding; no global cleartext or certificate bypass is used.

Only a user selection in the bundled screen can choose a server origin. Top-level remote navigation stays on that origin. Remote pages cannot use the connection commands; those commands verify the native WebView's current local URL. Desktop administration is absent from the Android command handler. The desktop bridge's local-profile validation stays unchanged.

Enter the full Mac profile address shown by Music Studio KH, including its port. The profile must already have authorized LAN access and be reachable over Wi-Fi or the tailnet. Log in on the Mac-served library page; the connection screen stores only the validated server URL. Normal WebView cookies hold the library session. The APK does not configure networking, open internet ports or alter the Mac profile.

## Touch and TV

The Mac library keeps its existing responsive layout. Android-only initialization adds 48px player targets, visible focus, a connection bar with the APK version, and spatial arrow navigation. Left/Right retain cursor movement in text fields and adjustment in sliders; Up/Down can leave single-line inputs. OK activates focused controls. Android Back closes the visible dialog/panel, returns to Songs from another page, then returns to the connection screen. Back from the connection screen exits.

The connection screen contains the app version and CHANGELOG-backed What's new. The remote library may show its own different Mac version; the Android connection bar identifies the APK version separately.

## Checks and limits

```bash
node scripts/test-android-config.mjs
```

```bash
node scripts/android-navigation-smoke.mjs
```

Rust library tests also cover exact-host URL validation. `scripts/android-emulator-smoke.mjs` drives an installed debug WebView over CDP with ADB D-pad/Back events against an isolated fixture server on port 4800. It requires an already running emulator, installed APK, forwarded WebView debugging endpoint on port 9223, and the fixture server; it does not touch the owner's library. The delivery report records the actual emulator/build results.

This is a foreground streaming client. Offline caching and dependable background/locked-screen audio are not implemented or qualified. Mac sleep/network loss interrupts streaming. ARMv7 compilation does not certify Android 7.1.2 or the owner's 1 GB TV box. Real phone/tablet/TV touch, remote firmware, old WebView, physical audio outputs, Bluetooth/headsets, sleep/background behavior and memory on low-RAM hardware require physical tests.
