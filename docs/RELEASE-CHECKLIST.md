# TuneHarbor 0.1 release checklist

Each requirement from the [approved design](superpowers/specs/2026-10-12-music-app-design.md) with its evidence.

- **PASS:** automated evidence runs in CI on every push.
- **BLOCKED:** needs something CI cannot provide (real phones, signing identities, an unrestricted network, a person at a real machine).

A stable release needs every row to be PASS. Until then, builds are **unsigned release candidates**.

## Gates that need the owner

| Gate | Why CI cannot do it | What to do |
| --- | --- | --- |
| iOS interoperability (Amperfy) | Needs a real iPhone on the same Wi-Fi | Install a candidate, enable phone access on a profile, then in Amperfy sign in with the address shown in Settings. Check browsing, search, artwork, playback and seeking. Record the app and iOS versions here. |
| Android interoperability (e.g. Symfonium, Ultrasonic, DSub) | Needs a real Android phone | Same steps; record the app and Android versions. |
| Live YouTube download | The cloud build environment's proxy blocks YouTube | On a home network, paste a song link and a playlist link; confirm both finish and appear in the library. |
| Windows code signing | Needs an Authenticode certificate | Provide a certificate (or Azure Trusted Signing) as CI secrets; until then SmartScreen warns. |
| macOS Developer ID signing and notarization | Needs an Apple Developer account | Provide the signing identity and notarization credentials as CI secrets; until then Gatekeeper asks users to approve the app in System Settings. |
| Hands-on run on Windows 11 and macOS | CI checks install, payload and uninstall, but not the GUI | Install, create a profile, import, download, play, use a phone on the same Wi-Fi, close to tray, quit, uninstall. Confirm music remains. |
| Third-party license review | Needs a legal decision on distribution terms | Review `runtime-tools/notices` and the dependency manifest that CI bundles. yt-dlp is GPL-3.0-or-later and shipped as a separate executable. FFmpeg is built LGPL-only with source archives included. |

## Requirements and evidence

| Requirement | Status | Evidence |
| --- | --- | --- |
| Installers: Windows NSIS, macOS DMG (Apple Silicon and Intel), Linux .deb and AppImage | PASS (unsigned) | `desktop.yml` builds each target. `scripts/install-smoke.py` checks the payload, installs and removes (Linux via apt, Windows silent install and uninstall, macOS bundle seal). |
| No Docker, terminal or preinstalled tools | PASS | yt-dlp, FFmpeg/ffprobe (LGPL build) and Deno are bundled with SHA-256 pinned sources (`packaging/runtime-tools.json`). The Linux E2E runs on a machine without them on the PATH. |
| First run: profile, folder picker, password, library opens | PASS (Linux GUI) | `scripts/desktop-e2e.sh` drives the installed app and the real GTK folder dialog. |
| Drag-and-drop and file imports never overwrite and keep originals | PASS | Library, desktop and browser tests: collisions get `(n)` suffixes, sources are kept, concurrent imports keep both, unreadable entries are skipped, and imports into the folder itself are refused. |
| exFAT/FAT drives | PASS (simulated) | Exclusive-copy fallback when hard links fail; test with link failure injected. |
| Download queue: persistent, progress, cancel, retry, playlists as child jobs, restart recovery | PASS (fixture tool) | Queue tests use a controlled yt-dlp stand-in. Progress output was checked with the real bundled yt-dlp on a local HTTP file. Live YouTube is a gate above. |
| URL safety (YouTube HTTP(S) only, no argument injection, a song link stays one song) | PASS | URL validation tests; arguments always passed as arrays after `--`. |
| Automatic indexing, rescans, missing files, unreadable folders | PASS | Scan and reconcile tests, a 10,000-track test, watcher test, unreadable-folder test (CI runs as a normal user). |
| Disconnected or changed folder pauses work and preserves data | PASS | Replacement and empty-folder tests, reconnect-after-replug test, downloads pause while the folder is away, replacement folder via profile settings. |
| Artwork: embedded, folder, song and album overrides, reset, malformed images | PASS | Library artwork and thumbnail tests; browser smoke uploads custom art and checks isolation between profiles. |
| Appearance: theme, accent, density, layout, saved per profile | PASS | Settings tests and browser smoke reloads. |
| Player: play/pause, seek, volume, next/previous, queue | PASS | Browser smoke and desktop E2E play real audio. |
| Streaming: auth required, byte ranges, conversion when needed, bounded conversions | PASS | Server range tests and FFmpeg conversion and cache tests. |
| Multiple concurrent profiles: own ports, credentials, data, artwork, logs | PASS | Manager lifecycle, port-conflict and log tests; browser smoke runs two profiles on one shared folder. |
| Profile removal keeps music | PASS | Manager and profile tests. |
| Browser auth: salted hashes, expiring sessions, sign out everywhere, rate limits, origin checks | PASS | Server auth tests. |
| Subsonic subset: password and token-and-salt sign-in, browse, search, artwork, stream, seek | PASS (protocol) / BLOCKED (real clients) | Protocol tests and an installed-sidecar token sign-in with OS keychain encryption (`native-rpc-smoke.py`). Real phones are a gate above. |
| Subsonic throttling does not break parallel clients | PASS | Parallel valid-request test; failed attempts throttled separately from the browser. |
| Mobile secret encrypted with an OS-keychain key; keychain failure does not block profiles | PASS | Vault tests; a locked-keychain test creates a browser-only profile. |
| LAN off by default, HTTP-only warning, addresses shown, firewall guidance | PASS | Settings UI; server binds to loopback unless LAN is enabled. |
| Desktop bootstrap credential is private, single-use, expires, loopback only | PASS | Bootstrap tests; installed-sidecar check. |
| Tray: close keeps servers, first close explains, Quit stops everything | PASS (Linux) | Desktop E2E checks the explanation, close-to-tray and server exit with the app. On Linux, tools die with a killed server via a parent-death signal test. |
| Start at login (optional, per profile) | PASS (logic) | Autostart turns on only while a profile asks for it, and turns off when the last such profile is removed. The OS login item itself is unverified without a real sign-in. |
| Single instance | PASS (by construction) | Tauri single-instance plugin focuses the existing window. Not automated. |
| Actionable errors (disk full, folder unavailable, tool failures, port busy) | PASS | Storage error mapping test, port-conflict test, download failure messages. |
| Logs exclude secrets | PASS | Log test checks that neither the password nor the bootstrap token appears. |
| Windows 11, macOS Intel and Apple Silicon, Ubuntu 24.04 runtime | PASS (CI install smoke) / BLOCKED (hands-on) | See the gates above. |
| Signed and notarized installers | BLOCKED | Needs release credentials. |
