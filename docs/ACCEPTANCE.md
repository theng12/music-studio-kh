# Acceptance — Music Studio KH

Every screen a person works in, the end-to-end test that proves it, and the standard it is
compared against. A row is only `automated` when a test in this repo drives the real form to the
real API and checks the result. `live-pending` rows need a real phone, a real YouTube link or a
real signed install — they are named in [RELEASE-CHECKLIST.md](RELEASE-CHECKLIST.md) and are never
reported as passed.

Status values: **automated** (green in the routine suite), **live-pending** (needs a real
account/device/owner window), **not checked** (nothing proved it here).

## 1. Tasks

| Task (what a person must do) | Test (form → API → result) | Status |
| --- | --- | --- |
| Open the library after signing in | `scripts/browser-smoke.mjs` — login → library visible | automated |
| Add songs from files (keeps originals, never overwrites) | `scripts/browser-smoke.mjs` (import + duplicate skipped), `internal/library` tests | automated |
| Add songs from YouTube links | `scripts/downloads-smoke.mjs`, `internal/downloads` queue tests with the fixture yt-dlp | automated (fixture) / live-pending (real YouTube) |
| Watch, cancel, retry and clear a download | `scripts/downloads-smoke.mjs`, `internal/downloads/queue_test.go`, `hold_test.go` | automated |
| Play, pause, seek, next/previous, shuffle, queue | `scripts/player-controls-smoke.mjs`, `scripts/test-playback-queue.mjs`, `scripts/music-usability-smoke.mjs` | automated |
| Edit artwork, album, year, lyrics | `scripts/browser-smoke.mjs` — artwork upload, metadata save, lyrics drawer | automated |
| Search the library | `scripts/browser-smoke.mjs` | automated |
| Change profile, appearance, playback, close behaviour and keep them | `scripts/browser-smoke.mjs` — settings tabs, reload, values kept | automated |
| See who is streaming from this Mac | `scripts/browser-smoke.mjs` — Streaming activity, failed refresh keeps the last reading | automated |
| Sign in from a phone app (Subsonic) | `internal/server/server_test.go` (Subsonic sign-in, browse, stream), `scripts/native-rpc-smoke.py` | automated (protocol) / live-pending (real phone) |
| Add, start and remove a second profile; music is kept | `internal/desktop` manager tests, `scripts/browser-smoke.mjs` (two profiles) | automated |
| Check for updates and hand off to Studio Launcher KH | `internal/server/server_test.go` (update endpoints), `scripts/browser-smoke.mjs` | automated |
| Take an update hold, and carry on after it | `internal/downloads/hold_test.go`, `internal/server/maintenance_test.go` | automated |

## 2. Capability cross-check

Live sources: `internal/downloads/queue.go` (claim path), `internal/server/server.go` (routes),
`ui/src/*.tsx` (controls).

| Capability | Declared where | Control on screen | Exception (why) |
| --- | --- | --- | --- |
| New download claims | `POST /api/downloads`, worker `next()` | Downloads → paste link | — |
| Update hold (POST hold) | `POST /api/maintenance/hold` | none on purpose: Studio Launcher KH calls it | It is a loopback installer contract, not a user action |
| Update hold resume | `DELETE /api/maintenance/hold` | none on purpose | The pause expires by itself; the app resumes with no step |
| Cancel / retry a job | `POST /api/downloads/{id}/cancel`, `/retry` | Downloads row buttons | — |
| Clear finished, restore | `POST /api/downloads/clear-finished` | Downloads footer | — |
| Streaming quality, LAN off by default | `PUT /api/settings` | Settings → Playback, Streaming | — |
| Phone access | `PUT /api/settings` (LAN) | Settings → Streaming | Off by default until the owner turns it on |

## 3. Standard-practice comparison

Reference product: a desktop music player with a phone-facing server (Music.app + Navidrome).

| Standard control | Present | Deliberately absent (reason) | Missing (finding) |
| --- | --- | --- | --- |
| Import by file and by link | yes | — | — |
| Persistent queue with cancel/retry after a restart | yes | — | — |
| Phone clients on the same Wi-Fi | yes (Subsonic subset) | — | — |
| Updates that never interrupt a running job | yes (update hold) | an in-app updater — Studio Launcher KH stays the only installer | — |
| Multiple libraries/profiles at once | yes | — | — |

## 4. Reviewer

- Reviewer: the lead of the release that adds this file (Music Studio KH 0.3.11)
- Ran the task list: the rows cite tests the release gate runs
- Confirmed the capability cross-check: yes, against the routes registered in `internal/server/server.go`
- Live-pending rows are listed in [RELEASE-CHECKLIST.md](RELEASE-CHECKLIST.md) — a real phone, a
  real YouTube link, and signed/notarized installers are the owner's gates.
