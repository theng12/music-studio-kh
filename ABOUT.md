# Music Studio KH — what it is

Music Studio KH is a personal music library and server for Windows, macOS and Linux.
It indexes a folder of music, plays it in the app or browser, and serves
Subsonic-compatible phone apps on the same Wi-Fi. Docker and separately installed
tools are not needed.

## What it does

- Creates independent profiles with their own port, login, database, artwork,
  downloads and settings; two profiles can share one music folder.
- Imports files and folders by drag-and-drop without moving or overwriting
  originals, and keeps YouTube download queues with progress, cancel and retry.
- Indexes large libraries in SQLite, continues past unreadable folders and rescans
  when a drive reconnects; the README cites 10,000-track libraries.
- Searches songs, albums and artists, and uses embedded, folder or custom artwork
  with light, dark or system themes.
- Plays in the browser or desktop app with seeking, a queue and FFmpeg conversion
  for formats the browser cannot play.
- Supports a Subsonic/OpenSubsonic API subset for phone apps with password or
  token-and-salt sign-in; the mobile secret is encrypted in a private 0600 file.
- Limits browser sessions, throttles failed logins separately for browser and
  phone apps, and confines streaming to paths inside the library.
- Shows recent authenticated clients and open audio requests in Settings; a player
  can keep playing buffered audio after its request ends.
- Leaves saved playlists, scrobbling and favorites unimplemented; real
  phone-client interoperability, Intel Mac installs and installer signing remain
  release checks.

## How it is built

Go server (`cmd/music-studio-kh`, `internal/`), React/TypeScript UI (`ui/`), Tauri 2
desktop shell (`src-tauri/`) and bundled yt-dlp, Deno, FFmpeg and ffprobe. Tests
include Go suites, browser smoke scripts and desktop end-to-end scripts.

Build the signed macOS DMG with:

    bash scripts/build-mac.sh

## Where it fits

It is a standalone server on the owner's machine: a profile owns its folder and
port, and optional `--lan` listening is plain HTTP for a trusted network. It has no
Studio Hub or Gateway job role. macOS DMGs are built here and travel through the
shared Updater feed after acceptance.

## Current version

0.3.17, from the `version` field in `package.json`; the newest `CHANGELOG.md` entry
agrees.
