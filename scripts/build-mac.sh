#!/usr/bin/env bash
# Build portable locally signed Music Studio KH .app and DMG under artifacts/.
#
# Usage:
#   bash scripts/build-mac.sh          # Full build
#   bash scripts/build-mac.sh --check  # Quick preflight / portability validation
#   bash scripts/build-mac.sh --retain-accepted /absolute/accepted-pins.json
#
# Launching helpers such as run-detached.sh may ignore the shebang and start this with
# zsh; re-exec under bash so its arrays and bash-only syntax keep working.
[ -n "${BASH_VERSION:-}" ] || exec bash "$0" "$@"
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
fail() { printf '\n\033[31mStopped: %s\033[0m\n' "$1" >&2; exit 1; }

# Only the post-install acceptance caller supplies these trusted pins; builds never promote themselves.
if [ "${1:-}" = "--retain-accepted" ]; then
  [ "$#" -eq 2 ] || fail "usage: build-mac.sh --retain-accepted /absolute/accepted-pins.json"
  helper="${RELEASE_DIST_HELPER:-${HOME}/Developer/Shared Tools/portfolio/tools/release-dist.py}"
  exec python3 -I -B "$helper" "$2"
fi

# Protecting work: never rm. Move to trash preserving directory structure.
retire() {
  local target="$1"
  if [ -e "$target" ] || [ -L "$target" ]; then
    local trash_root="${HOME}/Desktop/_trash/music-studio-kh/$(date +%Y-%m-%d)"
    local timestamp
    timestamp="$(date +%Y%m%d-%H%M%S)"
    local rel_path="${target#/}"
    rel_path="${rel_path#./}"
    local dest="$trash_root/$rel_path.$timestamp"
    mkdir -p "$(dirname "$dest")"
    mv "$target" "$dest"
    printf 'Retired %s -> %s\n' "$target" "$dest"
  fi
}

[ "$(uname -s)" = Darwin ] || fail "this script builds the macOS app; run it on macOS."
host_arch="$(uname -m)"
case "$host_arch" in
  arm64) runtime="macos-arm64"; dmg_arch="arm64" ;;
  x86_64) runtime="macos-x64"; dmg_arch="x86_64" ;;
  *) fail "unsupported Mac processor: $host_arch" ;;
esac

# Ensure standard tool paths are on PATH
for p in /opt/homebrew/bin /opt/homebrew/opt/rustup/bin "${HOME}/.cargo/bin" /usr/local/bin; do
  [ -d "$p" ] && [[ ":$PATH:" != *":$p:"* ]] && export PATH="$p:$PATH"
done

step "Checking required developer tools"
if ! xcode-select -p >/dev/null 2>&1; then
  fail "Xcode command line tools are missing. Run 'xcode-select --install' and retry."
fi

require_tool() {
  local cmd="$1"
  local help="$2"
  command -v "$cmd" >/dev/null 2>&1 || fail "Required tool '$cmd' is missing. $help"
}

require_tool go "Install Go from https://go.dev/dl/ or via package manager."
require_tool node "Install Node.js (v20+) from https://nodejs.org/."
require_tool npm "npm is bundled with Node.js."
require_tool rustc "Install Rust via rustup (https://rustup.rs)."
require_tool cargo "cargo is bundled with Rust."
require_tool python3 "Install Python 3 (https://python.org)."
require_tool make "Install make via Xcode command line tools or package manager."
require_tool pkg-config "Install pkg-config via package manager."
require_tool lipo "lipo is part of Xcode command line tools."
require_tool hdiutil "hdiutil is a built-in macOS tool."
require_tool codesign "codesign is part of macOS command line tools."
require_tool otool "otool is part of macOS command line tools."
require_tool vtool "vtool is part of Xcode command line tools."

step "Checking the existing local Keychain identity (no imports or trust changes)"
[ -z "${APPLE_CERTIFICATE:-}" ] && [ -z "${APPLE_CERTIFICATE_PASSWORD:-}" ] || fail "use the existing Keychain identity, not certificate imports"
[ -z "${APPLE_SIGNING_IDENTITY:-}" ] || [ "$APPLE_SIGNING_IDENTITY" = "KH Studio Local Signing" ] || fail "preserve the explicitly selected identity; review the local signing configuration first"
security find-identity -p codesigning | grep -F '"KH Studio Local Signing"' >/dev/null || fail "KH Studio Local Signing certificate/private-key pair is missing; do not create a replacement"

step "Reading project configuration and verifying version consistency"
[ -f src-tauri/tauri.conf.json ] || fail "src-tauri/tauri.conf.json not found."

product_name="$(python3 -c "import json; print(json.load(open('src-tauri/tauri.conf.json')).get('productName', 'Music Studio KH'))")"
version="$(python3 -c "import json; print(json.load(open('src-tauri/tauri.conf.json')).get('version', ''))")"
bundle_id="$(python3 -c "import json; print(json.load(open('src-tauri/tauri.conf.json')).get('identifier', ''))")"

[ -n "$version" ] || fail "could not read version from src-tauri/tauri.conf.json"
[ "$bundle_id" = "app.tuneharbor.desktop" ] || fail "expected bundle identifier app.tuneharbor.desktop, got $bundle_id"

cargo_ver="$(sed -n 's/^version = "\(.*\)"/\1/p' src-tauri/Cargo.toml | head -n1)"
pkg_ver="$(python3 -c "import json; print(json.load(open('package.json')).get('version', ''))" 2>/dev/null || true)"

printf 'Config version:    %s\n' "$version"
printf 'Product name:      %s\n' "$product_name"
printf 'Bundle identifier: %s\n' "$bundle_id"
printf 'Cargo.toml:        %s\n' "$cargo_ver"
printf 'package.json:      %s\n' "$pkg_ver"

[ "$cargo_ver" = "$version" ] || fail "Cargo.toml version ($cargo_ver) does not match config version ($version)"
[ "$pkg_ver" = "$version" ] || fail "package.json version ($pkg_ver) does not match config version ($version)"

[ -f CHANGELOG.md ] || fail "CHANGELOG.md not found"
top_changelog_ver="$(awk '/^## / { print $2; exit }' CHANGELOG.md)"
[ -n "$top_changelog_ver" ] || fail "could not parse top version from CHANGELOG.md"
[ "$top_changelog_ver" = "$version" ] || fail "CHANGELOG.md top version ($top_changelog_ver) does not match config version ($version)"
printf 'CHANGELOG.md top:  %s\n' "$top_changelog_ver"

check_native_ffmpeg() {
  local dir="$1"
  [ -d "$dir" ] || return 1
  [ -x "$dir/ffmpeg" ] || return 1
  [ -x "$dir/ffprobe" ] || return 1
  "$dir/ffmpeg" -version >/dev/null 2>&1 || return 1
  "$dir/ffprobe" -version >/dev/null 2>&1 || return 1

  # Check config.h for LGPL flags
  local cfg="$dir/notices/ffmpeg-build-config.h"
  [ -f "$cfg" ] || return 1
  grep -q '#define CONFIG_GPL 0' "$cfg" || return 1
  grep -q '#define CONFIG_NONFREE 0' "$cfg" || return 1
  grep -q '#define CONFIG_SHARED 0' "$cfg" || return 1
  grep -q '#define CONFIG_STATIC 1' "$cfg" || return 1
  grep -q '#define CONFIG_LIBMP3LAME 1' "$cfg" || return 1
  grep -q '#define CONFIG_NETWORK 0' "$cfg" || return 1
  grep -q '#define CONFIG_ZLIB 1' "$cfg" || return 1
  "$dir/ffmpeg" -hide_banner -encoders 2>/dev/null | grep ' png ' >/dev/null || return 1

  # Check host arch, dylibs, and Mach-O minos <= 13.0
  for bin in "$dir/ffmpeg" "$dir/ffprobe"; do
    local bin_arch
    bin_arch="$(lipo -archs "$bin" 2>/dev/null || true)"
    [ "$bin_arch" = "$host_arch" ] || return 1

    local bad_dylibs
    bad_dylibs="$(otool -L "$bin" 2>/dev/null | awk 'NR>1 {print $1}' | grep -v -E '^/(System/Library/|usr/lib/)' || true)"
    [ -z "$bad_dylibs" ] || return 1

    local minos
    minos="$(vtool -show-build "$bin" 2>/dev/null | awk '$1 == "minos" {print $2}' || true)"
    [ -n "$minos" ] || return 1
    python3 -c "import sys; v=sys.argv[1].split('.'); sys.exit(0 if (int(v[0]), int(v[1]) if len(v)>1 else 0) <= (13, 0) else 1)" "$minos" || return 1
  done
  return 0
}

# Preflight / self-check mode: strictly tests tool requirements, version consistency, trash helper, and cache status
if [ "${1:-}" = "--check" ] || [ "${1:-}" = "--dry-run" ]; then
  step "Self-check mode: testing preflight validations (no installs or builds)"
  test_tmp="$(mktemp /tmp/music-studio-kh-check.XXXXXX)"
  touch "$test_tmp"
  retire "$test_tmp"
  printf 'Trash retirement helper: verified\n'

  if check_native_ffmpeg "artifacts/native-ffmpeg" >/dev/null 2>&1; then
    printf 'Native FFmpeg cache: valid and portable (minos <= 13.0, %s, LGPL flags)\n' "$host_arch"
  else
    printf 'Native FFmpeg cache: invalid or minos > 13.0 (will be automatically retired and rebuilt on full run)\n'
  fi

  printf '\n\033[32mPreflight check passed. All versions match %s. Ready to build.\033[0m\n' "$version"
  exit 0
fi

step "Validating native LGPL FFmpeg/ffprobe"
mkdir -p artifacts/native-ffmpeg
if check_native_ffmpeg "artifacts/native-ffmpeg"; then
  step "Reusing verified portable native FFmpeg from artifacts/native-ffmpeg"
else
  if [ -e "artifacts/native-ffmpeg/ffmpeg" ]; then
    step "Retiring incompatible or non-portable native FFmpeg build to trash"
    retire "artifacts/native-ffmpeg"
  fi
  step "Preparing verified source archives for static LGPL FFmpeg and LAME"
  python3 scripts/prepare-runtime.py --target "$runtime" --sources-only
  ffmpeg_ver="$(python3 -c "import json; print(json.load(open('packaging/runtime-tools.json'))['ffmpeg']['version'])")"
  lame_ver="$(python3 -c "import json; print(json.load(open('packaging/runtime-tools.json'))['lame']['version'])")"
  step "Building portable static LGPL FFmpeg and LAME (MACOSX_DEPLOYMENT_TARGET=13.0)"
  bash scripts/build-ffmpeg.sh \
    "$PWD/artifacts/tool-cache/ffmpeg-${ffmpeg_ver}.tar.gz" \
    "$PWD/artifacts/tool-cache/lame-${lame_ver}.tar.gz" \
    "$PWD/artifacts/native-ffmpeg"
  check_native_ffmpeg "artifacts/native-ffmpeg" || fail "built FFmpeg failed portability checks"
fi

step "Preparing verified runtime tools (yt-dlp, Deno, LGPL FFmpeg)"
for directory in src-tauri/runtime-tools src-tauri/runtime-tools-macos-bundle; do
  if [ -d "$directory" ]; then
    retire "$directory"
  fi
done
python3 scripts/prepare-runtime.py --target "$runtime" --ffmpeg-dir "artifacts/native-ffmpeg"
for tool in ffmpeg ffprobe yt-dlp deno; do
  [ -x "src-tauri/runtime-tools/$tool" ] || fail "runtime tool missing or not executable: src-tauri/runtime-tools/$tool"
done
python3 scripts/audio-tools-smoke.py "$PWD/src-tauri/runtime-tools"

step "Building frontend web assets"
npm ci --no-audit --no-fund
npm run build:web

step "Building music server sidecar"
triple="$(rustc -vV | sed -n 's/^host: //p')"
mkdir -p src-tauri/binaries
sidecar_bin="src-tauri/binaries/music-studio-kh-server-$triple"
if [ -e "$sidecar_bin" ]; then
  retire "$sidecar_bin"
fi
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/theng12/music-studio-kh/internal/server.appVersion=$version" -o "$sidecar_bin" ./cmd/music-studio-kh
chmod 0755 "$sidecar_bin"

# externalBin makes Tauri sign each runtime before sealing the outer bundle.
for tool in ffmpeg ffprobe yt-dlp deno; do
  cp "src-tauri/runtime-tools/$tool" "src-tauri/binaries/$tool-$triple"
done

step "Staging the music server as Music Studio KH Helper.app"
# The server runs as a proper helper app so Activity Monitor shows the app's own name,
# never a bare binary. macOS only: Windows and Linux keep the sidecar layout.
helper_stage="src-tauri/helper-app/Music Studio KH Helper.app"
retire "src-tauri/helper-app"
mkdir -p "$helper_stage/Contents/MacOS" "$helper_stage/Contents/Resources"
cp "$sidecar_bin" "$helper_stage/Contents/MacOS/Music Studio KH Helper"
chmod 0755 "$helper_stage/Contents/MacOS/Music Studio KH Helper"
cp src-tauri/icons/icon.icns "$helper_stage/Contents/Resources/icon.icns"
python3 - "$helper_stage/Contents/Info.plist" "$version" <<'PY'
import plistlib, sys
plist = {
    "CFBundleName": "Music Studio KH Helper",
    "CFBundleDisplayName": "Music Studio KH Helper",
    "CFBundleIdentifier": "app.tuneharbor.desktop.helper",
    "CFBundleExecutable": "Music Studio KH Helper",
    "CFBundlePackageType": "APPL",
    "CFBundleShortVersionString": sys.argv[2],
    "CFBundleVersion": sys.argv[2],
    "CFBundleIconFile": "icon.icns",
    "LSUIElement": True,
    "NSHighResolutionCapable": True,
}
with open(sys.argv[1], "wb") as stream:
    plistlib.dump(plist, stream)
PY
codesign --force --sign "KH Studio Local Signing" "$helper_stage/Contents/MacOS/Music Studio KH Helper"
codesign --force --sign "KH Studio Local Signing" "$helper_stage"
codesign --verify --strict --verbose=2 "$helper_stage"

step "Collecting dependency notices"
python3 scripts/collect-notices.py

step "Building .app bundle via Tauri"
npm run build:desktop -- --config src-tauri/tauri.macos-local.conf.json --bundles app

app="src-tauri/target/release/bundle/macos/${product_name}.app"
if [ ! -d "$app" ]; then
  app="src-tauri/target/${triple}/release/bundle/macos/${product_name}.app"
fi
[ -d "$app" ] || fail "the .app bundle was not produced at expected path: $app"
printf 'Found app bundle: %s\n' "$app"

validate_plist() {
  local plist="$1"
  [ -f "$plist" ] || fail "Info.plist missing at $plist"
  python3 -c "import plistlib, sys; p = plistlib.load(open(sys.argv[1], 'rb')); assert p.get('CFBundleIdentifier') == sys.argv[2], f'ID mismatch: {p.get(\"CFBundleIdentifier\")} != {sys.argv[2]}'; assert p.get('CFBundleShortVersionString') == sys.argv[3], f'Version mismatch: {p.get(\"CFBundleShortVersionString\")} != {sys.argv[3]}'; assert p.get('CFBundleName') == sys.argv[4], f'Name mismatch: {p.get(\"CFBundleName\")} != {sys.argv[4]}'" \
    "$plist" "$bundle_id" "$version" "$product_name"
}

validate_plist "$app/Contents/Info.plist"

step "Embedding Sparkle and signing nested code first"
fw_src="src-tauri/vendor/khsparkle/Sparkle.framework"
[ -d "$fw_src" ] || fail "the vendored Sparkle.framework is missing (src-tauri/vendor/khsparkle/)"
mkdir -p "$app/Contents/Frameworks"
cp -R "$fw_src" "$app/Contents/Frameworks/" || fail "could not copy the vendored Sparkle.framework into Contents/Frameworks"
# Nested code first, the app last (INTEGRATION.md §3A). A nested binary signed after
# the app is what breaks `codesign --verify --deep --strict`.
sign_id="KH Studio Local Signing"
fw="$app/Contents/Frameworks/Sparkle.framework/Versions/B"
for nested in "$fw/XPCServices/Downloader.xpc" "$fw/XPCServices/Installer.xpc" "$fw/Updater.app" "$fw/Autoupdate"; do
  [ -e "$nested" ] || fail "the vendored Sparkle framework has no $(basename "$nested")"
  codesign --force --sign "$sign_id" --timestamp=none "$nested" || fail "could not sign $(basename "$nested")"
done
codesign --force --sign "$sign_id" --timestamp=none "$fw" || fail "could not sign the Sparkle framework"
app_exe="$(/usr/libexec/PlistBuddy -c "Print :CFBundleExecutable" "$app/Contents/Info.plist")"
[ -x "$app/Contents/MacOS/$app_exe" ] || fail "app executable $app_exe not found"
codesign --force --sign "$sign_id" --timestamp=none "$app/Contents/MacOS/$app_exe" || fail "could not sign the app binary"
codesign --force --sign "$sign_id" --timestamp=none "$app" || fail "could not sign the app"
codesign --verify --deep --strict --verbose=2 "$app" || fail "the app's signature is broken after embedding Sparkle"
otool -l "$app/Contents/MacOS/$app_exe" | grep -q '@executable_path/../Frameworks' || fail "the binary carries no @executable_path/../Frameworks rpath"
python3 - "$app/Contents/Info.plist" <<'PY'
import plistlib, sys
p = plistlib.load(open(sys.argv[1], "rb"))
assert p.get("SUFeedURL") == "https://theng12.github.io/music-studio-kh/appcast.xml", "SUFeedURL missing"
assert len(p.get("SUPublicEDKey", "")) == 44, "SUPublicEDKey missing or malformed"
assert p.get("KHHoldURL") == "http://127.0.0.1:8645/api/maintenance/hold", "KHHoldURL missing"
assert p.get("KHHoldTokenFile") == "~/Library/KHUpdater/state/hold-token", "KHHoldTokenFile missing"
print("Sparkle keys present: SUFeedURL, SUPublicEDKey, KHHoldURL, KHHoldTokenFile")
PY

step "Verifying packaged signatures (helper app + packager binaries)"
exe_name="$(/usr/libexec/PlistBuddy -c "Print :CFBundleExecutable" "$app/Contents/Info.plist")"
[ -n "$exe_name" ] || fail "could not read CFBundleExecutable from $app/Contents/Info.plist"
[ -x "$app/Contents/MacOS/$exe_name" ] || fail "executable $app/Contents/MacOS/$exe_name not found or not executable"
helper_app="$app/Contents/Helpers/Music Studio KH Helper.app"
helper_exe="$helper_app/Contents/MacOS/Music Studio KH Helper"
[ -x "$helper_exe" ] || fail "helper executable $helper_exe not found or not executable"
[ "$(/usr/libexec/PlistBuddy -c "Print :CFBundleDisplayName" "$helper_app/Contents/Info.plist" 2>/dev/null)" = "Music Studio KH Helper" ] || fail "helper CFBundleDisplayName is missing"
[ "$(/usr/libexec/PlistBuddy -c "Print :LSUIElement" "$helper_app/Contents/Info.plist" 2>/dev/null)" = "true" ] || fail "helper LSUIElement is not true"

for tool in ffmpeg ffprobe yt-dlp deno; do
  codesign --verify --strict --verbose=2 "$app/Contents/MacOS/$tool"
done
codesign --verify --strict --verbose=2 "$helper_exe"
codesign --verify --strict --verbose=2 "$helper_app"
[ ! -e "$app/Contents/MacOS/music-studio-kh-server" ] || fail "bare music-studio-kh-server must not ship beside the helper app"
codesign --verify --deep --strict --verbose=2 "$app"
python3 -I scripts/test-ytdlp-startup.py "$app/Contents/MacOS/yt-dlp" --budget-seconds 15 --receipt "artifacts/yt-dlp-startup-${version}.json"
python3 -I scripts/verify-macos-signing.py --app "$app" --receipt "artifacts/signing-${version}.json"

step "Building the DMG from the signed bundle"
# The DMG is made here, not by the packager, because the bundle now carries Sparkle
# and must be signed (nested first, app last) before it is sealed into the image.
mkdir -p artifacts
dmg_path="artifacts/${product_name}-${version}.dmg"
[ ! -e "$dmg_path" ] || fail "artifact already exists; preserve it and choose a fresh release version: $dmg_path"
dmg_stage="$(mktemp -d /tmp/music-dmg-src.XXXXXX)"
cp -R "$app" "$dmg_stage/${product_name}.app"
hdiutil create -volname "${product_name} ${version}" -srcfolder "$dmg_stage" -format UDZO -ov "$dmg_path" >/dev/null \
  || { retire "$dmg_stage"; fail "hdiutil could not build the DMG"; }
retire "$dmg_stage"
codesign --verify --verbose=2 "$dmg_path"

step "Verifying DMG payload"
verify_mount="$(mktemp -d /tmp/dmg-verify.XXXXXX)"
hdiutil attach -readonly -nobrowse -mountpoint "$verify_mount" "$dmg_path"
mounted_app="$verify_mount/${product_name}.app"
[ -d "$mounted_app" ] || { hdiutil detach "$verify_mount"; retire "$verify_mount"; fail "DMG does not contain ${product_name}.app"; }
validate_plist "$mounted_app/Contents/Info.plist"
codesign --verify --deep --strict --verbose=2 "$mounted_app"

step "Scanning the shipped files for owner build paths"
# Rust embeds absolute source paths unless they are remapped (scripts/build-desktop.mjs
# does that). Read the DMG payload raw: grep -a on bytes, never strings, which skips
# local symbol names and has silently passed a leaking build before.
# The cargo-registry token is checked in our own binary only: bundled third-party
# tools (deno) carry it as their own build string and reveal nothing of ours.
owner_markers=("$HOME" "_orchestra")
leaks=""
files_checked=0
while IFS= read -r -d '' file; do
  files_checked=$((files_checked + 1))
  grep -aqF -e "${owner_markers[0]}" -e "${owner_markers[1]}" "$file" || continue
  for marker in "${owner_markers[@]}"; do
    if grep -aqF -- "$marker" "$file"; then
      leaks="${leaks}  ${marker} in ${file#"$verify_mount"/}"$'\n'
    fi
  done
done < <(find "$verify_mount" -type f -print0)
if grep -aqF -- ".cargo/registry" "$mounted_app/Contents/MacOS/$exe_name"; then
  leaks="${leaks}  .cargo/registry in Contents/MacOS/$exe_name"$'\n'
fi
printf 'Checked %s files in the shipped app.\n' "$files_checked"
hdiutil detach "$verify_mount"
retire "$verify_mount"
[ -z "$leaks" ] || fail "owner build paths found in the shipped app:
$leaks"
printf 'No owner paths in the shipped app: %s absent\n' "$(printf '%s / ' "${owner_markers[@]}" | sed 's/ \/ $//')"

dest_app="artifacts/${product_name}.app"
if [ -e "$dest_app" ]; then
  retire "$dest_app"
fi
cp -R "$app" "$dest_app"

step "Build complete"
dmg_checksum="$(shasum -a 256 "$dmg_path" | awk '{print $1}')"
printf '\n\033[32mPortable macOS release ready:\033[0m\n'
printf '  App bundle:       %s\n' "$dest_app"
printf '  DMG artifact:     %s\n' "$dmg_path"
printf '  DMG SHA-256:      %s\n' "$dmg_checksum"
printf '  Bundle ID:        %s\n' "$bundle_id"
printf '  Version:          %s\n' "$version"
printf '  Architecture:     %s\n' "$dmg_arch"
printf '  Minimum macOS:    13.0 (Ventura)\n\n'
