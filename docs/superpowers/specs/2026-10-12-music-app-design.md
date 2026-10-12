# Desktop music library and server

## Purpose and accepted scope

Build an installable app for people with little technical knowledge. It stores music on their own computer, downloads music through yt-dlp, and serves a library through its own music server and player. No Docker installation or terminal setup is required. Target Windows, macOS, and Linux with separate installers and platform validation.

The first version includes folder selection, drag-and-drop music imports, pasted or dropped YouTube links, a persistent download queue, understandable failures, automatic metadata scanning, browsing and playback, and password-protected listening from devices on the same Wi-Fi. The host computer must remain awake and the app's server must remain running.

Updated scope: multiple named server profiles on one computer, each with its own owner account, selected music folder, and local-network settings. Profiles can share a folder or use separate libraries. Song and album artwork, manual artwork customization, and appearance settings are included. Internet exposure, multi-user administration within a profile, general metadata editing, saved playlist management, external artwork lookup, and a plugin system are deferred.

## Artwork and customization

Display artwork in track lists, album pages, the player, and compatible mobile clients. Prefer a track-specific artwork override, then an album override, then embedded artwork, then conventional cover.jpg/cover.png in the album folder, then a placeholder. Cache decoded thumbnails and expose artwork through authenticated endpoints.

The owner can select or drop a JPEG, PNG, or WebP image for a song or album, preview it, replace it, or reset it. Validate actual image contents, file size, and decoded dimensions before processing. Store overrides in the profile's application-data directory; do not rewrite source audio files. Artwork overrides are profile-specific even when audio files are shared. Album covers selected manually apply to tracks without a track-specific override.

Appearance settings include light/dark/system mode, an accent color, comfortable/compact library density, and list/grid album presentation. Persist these defaults per profile and apply them in the desktop and browser interfaces. Third-party mobile apps control their own appearance. Arbitrary CSS, downloadable themes, and a theme/plugin marketplace are deferred.

## Multiple profiles and running instances

A profile is a named, independently configured server instance, not an additional user account within one server. A profile manager creates, renames, selects, starts, stops, and removes profiles. Multiple profiles can run concurrently under one desktop supervisor. Switching the displayed profile does not stop other running profiles.

Each profile owns a unique identifier, SQLite database, owner credentials, OS credential-store entries, cache, artwork overrides, download queue, logs, appearance defaults, and listening port. Allocate a free port when creating a profile and persist it; report conflicts with an action to choose another port. Display the profile's URL and running status clearly so users configure the correct mobile client. Startup preferences are per profile. Quit stops all processes supervised by the app; closing to the tray leaves running profiles active.

Separate-library profiles choose different music folders. Shared-library profiles explicitly select the same canonical folder and maintain independent indexes and settings. Added files become visible to every running profile using that folder after a rescan/notification. Desktop and mobile clients remain attached to the profile they opened; authentication does not carry over to another profile.

All app-managed writes to a shared folder use a cross-process folder lock and unique staging/final paths. Finalize imports/downloads without overwriting existing files. Indexers ignore staging files. Coordinate notifications across running profiles, with periodic reconciliation for missed notifications or external changes. Do not promise reliable locking on network-mounted folders in the first version; supported shared libraries are on a local disk. Library deletion and in-place tag editing are outside the first version, avoiding destructive shared-file operations.

Removing a profile requires confirming that its configuration and private data will be removed; it never removes the music folder or audio files. Stop that profile before removal. A failure in one server must not stop the others. A second launch of the desktop app focuses the existing supervisor/profile manager instead of duplicating its processes.

## Architecture

- Tauri desktop shell manages setup, folder dialogs, drag-and-drop, tray actions, and service lifecycle.
- A React/TypeScript interface provides the library, player, downloads, and settings. The same interface runs in a browser for phone and local-network listening.
- A Go server owns authentication, library indexing, SQLite persistence, job orchestration, and streaming. It is a bundled executable supervised by the desktop shell.
- yt-dlp and FFmpeg are bundled runtime tools invoked with explicit argument arrays. Users do not install or configure these tools themselves.

The UI talks to a versioned HTTP API. Library indexing, downloads, streaming, and authentication have separate modules behind internal interfaces. This permits future features without making all modules depend on each other. A full extension API is unnecessary for the first version.

## Setup and everyday use

On first launch, the owner creates a named profile, chooses a writable music folder, and creates an owner username and password. The app checks folder access and bundled tool availability before showing the library. Settings persist in the OS application-data directory, separate from music files.

Dropping supported audio files copies them into the music folder without removing the originals. Directory imports recursively accept audio files and skip symbolic links. Name collisions receive a unique suffix; no existing file is overwritten. Initial playback support is MP3, M4A/AAC, FLAC, Ogg Vorbis, Opus, and WAV, subject to browser support and the streaming conversion path.

Pasting or dropping an HTTP(S) YouTube video or playlist URL creates a queue job. Other URL schemes and unsupported hosts are rejected. A playlist becomes individual child jobs. The downloader extracts the best available audio; FFmpeg produces a compatible audio file when required. Conversion does not claim to improve source quality. The interface shows title, state, progress when available, and retry/cancel actions.

Downloads use a staging directory outside the indexed library. Only successfully finalized files move into the library and trigger a scan. Jobs run one at a time per profile initially. If staging is on another filesystem, copy to an ignored temporary file on the library filesystem before finalizing under the shared-folder lock. Completed and failed jobs remain visible across restarts; interrupted work is marked interrupted and can be retried. Cancellation terminates child processes and removes incomplete staging files.

The library displays artists, albums, tracks, resolved artwork, and search. The player supports play/pause, seek, volume, next/previous, and a temporary play queue. A Listen action opens the built-in player; no Navidrome service is required.

## Library and streaming

SQLite stores tracks, extracted metadata, file identity, scan history, download jobs, and owner authentication records. Music remains ordinary files in the selected folder. Missing tags fall back to filenames and an unknown artist/album label.

A startup scan, post-import scan, filesystem notifications, and a manual rescan keep the index current. Notifications are debounced; full rescans recover missed changes. Missing files are removed from the index without deleting user files. Metadata failures affect the individual file and are reported without aborting the scan.

Streaming uses authenticated track identifiers rather than arbitrary paths and supports HTTP byte ranges for seeking. Compatible formats stream directly. FFmpeg provides a bounded conversion stream when the requesting browser cannot play the source format. Resource limits prevent concurrent conversions from overwhelming the host.

## Local-network access and authentication

Passwords require a minimum of six characters; numbers-only passwords such as 123456 are accepted. Usernames are case-insensitive in browser and Subsonic authentication. Preserve their display spelling.

Alongside the browser player, support existing Subsonic/OpenSubsonic mobile clients, as with Navidrome. Setup creates an owner username and password. Settings show the full server URL (IP address and port) and username to enter in a compatible client. General-purpose App Store music apps are not automatically compatible.

Implement a documented compatibility subset covering ping, server/license information, music folders, artists, albums, tracks, search, artwork, and streaming with seeking. Match protocol response envelopes, identifiers, errors, and supported authentication modes. Advertise only implemented OpenSubsonic extensions. Saved playlists, scrobbling, favorites, and other optional APIs remain outside the first version; do not claim complete Navidrome parity.

Subsonic clients can require credential authentication different from browser sessions, including legacy token-and-salt authentication that cannot be verified from an ordinary password hash alone. Preserve browser password hashing and isolate compatibility authentication in an adapter. Any recoverable compatibility secret must be encrypted with a key held in the OS credential store, never stored as plaintext in SQLite or logs. Verify the selected client authentication modes before implementation. Apply rate limits to protocol authentication as well as browser login.

Acceptance requires actual interoperability checks with at least one iOS and one Android Subsonic/OpenSubsonic client: login by server address, browsing, search, artwork, playback, and seeking. Record tested app names and versions and document unsupported features. No mobile compatibility claim is complete until these checks pass.

The server binds to loopback by default. The owner can enable local-network listening in settings, see the host address and port, and open or copy the browser-player address. Operating-system firewall permission may be necessary; the app explains the concrete step when connectivity is blocked. No router port forwarding or public exposure is configured.

Remote browsers sign in with the owner password. Passwords are stored as salted password hashes. Sessions expire and can be revoked by signing out all devices. Login attempts are rate limited. Mutating requests enforce origin/CSRF protections, and APIs require authentication, including streaming. The desktop establishes its local session through a private bootstrap credential, never a token embedded in a public URL.

Initial LAN delivery uses HTTP: passwords and traffic lack transport encryption on the network. Setup states this limitation before LAN mode is enabled. HTTPS with trusted certificates is a later feature. Authentication limits access but does not make an untrusted Wi-Fi network safe.

## Lifecycle and failure handling

The shell starts the Go service with a private readiness handshake and detects startup errors and port conflicts. Closing the window keeps the app in the tray by default; an explicit Quit stops the server and downloader processes cleanly. The first close explains this behavior. Start-at-login is optional and initially disabled.

The app shows actionable errors for unavailable sources, failed downloads, disk exhaustion, unreadable files, unavailable folders, and tool failures. Logs exclude passwords, session credentials, and sensitive URL query parameters. A changed or disconnected library folder pauses imports and downloads until access returns or the owner selects a replacement.

## Reuse, licensing, and packaging

Use existing services as references and reuse compatible libraries where useful. Copying Navidrome, MeTube, or other project code is not an implementation assumption: any reuse first records the exact source, version, license obligations, and compatibility with the app's distribution license. Maintain third-party notices and bundled-tool manifests. FFmpeg build options and all bundled dependency licenses must be checked for each distribution.

Release packages include compatible Go, yt-dlp, and FFmpeg binaries and the Tauri prerequisites needed on each platform. Support Windows 11 x64, macOS Apple Silicon and Intel, and Ubuntu 24.04 x64 initially. This does not promise every Linux distribution or CPU architecture. Broader support can be added after validation.

Windows gets an installer, macOS an application bundle distributed through a disk image, and Linux a Debian package plus an AppImage where supported. Signing and macOS notarization require release credentials; unsigned development packages are distinguished from signed releases. App/tool updates use versioned, integrity-checked artifacts; the first version can notify users and direct them to a full installer update rather than silently replacing tools.

## Validation and acceptance

Additional acceptance checks cover embedded covers, folder covers, song/album overrides and reset, malformed images, appearance persistence, and artwork delivery to mobile clients. Run two profiles concurrently with different ports and credentials; verify separate-library isolation and shared-library visibility, simultaneous downloads/imports without overwrites, profile switching without server interruption, port-conflict recovery, and profile removal preserving music. Credential, database, and artwork isolation must hold even when the audio folder is shared.

Server tests cover scan reconciliation, collisions, persistent job states, cancellation, failed staging cleanup, authentication, path containment, streaming ranges, and migration behavior. UI tests cover setup, import/download state changes, search, playback controls, and actionable failures. Downloader integration tests use a controlled fixture process so routine tests do not depend on live YouTube availability; a separate manual source check verifies the bundled yt-dlp version.

On each target OS, verify fresh install, first launch without Docker or preinstalled tools, a music import, a completed download, automatic library appearance, seeking/playback, a second-device login, restart recovery, tray behavior, clean quit, and uninstall behavior. Uninstall must preserve music files. Browser playback is checked on desktop Chromium/Safari and mobile Safari/Chrome where available.

Success means a new user can install the app, choose a folder, add music, and listen without terminal commands. Development in a Linux cloud environment can validate Linux functionality and cross-platform source structure; Windows/macOS installer and runtime claims require actual checks on those systems.
