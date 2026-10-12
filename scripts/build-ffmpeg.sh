#!/usr/bin/env bash
set -euo pipefail
# Build native LGPL FFmpeg/ffprobe with MP3 support from verified source archives.
# Inputs are absolute paths to FFmpeg source archive, LAME archive and output dir.
FFMPEG_ARCHIVE="$1"
LAME_ARCHIVE="$2"
OUTPUT_DIR="$3"
BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT
mkdir -p "$BUILD_DIR/lame" "$BUILD_DIR/ffmpeg" "$OUTPUT_DIR"
tar -xzf "$LAME_ARCHIVE" --strip-components=1 -C "$BUILD_DIR/lame"
tar -xzf "$FFMPEG_ARCHIVE" --strip-components=1 -C "$BUILD_DIR/ffmpeg"
PREFIX_DIR="$BUILD_DIR/prefix"
cd "$BUILD_DIR/lame"
./configure --prefix="$PREFIX_DIR" --disable-shared --enable-static --disable-frontend > "$BUILD_DIR/lame-config.log" 2>&1
make -j2 > "$BUILD_DIR/lame-build.log" 2>&1
make install > /dev/null 2>&1
cd "$BUILD_DIR/ffmpeg"
PKG_CONFIG_PATH="$PREFIX_DIR/lib/pkgconfig" ./configure --prefix="$PREFIX_DIR" --disable-shared --enable-static --disable-doc --disable-debug --disable-ffplay --disable-network --disable-x86asm --enable-libmp3lame --extra-cflags="-I$PREFIX_DIR/include" --extra-ldflags="-L$PREFIX_DIR/lib" > "$BUILD_DIR/ffmpeg-config.log" 2>&1
make -j2 > "$BUILD_DIR/ffmpeg-build.log" 2>&1
# Copy only the executable products, not object or source files.
for name in ffmpeg ffprobe ffmpeg.exe ffprobe.exe; do if test -f "$name"; then cp "$name" "$OUTPUT_DIR/$name"; fi; done
mkdir -p "$OUTPUT_DIR/notices" "$OUTPUT_DIR/sources"
cp COPYING.LGPLv2.1 "$OUTPUT_DIR/notices/FFmpeg-LGPL-2.1.txt"
cp "$BUILD_DIR/lame/COPYING" "$OUTPUT_DIR/notices/LAME-LGPL-2.0.txt"
cp config.h "$OUTPUT_DIR/notices/ffmpeg-build-config.h"
cp "$FFMPEG_ARCHIVE" "$LAME_ARCHIVE" "$OUTPUT_DIR/sources/"
