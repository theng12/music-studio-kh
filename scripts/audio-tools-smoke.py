#!/usr/bin/env python3
"""Check bundled audio conversion and cover embedding with generated local fixtures."""
import json
import math
import pathlib
import struct
import subprocess
import sys
import tempfile
import wave

root = pathlib.Path(__file__).resolve().parents[1]
tools = pathlib.Path(sys.argv[1]).resolve()
proof = pathlib.Path(tempfile.mkdtemp(prefix="audio-tools-proof-", dir=root / "artifacts"))
ffmpeg = str(tools / "ffmpeg")
ffprobe = str(tools / "ffprobe")


def run(args):
    result = subprocess.run(args, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise AssertionError(result.stderr[-4000:])
    return result.stdout


source = proof / "tone.wav"
with wave.open(str(source), "wb") as audio:
    audio.setnchannels(1)
    audio.setsampwidth(2)
    audio.setframerate(8000)
    audio.writeframes(b"".join(struct.pack("<h", round(1000 * math.sin(i * 2 * math.pi * 440 / 8000))) for i in range(2400)))
results = []
for extension, codec in (("mp3", "libmp3lame"), ("flac", "flac"), ("m4a", "aac"), ("ogg", "vorbis"), ("opus", "opus")):
    target = proof / ("tone." + extension)
    run([ffmpeg, "-v", "error", "-i", str(source), "-ac", "2", "-c:a", codec, "-strict", "-2", str(target)])
    metadata = json.loads(run([ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "json", str(target)]))
    assert float(metadata["format"]["duration"]) > 0, extension
    run([ffmpeg, "-v", "error", "-i", str(target), "-f", "null", "-"])
    results.append({"format": extension, "encoded": True, "probed": True, "decoded": True})

# yt-dlp converts thumbnails to PNG before embedding; an audio-only check misses this.
jpeg = proof / "cover.jpg"
png = proof / "cover.png"
run([ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=green:s=16x16", "-frames:v", "1", str(jpeg)])
run([ffmpeg, "-v", "error", "-i", str(jpeg), "-frames:v", "1", str(png)])
assert png.read_bytes().startswith(b"\x89PNG\r\n\x1a\n"), "PNG thumbnail encoding"
covered = proof / "with-cover.mp3"
run([ffmpeg, "-v", "error", "-i", str(proof / "tone.mp3"), "-i", str(png), "-map", "0:a", "-map", "1:v", "-c", "copy", "-id3v2_version", "3", "-disposition:v", "attached_pic", str(covered)])
streams = json.loads(run([ffprobe, "-v", "error", "-show_streams", "-of", "json", str(covered)]))["streams"]
assert any(s["codec_type"] == "audio" for s in streams), "audio retained with artwork"
assert any(s["codec_type"] == "video" and s.get("disposition", {}).get("attached_pic") == 1 for s in streams), "cover embedded"
results.append({"thumbnailToPNG": True, "mp3CoverEmbedded": True})
(proof / "result.json").write_text(json.dumps(results, indent=2) + "\n")
print("PASS: five audio format roundtrips, thumbnail-to-PNG conversion and MP3 cover embedding")
print("Proof: " + str(proof / "result.json"))
