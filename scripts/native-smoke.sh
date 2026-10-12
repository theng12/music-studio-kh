#!/usr/bin/env bash
set -euo pipefail
# Software compositing is required in the virtual CI desktop.
export WEBKIT_DISABLE_COMPOSITING_MODE=1
openbox > /tmp/music-studio-kh-windowmanager.log 2>&1 &
WINDOWMANAGER_PID=$!
dbus-update-activation-environment DISPLAY XAUTHORITY
mkdir -p /home/music-studio-kh-user/Music
printf 'fixture-keyring-password' | gnome-keyring-daemon --unlock --components=secrets > /tmp/music-studio-kh-keyring.log 2>&1
python3 /workspace/tunestead/scripts/native-rpc-smoke.py
APP_BINARY=/workspace/tunestead/src-tauri/target/debug/music-studio-kh
"$APP_BINARY" > /tmp/music-studio-kh-native.log 2>&1 &
APP_PID=$!
trap 'kill "$APP_PID" 2>/dev/null || true' EXIT
for try in $(seq 1 50); do
 if xdotool search --onlyvisible --name '^MusicStudioKH$' > /tmp/music-studio-kh-window-id 2>/dev/null; then break; fi
 sleep 0.2
done
WINDOW_ID=$(head -1 /tmp/music-studio-kh-window-id)
xdotool windowactivate --sync "$WINDOW_ID" || true
sleep 2
import -window root /workspace/tunestead/artifacts/music-studio-kh-native.png
# Close should hide, retaining the owned sidecar. Process death closes IPC and
# must trigger the supervisor's graceful shutdown; actual tray Quit is tested separately.
xdotool key --window "$WINDOW_ID" alt+F4
sleep 1
kill -0 "$APP_PID"
pgrep -u 1000 -f 'music-studio-kh-server.*desktop' > /tmp/music-studio-kh-sidecar-pid
kill "$APP_PID"
wait "$APP_PID" || true
for try in $(seq 1 50); do if ! pgrep -u 1000 -f 'music-studio-kh-server.*desktop' >/dev/null; then echo 'PASS: native window launched, close-to-tray kept sidecar alive, parent exit stopped sidecar';exit 0;fi;sleep 0.2;done
exit 1
