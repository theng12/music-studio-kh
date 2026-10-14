#!/usr/bin/env bash
# Build TuneHarbor.app on this Mac and install it in /Applications.
# For personal use: it bundles Homebrew's FFmpeg instead of the LGPL build
# that release installers use, so do not redistribute the result.
#
# Run from the project folder:  bash scripts/build-mac.sh
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
fail() { printf '\n\033[31mStopped: %s\033[0m\n' "$1" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || fail "this script builds the macOS app; run it on a Mac."
case "$(uname -m)" in
  arm64) runtime=macos-arm64 ;;
  x86_64) runtime=macos-x64 ;;
  *) fail "unsupported Mac processor: $(uname -m)" ;;
esac

step "Checking Apple's command line tools"
if ! xcode-select -p >/dev/null 2>&1; then
  xcode-select --install || true
  fail "a window asks to install Apple's command line tools. Install them, then run this script again."
fi

step "Checking Homebrew"
if ! command -v brew >/dev/null 2>&1; then
  for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew; do
    [ -x "$candidate" ] && eval "$("$candidate" shellenv)" && break
  done
fi
if ! command -v brew >/dev/null 2>&1; then
  fail "Homebrew is not installed. Install it from https://brew.sh (one command), then run this script again."
fi

step "Installing build tools (Go, Node.js, Rust, Python, FFmpeg). The first time can take several minutes."
brew install go node rust python ffmpeg
# Homebrew's Python has current certificates for the verified downloads below.
python="$(brew --prefix python)/libexec/bin/python3"
[ -x "$python" ] || python="$(brew --prefix)/bin/python3"

step "Downloading checksum-verified yt-dlp and Deno"
rm -rf src-tauri/runtime-tools
"$python" scripts/prepare-runtime.py --target "$runtime" --local --ffmpeg-dir "$(brew --prefix ffmpeg)/bin"

step "Building the interface"
npm ci --no-audit --no-fund
npm run build:web

step "Building the music server"
triple="$(rustc -vV | sed -n 's/^host: //p')"
mkdir -p src-tauri/binaries
# Go downloads the toolchain version go.mod asks for if Homebrew's is older.
GOTOOLCHAIN=auto CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "src-tauri/binaries/tuneharbor-server-$triple" ./cmd/tuneharbor

step "Building TuneHarbor.app (the first Rust build takes a few minutes)"
npm run build:desktop -- --bundles app
app="src-tauri/target/release/bundle/macos/TuneHarbor.app"
[ -d "$app" ] || fail "the app bundle was not produced; see the messages above."

step "Installing into /Applications"
osascript -e 'tell application "TuneHarbor" to quit' >/dev/null 2>&1 || true
sleep 2
rm -rf /Applications/TuneHarbor.app
cp -R "$app" /Applications/
printf '\n\033[32mDone. Open TuneHarbor from Applications or Launchpad.\033[0m\n'
open /Applications/TuneHarbor.app
