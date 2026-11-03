# Windows and Linux desktop pilot

Use the existing **Desktop installers (unsigned development)** workflow with `windows_linux_only=true` to build Windows x86-64 and Linux x86-64 from the same commit. It uses standard GitHub-hosted runners in this public repository, with no signing credentials, new account or paid builder. It does not create a release or publish an update feed.

```bash
gh workflow run desktop.yml --ref YOUR_BRANCH -f windows_linux_only=true
```

Leave `windows_only` false for this pilot. The existing `windows_only=true` mode takes precedence if both inputs are set. With both false, the normal four-platform matrix, including both Mac targets, is unchanged.

The workflow builds checksum-verified runtime tools, the Go sidecar, the existing React UI and the Tauri shell. Windows produces a per-user NSIS installer; Linux produces Debian and AppImage packages. Download the `Music Studio KH-unsigned-dev-windows-x64` and `Music Studio KH-unsigned-dev-linux-x64` artifacts from the workflow run. Artifacts expire after 14 days, so retain the pilot copies with the delivery report.

Windows installer smoke runs the NSIS install/uninstall and packaged FFmpeg on a standard Windows runner. That alone is **build-only, not runtime-tested for the Music GUI**; it does not prove the app window, WebView2, tray, keychain or playback.

Linux installer smoke checks the payload and dependency installation. The subsequent desktop end-to-end lane launches the installed app under Xvfb/Openbox with WebKitGTK and an isolated keyring/profile. Report **runtime-tested on that virtual Ubuntu runner** only if this lane actually runs and passes. AppImage generation alone does not prove AppImage startup or compatibility with every Linux distribution.

The delivery report records each target as runtime-tested, build-only, or blocked, with the run URL, exact source commit, real exit status, artifacts and remaining limitations. A queued, skipped or billing-blocked job is not a build or runtime pass.

```bash
python3 scripts/test-desktop-matrix.py
```
