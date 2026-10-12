# TuneHarbor

**Your music, at home.**

TuneHarbor is a music library and server for Windows, macOS, and Linux. The approved product will offer a desktop installer, yt-dlp downloads, artwork, profiles, and Subsonic-compatible listening without Docker.

## Current build: development foundation

Implemented:

- Independently authenticated server profiles running concurrently on separate ports.
- Shared or separate music folders, with separate databases and artwork per profile.
- SQLite indexing, metadata extraction, search, and periodic/manual library scans.
- No-overwrite file imports and drag-and-drop browser uploads.
- Embedded/folder artwork and custom song artwork with reset.
- Browser player with seeking, volume, previous/next, responsive layout, and appearance settings.
- Six-character minimum passwords, including numbers-only passwords; case-insensitive usernames.
- Expiring browser sessions, failed-login throttling, bounded uploads, and path-contained streaming.
- A documented Subsonic API subset using password authentication.

Still to build: yt-dlp/FFmpeg download jobs and transcoding, a Tauri desktop profile manager and native folder picker, custom album artwork, cross-device appearance defaults, OS-keychain-backed token authentication, mobile-client interoperability checks, and signed installers. This build requires development setup; it is **not yet the simple drag-and-drop desktop release**.

No code was copied from Navidrome or MeTube. Third-party dependency notices and redistribution licenses must be completed before releases.

## Run the development build

Install Go 1.27.2 or newer, then:

```sh
go build -o bin/tuneharbor ./cmd/tuneharbor
./bin/tuneharbor profile create --name Personal --library /absolute/path/to/Music --username owner --port 4747
./bin/tuneharbor serve
```

On Windows the executable is `bin/tuneharbor.exe`. Use the executable from PowerShell with your Windows music-folder path. Profile creation prompts for a password, hidden in an interactive terminal. The minimum is six characters; no symbols or mixed-case requirement. Passwords are case-sensitive, usernames are not.

Open `http://127.0.0.1:4747` and sign in. Add existing audio files through the file picker or drop them into the collection area. Imports preserve source files. Browsers directly play supported formats; automatic conversion is not implemented yet.

Profile data defaults to the operating system's user configuration directory under `TuneHarbor/profiles`. To use another location, put `--data /path/to/profiles` **before** the command.

## Multiple profiles

Create a second profile with another port:

```sh
./bin/tuneharbor profile create --name Family --library /absolute/path/to/Music --username family --port 4748
./bin/tuneharbor profile list
./bin/tuneharbor serve
```

Use the same music folder to share files, or choose a different folder for a separate collection. Each profile has its own credentials, database, artwork overrides, and port. Open port 4747 for Personal and 4748 for Family. Imported music appears in other profiles sharing that folder on their next scan (within 20 seconds). Profiles are currently managed through the command line; the desktop profile manager is a later milestone.

A hidden `.tuneharbor-library-id` marker identifies the selected folder; preserve it when moving or backing up the library. Existing indexed profiles stay online with their last index while a folder is disconnected, and resume scanning when the original folder returns.

Stop the server with Ctrl+C. Music files remain in place. Import finalization uses hard links on the local library filesystem; filesystems lacking hard-link support are not supported by this initial implementation.

## Local-network listening

Add `--lan` when creating a profile to enable listening on network interfaces. The server prints usable IPv4 addresses. An OS firewall rule may also be needed. LAN mode uses unencrypted HTTP, intended for trusted networks. The host computer must stay awake and the server must keep running. No public internet access is configured.

Subsonic password mode is implemented, including `enc:` hex-encoded password input (encoding is not encryption). Token-and-salt authentication is **not implemented**. Clients that require it cannot connect yet. Endpoints include `ping`, `getLicense`, `getMusicFolders`, `getArtists`, `getArtist`, `getAlbumList2`, `getAlbum`, `getSong`, `search3`, `getCoverArt`, `stream`, and `download`. Saved playlists, scrobbling, favorites, pagination, duration metadata, and complete client compatibility remain outstanding. No real iOS/Android client interoperability is claimed yet.

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

The browser smoke script currently runs on Linux/macOS; the Go CI matrix also tests Windows. Browser testing creates temporary profiles and real audio/image fixtures, checks login/import/playback/search/artwork/theme behavior, and removes the temporary data afterward. It writes desktop/mobile screenshots under `artifacts/`.

Go sources are cross-compiled for Windows x64 and macOS Intel/Apple Silicon during development. Native runtime tests, mobile interoperability, installer signing, and notarization are distinct release checks.

## Design and build plan

- [Approved design](docs/superpowers/specs/2026-10-12-music-app-design.md)
- [Foundation plan and follow-up milestones](docs/superpowers/plans/2026-10-12-foundation.md)
- [Execution record](docs/superpowers/plans/foundation-progress.md)
