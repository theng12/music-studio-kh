#!/usr/bin/env python3
"""Record a verified release in dist/ and rotate older versions out of it.

Run only after the DMG was installed and verified. dist/ keeps the newest two
versions; anything older moves automatically to the app's own
`_trash/<date> <what>/`, without a hand step. The shared release-dist helper
still does the copy itself.

Guards (owner): a version named by release.json or previous-release.json, by a
running or queued job, or installed in /Applications is never retired, and
every move is logged with its hash and destination in the build receipt.

Usage: scripts/record-dist.py <version> [evidence]
"""
import datetime
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DIST = ROOT / "dist"
MANIFEST = DIST / ".release-dist.json"
TRASH = ROOT / "_trash"
HELPER = pathlib.Path(
    os.environ.get("RELEASE_DIST_HELPER", "")
    or pathlib.Path.home() / "Developer/Shared Tools/portfolio/tools/release-dist.py"
)
APP = "Music Studio KH"
KEEP = 2


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def files():
    if not MANIFEST.exists():
        return {}
    return json.loads(MANIFEST.read_text()).get("apps", {}).get(APP, {}).get("files", {})


def protected_versions():
    """Versions another consumer still names, so rotation must not touch them."""
    pinned = set()
    for name in ("release.json", "previous-release.json"):
        for path in (DIST / name, ROOT / name):
            if not path.exists():
                continue
            try:
                data = json.loads(path.read_text())
            except ValueError:
                continue
            for app in data.get("apps", []) if isinstance(data.get("apps"), list) else []:
                pinned.add(str(app.get("version", "")))
            pinned.add(str(data.get("version", "")))
    plist = pathlib.Path("/Applications") / f"{APP}.app/Contents/Info.plist"
    if plist.exists():
        out = subprocess.run(
            ["/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString", str(plist)],
            capture_output=True, text=True,
        )
        if out.returncode == 0:
            pinned.add(out.stdout.strip())
    return {value for value in pinned if value}


def busy(path):
    """True when a process holds the file open (a running or queued job)."""
    out = subprocess.run(["lsof", "--", str(path)], capture_output=True, text=True)
    return out.returncode == 0 and bool(out.stdout.strip())


def relocate(retired, version, before):
    """Move the helper's retirements to `_trash/<date> <what>/` and log them."""
    today = datetime.date.today().isoformat()
    moves = []
    for entry in retired:
        name = entry["name"]
        source = pathlib.Path(entry["moved_to"])
        record = before.get(name, {})
        destination = TRASH / f"{today} {APP} {record.get('version', 'superseded')} retired"
        destination.mkdir(parents=True, exist_ok=True)
        shutil.move(str(source), str(destination / name))
        moves.append({
            "version": record.get("version", ""),
            "sha256": record.get("sha256", ""),
            "moved_to": str(destination / name),
        })
    if moves:
        receipt = ROOT / f"artifacts/signing-{version}.json"
        if receipt.exists():
            data = json.loads(receipt.read_text())
            data.setdefault("rotation", []).extend(moves)
            receipt.write_text(json.dumps(data, indent=2) + "\n")
    return moves


def keepers(entries, version, keep=KEEP):
    """The versions dist keeps: this one plus the newest others, newest first."""
    others = sorted(
        (rec for rec in entries.values() if rec["version"] != version),
        key=lambda rec: tuple(int(part) for part in rec["version"].split(".")),
        reverse=True,
    )
    return [version] + [rec["version"] for rec in others[: keep - 1]]


def main():
    if len(sys.argv) < 2:
        sys.exit("usage: record-dist.py <version> [evidence]")
    version = sys.argv[1]
    evidence = sys.argv[2] if len(sys.argv) > 2 else f"artifacts/signing-{version}.json:{version}"
    dmg = ROOT / f"artifacts/{APP}-{version}.dmg"
    if not dmg.exists():
        sys.exit(f"missing {dmg}")

    entries = files()
    older = [rec for rec in entries.values() if rec["version"] != version]
    releases = [{
        "version": version,
        "source": str(dmg),
        "sha256": sha256(dmg),
        "bytes": dmg.stat().st_size,
        "evidence": evidence,
    }]
    for wanted in keepers(entries, version)[1:]:
        record = next(rec for rec in older if rec["version"] == wanted)
        path = DIST / f"{APP}-{wanted}.dmg"
        if not path.exists():
            continue
        releases.append({
            "version": record["version"],
            "source": str(path),
            "sha256": record["sha256"],
            "bytes": record["bytes"],
            "evidence": record["evidence"],
        })
    # A protected version is pinned too; the helper must refuse rather than retire it.
    protected = protected_versions()
    held = []
    for record in sorted(older, key=lambda rec: rec["version"], reverse=True):
        path = DIST / f"{APP}-{record['version']}.dmg"
        if record["version"] in protected and path.exists() and not busy(path):
            if record["version"] not in [r["version"] for r in releases]:
                releases.append({
                    "version": record["version"],
                    "source": str(path),
                    "sha256": record["sha256"],
                    "bytes": record["bytes"],
                    "evidence": record["evidence"],
                })
                held.append(record["version"])

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
    out = subprocess.run(
        ["python3", "-I", "-B", str(HELPER), str(pins)], check=True, capture_output=True, text=True
    )
    retired = json.loads(out.stdout or "{}").get(APP, {}).get("retired", [])
    for move in relocate(retired, version, entries):
        print(f"retired {move['version']} -> {move['moved_to']}")
    if held:
        print("held (named elsewhere):", ", ".join(held))


if __name__ == "__main__":
    main()
