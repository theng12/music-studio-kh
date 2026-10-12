#!/usr/bin/env bash
# Linux desktop end-to-end check for an installed TuneHarbor package.
# Requires: xvfb-run, dbus-run-session, gnome-keyring, openbox, WebKitWebDriver, tauri-driver, node.
set -euo pipefail
cd "$(dirname "$0")/.."
if [ "${TUNEHARBOR_E2E_INNER:-}" != 1 ]; then
  export TUNEHARBOR_E2E_INNER=1
  exec xvfb-run -a -s "-screen 0 1440x960x24" dbus-run-session -- "$0" "$@"
fi
# Isolate profile data and keyring from the user running the check.
TEMP_HOME=$(mktemp -d)
export XDG_CONFIG_HOME="$TEMP_HOME/config" XDG_DATA_HOME="$TEMP_HOME/data" XDG_CACHE_HOME="$TEMP_HOME/cache"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME"
export WEBKIT_DISABLE_COMPOSITING_MODE=1
openbox > "$TEMP_HOME/openbox.log" 2>&1 &
eval "$(printf 'tuneharbor-e2e' | gnome-keyring-daemon --unlock --components=secrets)"
export GNOME_KEYRING_CONTROL
tauri-driver > "$TEMP_HOME/tauri-driver.log" 2>&1 &
DRIVER_PID=$!
cleanup() {
  status=$?
  kill "$DRIVER_PID" 2>/dev/null || true
  if [ "$status" -ne 0 ]; then echo "--- tauri-driver log"; tail -50 "$TEMP_HOME/tauri-driver.log"; fi
  rm -rf "$TEMP_HOME"
}
trap cleanup EXIT
for _ in $(seq 1 50); do curl -fsS http://127.0.0.1:4444/status > /dev/null 2>&1 && break; sleep 0.2; done
python3 scripts/native-rpc-smoke.py
node scripts/desktop-e2e.mjs
