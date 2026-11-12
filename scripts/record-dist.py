#!/usr/bin/env python3
"""Record a verified release in dist/: this version plus the previous one.

Run only after the DMG was installed and verified. Uses the kind of receipt the
build leaves in artifacts/ as evidence, carries the previous accepted version
forward, and delegates the copy/retirement to the shared release-dist helper.

Usage: scripts/record-dist.py <version> [evidence]
"""
import hashlib
import json
import os
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DIST = ROOT / "dist"
MANIFEST = DIST / ".release-dist.json"
HELPER = pathlib.Path(
    os.environ.get("RELEASE_DIST_HELPER", "")
    or pathlib.Path.home() / "Developer/Shared Tools/portfolio/tools/release-dist.py"
)
APP = "Music Studio KH"


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def previous_release(version):
    if not MANIFEST.exists():
        return None
    files = json.loads(MANIFEST.read_text()).get("apps", {}).get(APP, {}).get("files", {})
    older = [rec for rec in files.values() if rec["version"] != version]
    for rec in sorted(older, key=lambda rec: rec["version"], reverse=True):
        path = DIST / f"{APP}-{rec['version']}.dmg"
        if path.exists() and sha256(path) == rec["sha256"]:
            return {"path": path, "record": rec}
    return None


def main():
    if len(sys.argv) < 2:
        sys.exit("usage: record-dist.py <version> [evidence]")
    version = sys.argv[1]
    evidence = sys.argv[2] if len(sys.argv) > 2 else f"artifacts/signing-{version}.json:{version}"
    dmg = ROOT / f"artifacts/{APP}-{version}.dmg"
    if not dmg.exists():
        sys.exit(f"missing {dmg}")

    releases = [{
        "version": version,
        "source": str(dmg),
        "sha256": sha256(dmg),
        "bytes": dmg.stat().st_size,
        "evidence": evidence,
    }]
    previous = previous_release(version)
    if previous:
        releases.append({
            "version": previous["record"]["version"],
            "source": str(previous["path"]),
            "sha256": previous["record"]["sha256"],
            "bytes": previous["record"]["bytes"],
            "evidence": previous["record"]["evidence"],
        })

    pins = pathlib.Path(os.environ.get("TMPDIR", "/tmp")) / f"dist-pins-{version}.json"
    pins.write_text(json.dumps({
        "schema": 1,
        "apps": [{
            "app": APP,
            "project": str(ROOT),
            "retirement_trash": str(pathlib.Path.home() / "Desktop/_trash/music-studio-kh"),
            "releases": releases,
        }],
    }, indent=2) + "\n")
    subprocess.run(["python3", "-I", "-B", str(HELPER), str(pins)], check=True)


if __name__ == "__main__":
    main()
