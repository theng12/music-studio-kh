# TuneHarbor foundation implementation plan

> **For agentic workers:** Use superpowers:executing-plans for implementation. Steps use checkboxes for tracking.

**Goal:** Deliver the first runnable development milestone: independently authenticated profile servers, persistent libraries, artwork, imports, browser playback, and Subsonic-compatible core endpoints.

**Architecture:** A Go process starts one HTTP server per configured profile. SQLite and music files remain separate, and an embedded browser UI uses authenticated profile APIs. The desktop shell, download worker, advanced compatibility, and release installers follow in separate milestones rather than being represented as finished here.

**Tech stack:** Go, modernc SQLite, bcrypt, dhowden/tag, vanilla browser JavaScript/CSS for an embedded development player. The planned React/Tauri production interface remains a later milestone.

**Spec:** ../specs/2026-10-12-music-app-design.md

## Global constraints

- Target Windows 11 x64, macOS Apple Silicon and Intel, and Ubuntu 24.04 x64 initially.
- Preserve music files on profile removal and never overwrite imports.
- Profiles have independent credentials, databases, queues, artwork, settings, and ports.
- Default to loopback; LAN access is explicit and uses HTTP only on trusted networks.
- No claim of full mobile interoperability or signed installers without platform validation.

## Review focus

1. Symbolic links escaping the music folder must never be indexed or streamed.
2. Failed scans of disconnected folders must preserve the previous index.
3. Same-name simultaneous imports must preserve both files.
4. Credentials and expired sessions must not cross profile boundaries.
5. Malformed images and oversized uploads must produce actionable failures.

## Task 1: persistent profiles

Files: `go.mod`, `internal/profiles/profiles.go`, `internal/profiles/profiles_test.go`.
Produces: `Create(root string, input Input) (Profile, error)`, `List(root string) ([]Profile, error)`, `Open(root, id string) (Profile, error)`; `Input{Name, Library, Username, Password string; Port int; LAN bool}`; Profile includes ID, Name, Library, Username, PasswordHash, Port, LAN, DataDir.

- [ ] Write tests for persistence, invalid names/ports, duplicate ports, shared-folder profiles, private credential storage, and symlink roots.
- [ ] Run `go test ./internal/profiles`; expect missing implementation, then implement validated, atomic config storage with bcrypt.
- [ ] Run that suite; expect PASS. Commit profile storage.

## Task 2: library, artwork, safe imports

Files: `internal/library/library.go`, `scan.go`, `import.go`, `artwork.go` and corresponding tests.
Consumes: `Profile.Library`, `Profile.DataDir`.
Produces: `Open(dataDir, root string) (*Library, error)`, `Scan() error`, `Tracks(query string) ([]Track, error)`, `Track(id string) (Track, error)`, `Import(name string, source io.Reader) (Track, error)`, `Artwork(id string) ([]byte, string, error)`, `SetArtwork(id string, source io.Reader) error`; Track has ID, Title, Artist, Album, Path, Format, Size and Artwork.

- [ ] Tests create actual WAV files, scan/import/query them, reject escaping symlinks, reconcile deleted files, preserve index after unavailable-root failure, preserve both collision imports, and reject malformed/oversized artwork.
- [ ] Run `go test ./internal/library`; verify missing behavior. Implement SQLite schema, deterministic path IDs, tag extraction/fallbacks, embedded/folder/custom artwork, canonical-path containment and exclusive no-overwrite imports.
- [ ] Run library and full suites; expect PASS. Commit library functionality.

## Task 3: HTTP auth, streaming, compatibility subset

Files: `internal/server/server.go`, `auth.go`, `api.go`, `subsonic.go`, `server_test.go`.
Consumes: profile and library from Tasks 1–2.
Produces: `New(profile profiles.Profile, library *library.Library) *Server`, `Handler() http.Handler`. Endpoints: login/logout, tracks, scan, imports, track artwork, streams, and a documented Subsonic subset.

- [ ] Tests assert unauthorized streams fail, wrong-profile sessions fail, login and range seek succeed, unsafe origins fail, invalid image returns 400, and Subsonic XML/JSON auth/error envelopes work.
- [ ] Run `go test ./internal/server`; verify missing behavior. Implement expiring HttpOnly SameSite sessions, origin validation, bounded uploads, login throttling and authenticated ServeContent. Subsonic password mode is supported initially; legacy token-and-salt remains explicitly unsupported until OS-keystore integration.
- [ ] Run all Go tests and race detector; expect PASS. Commit HTTP server.

## Task 4: runnable UI and command

Files: `internal/server/web/{index.html,app.js,style.css}`, `cmd/tuneharbor/main.go`, `scripts/dev.sh`, `README.md`.
Consumes: Task 3 HTTP API and Task 1 profiles.
Produces: embedded login/library/player UI and `tuneharbor profile create`, `profile list`, `serve` commands; all configured profiles run concurrently.

- [ ] Add an integration test that fetches the embedded UI and exercises a real login → import → scan → stream → artwork sequence.
- [ ] Run test before implementation; confirm missing static UI behavior.
- [ ] Implement the browser player with search, upload/drop imports, scan, artwork replacement, theme/accent/layout settings, now-playing controls, and profile status. CLI reads passwords interactively or through a password file, never via positional args.
- [ ] Run full Go suite, browser smoke checks, Linux build and Windows/macOS cross-compiles. Document this as a development foundation, with remaining desktop/download/installer milestones explicit.
- [ ] Commit, fresh review, fix material findings, push feature branch and open a draft PR.

## Follow-up plans

1. yt-dlp/FFmpeg job persistence, progress, cancellation, restart recovery and bounded transcoding.
2. Tauri/React desktop profile manager, native folder picker, drag/drop, tray supervision and OS-keystore-backed compatibility auth.
3. Mobile interoperability expansion, managed tool bundles, update integrity and native platform release checks.

These remain part of the approved product scope; the foundation is an incremental build, not a substitute for that scope.
