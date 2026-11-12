#!/usr/bin/env python3
"""Retention selection: dist keeps the newest three versions, plus any pinned.

Run: python3 -I -B scripts/test-record-dist.py
"""
import importlib.util
import pathlib
import unittest

SCRIPT = pathlib.Path(__file__).resolve().parent / "record-dist.py"
spec = importlib.util.spec_from_file_location("record_dist", SCRIPT)
record_dist = importlib.util.module_from_spec(spec)
spec.loader.exec_module(record_dist)


def entry(version):
    return {"version": version, "sha256": "0" * 64, "bytes": 1, "evidence": "test"}


class Keepers(unittest.TestCase):
    def test_newest_three_including_this_version(self):
        entries = {f"{v}.dmg": entry(v) for v in ("0.3.0", "0.3.1", "0.3.2", "0.3.3")}
        self.assertEqual(record_dist.keepers(entries, "0.3.3"), ["0.3.3", "0.3.2", "0.3.1"])

    def test_fewer_versions_than_the_limit_keeps_all(self):
        entries = {"0.3.0.dmg": entry("0.3.0"), "0.3.1.dmg": entry("0.3.1")}
        self.assertEqual(record_dist.keepers(entries, "0.3.1"), ["0.3.1", "0.3.0"])

    def test_a_rerun_of_an_old_version_keeps_it_newest(self):
        entries = {f"{v}.dmg": entry(v) for v in ("0.3.0", "0.3.1", "0.3.2")}
        self.assertEqual(record_dist.keepers(entries, "0.3.0"), ["0.3.0", "0.3.2", "0.3.1"])

    def test_the_limit_is_three_by_default(self):
        self.assertEqual(record_dist.KEEP, 3)


if __name__ == "__main__":
    unittest.main()
