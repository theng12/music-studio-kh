#!/usr/bin/env python3
"""Check the Mac onedir helper's first and repeated launches against Doctor's budget."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--budget-seconds", type=float, required=True)
    parser.add_argument("--receipt", type=Path, required=True)
    args = parser.parse_args()
    assert args.budget_seconds > 0
    assert not args.receipt.exists(), "Preserve the previous timing receipt"
    binary = args.binary.resolve(strict=True)
    runtime = binary.parent.parent / "Frameworks" if binary.parent.name == "MacOS" else binary.parent / "_internal"
    assert (runtime / "Python.framework").is_dir(), "Mac helper must use the unpackaged runtime, not unpack on every launch"
    version = json.loads((Path(__file__).resolve().parents[1] / "packaging/runtime-tools.json").read_text())["ytDlp"]["version"]
    args.receipt.parent.mkdir(parents=True, exist_ok=True)
    measurements = []
    for _ in range(3):
        temporary = tempfile.mkdtemp(prefix="yt-dlp-startup-", dir=args.receipt.parent)
        started = time.monotonic()
        process = subprocess.Popen([str(binary), "--version"], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   text=True, start_new_session=True, env={**os.environ, "TMPDIR": temporary})
        try:
            output, error = process.communicate(timeout=args.budget_seconds)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGTERM)
            process.communicate(timeout=5)
            raise AssertionError(f"yt-dlp exceeded the {args.budget_seconds:g}-second startup budget") from None
        elapsed = time.monotonic() - started
        assert process.returncode == 0, error
        assert output.strip() == version, "Packaged yt-dlp version differs from the pinned manifest"
        assert elapsed <= args.budget_seconds, f"Startup took {elapsed:.3f}s (budget {args.budget_seconds:g}s)"
        measurements.append(round(elapsed, 3))
    receipt = {"binary": str(binary), "version": version, "budgetSeconds": args.budget_seconds,
               "elapsedSeconds": measurements, "maximumSeconds": max(measurements), "passed": True}
    with args.receipt.open("x") as output:
        json.dump(receipt, output, indent=2)
        output.write("\n")
    print(f"PASS: yt-dlp first/repeated startup {measurements}s; budget {args.budget_seconds:g}s")


if __name__ == "__main__":
    main()
