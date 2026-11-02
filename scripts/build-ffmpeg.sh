#!/usr/bin/env bash
set -euo pipefail
# Build native LGPL FFmpeg/ffprobe with MP3 support from verified source archives.
# Inputs are absolute paths to FFmpeg source archive, LAME archive and output dir.
FFMPEG_ARCHIVE="$1"
LAME_ARCHIVE="$2"
OUTPUT_DIR="$3"

retire() {
  local target="$1"
  if [ -e "$target" ] || [ -L "$target" ]; then
    local trash_root="${HOME}/Developer/_trash/tuneharbor"
    local timestamp
    timestamp="$(date +%Y%m%d-%H%M%S)"
    local rel_path="${target#/}"
    rel_path="${rel_path#./}"
    local dest="$trash_root/$rel_path.$timestamp"
    mkdir -p "$(dirname "$dest")"
    mv "$target" "$dest"
  fi
}

BUILD_DIR="$(mktemp -d /tmp/tuneharbor-ffmpeg-build.XXXXXX)"
trap 'retire "$BUILD_DIR"' EXIT

mkdir -p "$BUILD_DIR/lame" "$BUILD_DIR/ffmpeg" "$OUTPUT_DIR"
tar -xzf "$LAME_ARCHIVE" --strip-components=1 -C "$BUILD_DIR/lame"
tar -xzf "$FFMPEG_ARCHIVE" --strip-components=1 -C "$BUILD_DIR/ffmpeg"
PREFIX_DIR="$BUILD_DIR/prefix"

EXTRA_CFLAGS=""
EXTRA_LDFLAGS=""
if [ "$(uname -s)" = "Darwin" ]; then
  export MACOSX_DEPLOYMENT_TARGET=13.0
  EXTRA_CFLAGS="-mmacosx-version-min=13.0"
  EXTRA_LDFLAGS="-mmacosx-version-min=13.0"
fi

cd "$BUILD_DIR/lame"
if [ "$(uname -s)" = "Darwin" ]; then
  CFLAGS="${EXTRA_CFLAGS}" LDFLAGS="${EXTRA_LDFLAGS}" ./configure --prefix="$PREFIX_DIR" --disable-shared --enable-static --disable-frontend > "$BUILD_DIR/lame-config.log" 2>&1
else
  ./configure --prefix="$PREFIX_DIR" --disable-shared --enable-static --disable-frontend > "$BUILD_DIR/lame-config.log" 2>&1
fi
make -j2 > "$BUILD_DIR/lame-build.log" 2>&1
make install > /dev/null 2>&1

cd "$BUILD_DIR/ffmpeg"
STATIC_FLAGS=""
case "$(uname -s)" in MINGW*|MSYS*) STATIC_FLAGS="-static";; esac
PKG_CONFIG_PATH="$PREFIX_DIR/lib/pkgconfig" ./configure \
  --prefix="$PREFIX_DIR" \
  --disable-shared \
  --enable-static \
  --disable-autodetect \
  --disable-doc \
  --disable-debug \
  --disable-ffplay \
  --disable-network \
  --disable-x86asm \
  --enable-libmp3lame \
  --extra-cflags="-I$PREFIX_DIR/include ${EXTRA_CFLAGS}" \
  --extra-ldflags="-L$PREFIX_DIR/lib $STATIC_FLAGS ${EXTRA_LDFLAGS}" > "$BUILD_DIR/ffmpeg-config.log" 2>&1
make -j2 > "$BUILD_DIR/ffmpeg-build.log" 2>&1

# Copy only the executable products, retiring preexisting outputs first.
for name in ffmpeg ffprobe ffmpeg.exe ffprobe.exe; do
  if test -f "$name"; then
    if [ -e "$OUTPUT_DIR/$name" ]; then
      retire "$OUTPUT_DIR/$name"
    fi
    cp "$name" "$OUTPUT_DIR/$name"
    chmod 0755 "$OUTPUT_DIR/$name"
  fi
done

# Verify macOS Mach-O minos <= 13.0, host arch, system-only dylibs, and LGPL config
if [ "$(uname -s)" = "Darwin" ]; then
  host_arch="$(uname -m)"
  for name in ffmpeg ffprobe; do
    bin="$OUTPUT_DIR/$name"
    if [ -f "$bin" ]; then
      minos="$(vtool -show-build "$bin" 2>/dev/null | awk '$1 == "minos" {print $2}' || true)"
      [ -n "$minos" ] || { echo "ERROR: $name missing minos!" >&2; exit 1; }
      python3 -c "import sys; v=sys.argv[1].split('.'); sys.exit(0 if (int(v[0]), int(v[1]) if len(v)>1 else 0) <= (13, 0) else 1)" "$minos" || {
        echo "ERROR: Built $name has minos $minos > 13.0 target!" >&2
        exit 1
      }
      bin_arch="$(lipo -archs "$bin" 2>/dev/null || true)"
      [ "$bin_arch" = "$host_arch" ] || {
        echo "ERROR: Built $name arch ($bin_arch) does not match host arch ($host_arch)!" >&2
        exit 1
      }
      bad_dylibs="$(otool -L "$bin" 2>/dev/null | awk 'NR>1 {print $1}' | grep -v -E '^/(System/Library/|usr/lib/)' || true)"
      if [ -n "$bad_dylibs" ]; then
        echo "ERROR: Built $name links against non-system dylibs: $bad_dylibs" >&2
        exit 1
      fi
    fi
  done
  # Require LGPL config flags: GPL0 NONFREE0 SHARED0 STATIC1 LIBMP3LAME1 NETWORK0
  grep -q '#define CONFIG_GPL 0' config.h || { echo "ERROR: config.h missing CONFIG_GPL 0" >&2; exit 1; }
  grep -q '#define CONFIG_NONFREE 0' config.h || { echo "ERROR: config.h missing CONFIG_NONFREE 0" >&2; exit 1; }
  grep -q '#define CONFIG_SHARED 0' config.h || { echo "ERROR: config.h missing CONFIG_SHARED 0" >&2; exit 1; }
  grep -q '#define CONFIG_STATIC 1' config.h || { echo "ERROR: config.h missing CONFIG_STATIC 1" >&2; exit 1; }
  grep -q '#define CONFIG_LIBMP3LAME 1' config.h || { echo "ERROR: config.h missing CONFIG_LIBMP3LAME 1" >&2; exit 1; }
  grep -q '#define CONFIG_NETWORK 0' config.h || { echo "ERROR: config.h missing CONFIG_NETWORK 0" >&2; exit 1; }
fi

mkdir -p "$OUTPUT_DIR/notices" "$OUTPUT_DIR/sources"
cp COPYING.LGPLv2.1 "$OUTPUT_DIR/notices/FFmpeg-LGPL-2.1.txt"
cp "$BUILD_DIR/lame/COPYING" "$OUTPUT_DIR/notices/LAME-LGPL-2.0.txt"
cp config.h "$OUTPUT_DIR/notices/ffmpeg-build-config.h"
cp "$FFMPEG_ARCHIVE" "$LAME_ARCHIVE" "$OUTPUT_DIR/sources/"
