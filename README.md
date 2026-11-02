# Music Studio KH

**Your music, at home.**

Music Studio KH is a music library and server for Windows, macOS, and Linux. The approved product will offer a desktop installer, yt-dlp downloads, artwork, profiles, and Subsonic-compatible listening without Docker.

## Current build: release candidate (unsigned)

Music Studio KH is a desktop app with its own music server. Install it, create a profile, choose a music folder, and listen in the app, in a browser, or in a Subsonic phone app such as Amperfy on the same Wi-Fi. Docker, a terminal and separately installed tools are not needed: yt-dlp, FFmpeg and Deno ship inside the app.

What works:

- **Profiles:** independently authenticated servers running at the same time on separate ports, with shared or separate music folders. Each profile has its own database, artwork, downloads, settings and logs. If a folder disappears, the profile can switch to a replacement folder.
- **Adding music:** drag-and-drop imports of files and folders (never overwriting or moving the originals; works on exFAT/FAT drives). Persistent YouTube download queues with progress, cancel and retry, and playlists split into individual songs.
- **Library:** SQLite indexing that keeps going past unreadable folders, recovers after a drive reconnects, and handles 10,000-track libraries. Search, albums and artists.
- **Artwork and appearance:** embedded, folder and custom song or album artwork, plus light/dark/system themes, accent colors, density and layout.
- **Player:** browser and desktop player with seeking, a queue and FFmpeg conversion for formats the browser cannot play.
- **Phone apps:** Subsonic/OpenSubsonic API subset with password and token-and-salt sign-in. The mobile secret is encrypted with a key held in the OS keychain.
- **Security:** expiring browser sessions, separate failed-login throttling for the browser and phone apps, origin checks, and path-contained streaming.
- **Desktop app:** tray behavior with a first-close explanation, optional start at login, single instance, and a clean quit that stops every server and tool.

Installers come from the **Desktop installers (unsigned development)** workflow: Windows (NSIS), macOS Apple Silicon and Intel (DMG, ad-hoc signed), and Linux (.deb and AppImage). They are not signed with a Developer ID or Authenticode certificate. macOS and Windows will warn about an unidentified developer until release credentials are added. [docs/RELEASE-CHECKLIST.md](docs/RELEASE-CHECKLIST.md) lists every requirement with its evidence and the checks that need real devices.

No code was copied from Navidrome or MeTube.

## Build the app on your Mac

`scripts/build-mac.sh` produces an ad-hoc signed Music Studio KH DMG under `artifacts/`, without installing or opening the app. It builds for this Mac's processor and needs Apple's command line tools, Go, Node.js, Rust, Python, make and pkg-config. Missing tools are reported before building. The app requires macOS 13 or later; this release was tested on Apple Silicon, and Intel remains unverified. From the project folder:

```sh
bash scripts/build-mac.sh
```

The first build includes compiling LGPL FFmpeg from checksum-verified sources; later builds reuse compatible audio tools. The DMG bundles yt-dlp, Deno, FFmpeg, ffprobe and their notices, so the installed app needs no Homebrew tools. The script prints the DMG path, SHA-256, bundle identifier and version for Update Studio KH's `app-dmg` recipe. It does not publish to the update feed. Cross-platform installers are also available through the manually started **Desktop installers** workflow or a `v*` tag.

## Run the development build

Install Go 1.27.2 or newer, then:

```sh
go build -o bin/tuneharbor ./cmd/tuneharbor
./bin/tuneharbor profile create --name Personal --library /absolute/path/to/Music --username owner --port 4747
./bin/tuneharbor serve
```

On Windows the executable is `bin/tuneharbor.exe`. Use the executable from PowerShell with your Windows music-folder path. Profile creation prompts for a password, hidden in an interactive terminal. The minimum is six characters; no symbols or mixed-case requirement. Passwords are case-sensitive, usernames are not.

Open `http://127.0.0.1:4747` and sign in. Add existing audio files through the file picker or drop them into the collection area. Imports preserve source files. Browsers directly play supported formats. Conversion and downloads need FFmpeg and yt-dlp: pass `serve --tools DIR` with a folder holding `ffmpeg`, `ffprobe`, `yt-dlp` and `deno` (the desktop app bundles them).

To build the desktop app, run `python scripts/prepare-runtime.py` and `scripts/build-ffmpeg.sh` as the desktop workflow does, then `npm run build:desktop`. `npm run desktop` starts it in development mode.

Profile data defaults to the operating system's user configuration directory under `TuneHarbor/profiles`. To use another location, put `--data /path/to/profiles` **before** the command.

## Your music folder

Each profile reads the music folder you choose. Music stays there: you can add songs and album folders in Finder and the running app scans them automatically. They normally appear within about 20 seconds, then become available in the library and for authenticated streaming. The Import controls copy files into that same folder without moving the originals or overwriting existing songs.

Downloads also finish in the profile's music folder. The Downloads page shows its actual path on the server computer; this may differ from the device you use to browse the library. Library setup belongs in Settings, while Songs, Albums, Artists and Folders browse the collection.

## Multiple profiles

Create a second profile with another port:

```sh
./bin/tuneharbor profile create --name Family --library /absolute/path/to/Music --username family --port 4748
./bin/tuneharbor profile list
./bin/tuneharbor serve
```

Use the same music folder to share files, or choose a different folder for a separate collection. Each profile has its own credentials, database, artwork overrides, and port. Open port 4747 for Personal and 4748 for Family. Imported music appears in other profiles sharing that folder on their next scan (within 20 seconds). The desktop app manages the same profiles without the command line.

A hidden `.tuneharbor-library-id` marker identifies the selected folder; preserve it when moving or backing up the library. Existing indexed profiles stay online with their last index while a folder is disconnected, and resume scanning when the original folder returns.

Stop the server with Ctrl+C. Music files remain in place. Imports finalize with a hard link, or an exclusive copy on drives without hard links (exFAT, FAT32). Network-mounted folders are not supported for shared libraries. Each profile keeps a small log in its data folder under `logs/tuneharbor.log`.

## Local-network listening

Add `--lan` when creating a profile to enable listening on network interfaces. The server prints usable IPv4 addresses. An OS firewall rule may also be needed. LAN mode uses unencrypted HTTP, intended for trusted networks. The host computer must stay awake and the server must keep running. No public internet access is configured.

Phone apps sign in with the profile's server address (shown in Settings), username and password. Both Subsonic password mode (including `enc:` hex encoding, which is not encryption) and token-and-salt mode are supported. Token mode needs the password saved, encrypted, through the OS keychain; if the keychain was locked when the profile was created, enter the password once in the profile settings. Endpoints include `ping`, `getLicense`, `getMusicFolders`, `getArtists`, `getArtist`, `getAlbumList2`, `getAlbum`, `getSong`, `getRandomSongs`, `search3`, `getCoverArt`, `stream`, and `download`, plus empty responses for genres, starred items, playlists and radio. Saved playlists, scrobbling and favorites are not implemented. Interoperability with real iOS and Android clients is a release gate that still needs a run on real devices (see the checklist).

In **Settings → Streaming & access**, activity shows open audio requests and authenticated clients seen in the last five minutes. A player can keep playing buffered music after its audio request finishes. Browser sessions and phone-app requests identify clients; this is not device discovery, and one device may have several sessions. The view reports its 100-client tracking limit if reached and shows refresh errors explicitly.

## Verification

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o bin/tuneharbor ./cmd/tuneharbor
npm ci
npx playwright install chromium
npm run test:browser
```

`scripts/desktop-e2e.sh` drives an installed Linux package under a virtual display: first-run profile creation through the real folder dialog, the signed-in library window, import, playback, close-to-tray and server cleanup. CI runs it after building the `.deb`. The browser smoke script runs on Linux/macOS; the Go CI matrix also tests Windows. Browser testing creates temporary profiles and real audio/image fixtures, checks login/import/playback/search/artwork/theme behavior, and retires its temporary data to project trash afterward. It writes desktop/mobile screenshots under `artifacts/`. `node scripts/settings-layout-smoke.mjs` checks stable Settings geometry in Chromium and WebKit when installed; `node scripts/library-folder-smoke.mjs` checks direct folder additions and authenticated streaming; `node scripts/downloads-smoke.mjs` checks the fixture queue and actual save location. `python3 scripts/audio-tools-smoke.py /path/to/runtime-tools` checks five audio formats, PNG conversion and embedded cover art without network calls.

Go sources are cross-compiled for Windows x64 and macOS Intel/Apple Silicon during development. Native runtime tests, mobile interoperability, installer signing, and notarization are distinct release checks.

## Design and build plan

- [Approved design](docs/superpowers/specs/2026-10-12-music-app-design.md)
- [Foundation plan and follow-up milestones](docs/superpowers/plans/2026-10-12-foundation.md)
- [Execution record](docs/superpowers/plans/foundation-progress.md)
- [Release plan](docs/superpowers/plans/2026-10-12-release.md) and [release progress](docs/superpowers/plans/release/progress.md)
- [Release checklist](docs/RELEASE-CHECKLIST.md)
