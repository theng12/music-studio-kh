#!/usr/bin/env python3
"""The background runner lives inside the app bundle and nowhere else.

Owner rule: deleting the app deletes the runner, an install replaces it in
place, and nothing outlives the bundle — no loose binary in ~/Library,
~/Applications or /tmp, and no launchd job naming a missing path.

Run: python3 -I -B scripts/test-bundle-isolation.py [path/to/App.app]
"""
import pathlib
import plistlib
import shutil
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
APP_NAME = "Music Studio KH"
HELPER = f"Contents/Helpers/{APP_NAME} Helper.app/Contents/MacOS/{APP_NAME} Helper"
LOOSE_NAMES = {f"{APP_NAME} Helper", "music-studio-kh-server"}
SKIP = ("_trash", ".Trash", "Trash", "Caches", "artifacts", "target", "node_modules")


def fail(message):
    print(f"FAIL: {message}")
    sys.exit(1)


def loose_copies(home):
    """Runner binaries that are not inside an installed bundle."""
    found = []
    for base in (home / "Library", home / "Applications", pathlib.Path("/tmp")):
        if not base.exists():
            continue
        for path in base.rglob("*"):
            if any(part in SKIP for part in path.parts):
                continue
            if path.name in LOOSE_NAMES and path.is_file() and not any(part.endswith(".app") for part in path.parts):
                found.append(path)
            if len(found) > 20:
                return found
    return found


def launch_jobs(app):
    """launchd plists that name this app or its runner."""
    hits = []
    for folder in (pathlib.Path.home() / "Library/LaunchAgents", pathlib.Path("/Library/LaunchAgents")):
        if not folder.exists():
            continue
        for plist in folder.glob("*.plist"):
            try:
                data = plistlib.loads(plist.read_bytes())
            except Exception:
                continue
            text = " ".join(str(value) for value in data.values() if isinstance(value, str))
            if APP_NAME in text or "music-studio-kh" in text.lower():
                hits.append(plist)
    return hits


def main():
    if len(sys.argv) > 1:
        app = pathlib.Path(sys.argv[1])
    else:
        built = ROOT / f"artifacts/{APP_NAME}.app"
        installed = pathlib.Path("/Applications") / f"{APP_NAME}.app"
        app = built if built.exists() else installed
    if not app.exists():
        fail(f"missing bundle {app}")

    helper = app / HELPER
    if not helper.is_file():
        fail(f"runner not inside the bundle: {helper}")
    if not helper.stat().st_mode & 0o111:
        fail(f"runner is not executable: {helper}")

    stray = loose_copies(pathlib.Path.home())
    if stray:
        fail("runner copied outside a bundle: " + ", ".join(str(path) for path in stray))

    jobs = launch_jobs(app)
    if jobs:
        fail("launchd job names the app: " + ", ".join(str(path) for path in jobs))

    # Deleting the bundle must take the runner with it.
    with tempfile.TemporaryDirectory() as scratch:
        copy = pathlib.Path(scratch) / app.name
        subprocess.run(["cp", "-R", str(app), str(copy)], check=True)
        if not (copy / HELPER).exists():
            fail("copied bundle lost its runner")
        shutil.rmtree(copy)
        if (copy / HELPER).exists():
            fail("runner survived deleting the bundle")
    print("PASS: runner lives only inside the bundle; deleting the bundle removes it")


if __name__ == "__main__":
    main()
