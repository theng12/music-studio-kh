# Execution ledger — 2026-10-12-foundation.md

User explicitly requested verification and starting the build; implement the runnable foundation inline.
Repository: theng12/tuneharbor, private, write access verified. Approved docs published to main.
Branch: feat/tuneharbor-foundation. Existing checkout is dedicated to this new project; use a feature branch in place rather than request additional worktree approval.
Pre-flight: profile → library → server → UI interfaces match the plan.
Ruling: use an embedded lightweight browser player for the first executable milestone; React/Tauri remains planned for the desktop release. Cost: porting UI components later.
Ruling: first Subsonic implementation supports password authentication, not token/salt; encrypted compatibility credentials require desktop OS keychain integration. Cost: some mobile clients cannot connect until the next milestone; document explicitly.
Task 1 complete: profile test suite watched fail with not-implemented result, then PASS.
Task 2 complete: library test suite watched fail with not-implemented result, then PASS; containment, missing-folder preservation, simultaneous imports and artwork validation covered.
Task 3: core HTTP/session/stream/protocol tests watched fail against missing routes, then PASS. Successful Subsonic requests no longer exhaust failed-login throttling (regression RED→GREEN).
Task 4: embedded browser player and CLI implemented. Browser tests PASS for uppercase login with six-digit password, imports, actual playback, search, artwork, preferences, shared-library servers, session/artwork isolation and mobile logout.
User steering: allow minimum six-character numeric passwords and case-insensitive usernames. Added failing tests, implemented, Go/browser suites PASS. Unicode password minimum counts characters rather than bytes (RED→GREEN).
Verification harness fixes: replace corrupt PNG fixture with CRC-valid PNG; use an explicit Playwright context to test multiple pages sharing cookies. No product code changed for these harness failures.
Cross-platform compilation and race checks remain in progress; native Windows/macOS runtime and installers remain unverified.

Independent review: two Important findings — path-replaced library roots could escape containment; empty replacement roots could erase the index. Added three regression tests, watched all fail, implemented retained os.Root handles plus folder identity and persistent marker, library suite PASS. Existing indexed profiles now stay online while disconnected and resume on reconnect, including restart with an unavailable mountpoint.
Deferred minor: browser smoke executable path is Unix-only; documented explicitly, Windows uses Go runtime CI.
An initial race run failed because production imports changed while compilation was in flight. A fresh stable-source race run passed all packages; rerun again after the root fixes before declaring final verification.
