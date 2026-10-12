#!/usr/bin/env bash
set -euo pipefail
openbox > /tmp/tuneharbor-windowmanager.log 2>&1 &
WINDOWMANAGER_PID=$!
dbus-update-activation-environment DISPLAY XAUTHORITY
mkdir -p /home/tuneharbor-user/Music
printf 'fixture-keyring-password' | gnome-keyring-daemon --unlock --components=secrets > /tmp/tuneharbor-keyring.log 2>&1
APP_BINARY=/workspace/tunestead/src-tauri/target/debug/tuneharbor
"$APP_BINARY" > /tmp/tuneharbor-native.log 2>&1 &
APP_PID=$!
trap 'kill "$APP_PID" 2>/dev/null || true' EXIT
for try in $(seq 1 50); do
 if xdotool search --onlyvisible --name '^TuneHarbor$' > /tmp/tuneharbor-window-id 2>/dev/null; then break; fi
 sleep 0.2
done
WINDOW_ID=$(head -1 /tmp/tuneharbor-window-id)
xdotool windowactivate --sync "$WINDOW_ID" || true
sleep 2
import -window root /workspace/tunestead/artifacts/tuneharbor-native.png
# Close should hide, retaining the owned sidecar. Process death closes IPC and
# must trigger the supervisor's graceful shutdown; actual tray Quit is tested separately.
xdotool key --window "$WINDOW_ID" alt+F4
sleep 1
kill -0 "$APP_PID"
pgrep -u 1000 -f 'tuneharbor-server.*desktop' > /tmp/tuneharbor-sidecar-pid
kill "$APP_PID"
wait "$APP_PID" || true
for try in $(seq 1 50); do if ! pgrep -u 1000 -f 'tuneharbor-server.*desktop' >/dev/null; then echo 'PASS: native window launched, close-to-tray kept sidecar alive, parent exit stopped sidecar';exit 0;fi;sleep 0.2;done
exit 1
