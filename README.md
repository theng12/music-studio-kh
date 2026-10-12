# TuneHarbor

Your music, at home.

TuneHarbor is a planned standalone desktop music library and server for Windows, macOS, and Linux. Install it, choose a music folder, import files or download audio with yt-dlp, and listen on your computer or through compatible Subsonic/OpenSubsonic mobile clients on your local network.

## Approved first-version scope

- Drag-and-drop music imports and YouTube links, persistent download queues, and automatic indexing.
- A custom server and desktop/browser player, with no Docker requirement.
- Song and album artwork, custom covers, and appearance settings.
- Multiple server profiles running concurrently with shared or separate music folders.
- Independent credentials and ports per profile.
- Separate installers and validation for Windows, macOS, and Linux.

## Project status

Design approved; implementation planning is next. No working app or installers exist yet.

Read the [design specification](docs/superpowers/specs/2026-10-12-music-app-design.md) for architecture, boundaries, and acceptance criteria. Planned technologies are Tauri, React/TypeScript, Go, SQLite, yt-dlp, and FFmpeg. Distribution licenses and any upstream code reuse will be reviewed before release.

## Development milestones

1. Profile storage, isolated server instances, and lifecycle management.
2. Library scanning, artwork, authentication, and audio streaming.
3. Downloader jobs and safe shared-library imports.
4. Desktop/browser player, appearance settings, and profile manager.
5. Subsonic/OpenSubsonic interoperability and platform packaging.

Each milestone needs a reviewable implementation plan and its own validation. The computer must remain awake with the server running for mobile playback. Initial local-network access uses HTTP and is intended for trusted networks.
