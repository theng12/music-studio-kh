#!/usr/bin/env python3
"""Verify a packaged Mac release, or compare receipts from two differing builds.

Run with python3 -I. Receipts contain public signing requirements and hashes only.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import secrets
import select
import socket
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request


TOOLS = ("tuneharbor-server", "ffmpeg", "ffprobe", "yt-dlp", "deno")
CERTIFICATE = "b8f43e85b80ec15c1bad26c6da03461e0bb317a2"


def run(*args):
    return subprocess.run(args, check=True, text=True, capture_output=True).stdout


def compare(before, after):
    assert before["version"] != after["version"], "Need two different packaged versions"
    assert before["requirements"] == after["requirements"], "Designated requirements changed"
    for requirement in after["requirements"].values():
        assert "cdhash" not in requirement and CERTIFICATE in requirement.lower(), "Certificate-based requirement required"
    for name in ("app", "tuneharbor-server"):
        assert before["hashes"][name] != after["hashes"][name], f"{name} executable did not change"
    assert before["launchd"]["passed"] and after["launchd"]["passed"], "Both builds must pass isolated launch"


def launch(app, version, evidence):
    """Launch only the packaged server with a fresh profile and empty music folder."""
    work = Path(tempfile.mkdtemp(prefix="music-signing-", dir=evidence))
    os.chmod(work, 0o700)
    music = work / "empty-music"
    music.mkdir()
    data = work / "profiles"
    password = secrets.token_hex(24)
    password_file = work / "password"
    password_file.write_text(password)
    password_file.chmod(0o600)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    executable = app / "Contents/MacOS/tuneharbor-server"
    run(str(executable), "--data", str(data), "profile", "create", "--library", str(music),
        "--port", str(port), "--username", "signing-check", "--password-file", str(password_file))
    label = "app.tuneharbor.signing-test." + secrets.token_hex(8)
    domain = f"gui/{os.getuid()}"
    service = domain + "/" + label
    job = work / "job.plist"
    job.write_bytes(plistlib.dumps({
        "Label": label,
        "ProgramArguments": [str(executable), "--data", str(data), "serve", "--tools", str(executable.parent)],
        "RunAtLoad": True,
        "StandardOutPath": str(work / "stdout.log"),
        "StandardErrorPath": str(work / "stderr.log"),
    }))
    loaded = False
    pid = None
    try:
        run("launchctl", "bootstrap", domain, str(job))
        loaded = True
        query = urllib.parse.urlencode({"u": "signing-check", "p": password, "v": "1.16.1", "c": "signing-test", "f": "json"})
        url = f"http://127.0.0.1:{port}/rest/ping.view?{query}"
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        deadline = time.monotonic() + 20
        while True:
            try:
                with opener.open(url, timeout=1) as response:
                    health = json.load(response)["subsonic-response"]
                assert health["status"] == "ok" and health["serverVersion"] == version, "Health/version mismatch"
                break
            except (OSError, urllib.error.URLError):
                if time.monotonic() >= deadline:
                    raise RuntimeError("Packaged launch did not become healthy within 20 seconds") from None
                time.sleep(0.2)
        state = run("launchctl", "print", service)
        match = re.search(r"^\s*pid = (\d+)$", state, re.M)
        assert match, "Launch job has no surviving PID"
        pid = int(match.group(1))
        os.kill(pid, 0)
        command = run("/bin/ps", "-p", str(pid), "-o", "command=").strip()
        assert command.startswith(str(executable) + " "), "Job PID is not the packaged executable"
        # A second authenticated request checks that the job survives its first response.
        with opener.open(url, timeout=2) as response:
            assert json.load(response)["subsonic-response"]["serverVersion"] == version
        return {"passed": True, "label": label, "pid": pid, "port": port, "work": str(work)}
    finally:
        if loaded:
            # bootout can return before the process exits; wait on its kernel event.
            queue = select.kqueue()
            try:
                if pid is not None:
                    try:
                        queue.control([select.kevent(pid, filter=select.KQ_FILTER_PROC,
                            flags=select.KQ_EV_ADD | select.KQ_EV_ONESHOT, fflags=select.KQ_NOTE_EXIT)], 0, 0)
                    except ProcessLookupError:
                        pid = None
                run("launchctl", "bootout", service)
                if pid is not None:
                    assert queue.control(None, 1, 10), "Owned test process did not exit after bootout"
            finally:
                queue.close()
            result = subprocess.run(["launchctl", "print", service], capture_output=True)
            assert result.returncode != 0, "Owned test job still loaded after process exit"
        # Retain all fixture evidence; blank the generated test password.
        password_file.write_text("")


def capture(app, evidence):
    app = app.resolve(strict=True)
    metadata = plistlib.loads((app / "Contents/Info.plist").read_bytes())
    assert metadata["CFBundleIdentifier"] == "app.tuneharbor.desktop"
    run("codesign", "--verify", "--deep", "--strict", str(app))
    paths = {"bundle": app, "app": app / "Contents/MacOS" / metadata["CFBundleExecutable"]}
    paths.update({name: app / "Contents/MacOS" / name for name in TOOLS})
    requirements, hashes = {}, {}
    for name, path in paths.items():
        run("codesign", "--verify", "--strict", str(path))
        result = subprocess.run(["codesign", "-d", "-r-", str(path)], check=True, text=True, capture_output=True)
        requirement = next(line for line in (result.stdout + result.stderr).splitlines() if line.startswith("designated =>"))
        assert "cdhash" not in requirement and CERTIFICATE in requirement.lower(), f"{name}: wrong signing identity"
        requirements[name] = requirement
        if path.is_file():
            with path.open("rb") as source:
                hashes[name] = hashlib.file_digest(source, "sha256").hexdigest()
    version = metadata["CFBundleShortVersionString"]
    return {"version": version, "requirements": requirements, "hashes": hashes, "launchd": launch(app, version, evidence)}


def self_test():
    before = {"version": "0.2.8", "requirements": {"app": 'designated => certificate leaf = H"' + CERTIFICATE + '"'},
              "hashes": {"app": "a", "tuneharbor-server": "b"}, "launchd": {"passed": True}}
    after = {**before, "version": "0.2.9", "hashes": {"app": "c", "tuneharbor-server": "d"}}
    compare(before, after)
    for invalid in [before, {**after, "requirements": {"app": "designated => cdhash H\"abc\""}},
                    {**after, "hashes": before["hashes"]}, {**after, "launchd": {"passed": False}}]:
        try:
            compare(before, invalid)
        except AssertionError:
            continue
        raise AssertionError("Invalid proof accepted")
    print("PASS: different builds, certificate requirements, changed binaries, and launch evidence enforced")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app", type=Path)
    parser.add_argument("--receipt", type=Path)
    parser.add_argument("--compare", nargs=2, type=Path, metavar=("BEFORE", "AFTER"))
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
    elif args.compare:
        compare(*(json.loads(path.read_text()) for path in args.compare))
        print("PASS: certificate designated requirements stable across two differing packaged versions")
    elif args.app and args.receipt:
        args.receipt.parent.mkdir(parents=True, exist_ok=True)
        assert not args.receipt.exists(), "Preserve the existing receipt; use a fresh path"
        receipt = capture(args.app, args.receipt.parent)
        with args.receipt.open("x") as output:
            json.dump(receipt, output, indent=2)
            output.write("\n")
        print("PASS: packaged signatures, identity, isolated launch health/version/PID and owned job cleanup")
    else:
        parser.error("Use --app APP --receipt JSON, --compare BEFORE AFTER, or --self-test")


if __name__ == "__main__":
    main()
