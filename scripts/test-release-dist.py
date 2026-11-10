#!/usr/bin/env python3
"""Exercise Music's post-acceptance entry point with mock DMGs, never a build."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
HELPER = Path(os.environ.get('RELEASE_DIST_HELPER', str(Path.home() / 'Developer/Shared Tools/portfolio/tools/release-dist.py')))


class RetentionTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.project = self.root / 'Music project'
        self.project.mkdir()
        self.dist = self.project / 'dist'
        (self.dist / 'android').mkdir(parents=True)
        (self.dist / 'android/client.apk').write_bytes(b'unmanaged Android fixture')
        tools = self.root / 'tools'
        tools.mkdir()
        # If the retention branch falls through, stop before any packaging work.
        uname = tools / 'uname'
        uname.write_text('#!/bin/sh\nprintf "unexpected-build-path\\n" >&2\nexit 91\n')
        uname.chmod(0o755)
        self.env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'], RELEASE_DIST_HELPER=str(HELPER))

    def pins(self, versions):
        releases = []
        for version in versions:
            data = ('mock accepted ' + version).encode()
            source = self.root / ('Music Studio KH-' + version + '.dmg')
            source.write_bytes(data)
            releases.append(dict(version=version, source=str(source), sha256=hashlib.sha256(data).hexdigest(), bytes=len(data), evidence='mock installed/native acceptance'))
        return dict(schema=1, apps=[dict(app='Music Studio KH', project=str(self.project), releases=releases)])

    def run_flow(self, pins):
        path = self.root / 'accepted-pins.json'
        path.write_text(json.dumps(pins))
        return subprocess.run(['bash', str(ROOT / 'scripts/build-mac.sh'), '--retain-accepted', str(path)], env=self.env, capture_output=True, text=True)

    def snapshot(self):
        return {str(p.relative_to(self.dist)): p.read_bytes() for p in self.dist.rglob('*') if p.is_file()}

    def test_normal_flow_retires_only_managed_old_pair_and_is_idempotent(self):
        old = self.run_flow(self.pins(['0.2.6', '0.2.5']))
        self.assertEqual(old.returncode, 0, old.stderr)
        pins = self.pins(['0.2.8', '0.2.7'])
        result = self.run_flow(pins)
        self.assertEqual(result.returncode, 0, result.stderr)
        summary = json.loads(result.stdout)['Music Studio KH']
        self.assertEqual(len(summary['retired']), 2)
        self.assertEqual({p.name for p in self.dist.glob('*.dmg')}, {'Music Studio KH-0.2.8.dmg', 'Music Studio KH-0.2.7.dmg'})
        self.assertEqual((self.dist / 'android/client.apk').read_bytes(), b'unmanaged Android fixture')
        self.assertEqual(len(list((self.project / '_trash/dist').glob('*.dmg'))), 2)
        before = self.snapshot()
        repeated = self.run_flow(pins)
        self.assertEqual(repeated.returncode, 0, repeated.stderr)
        self.assertEqual(self.snapshot(), before)
        self.assertEqual(json.loads(repeated.stdout)['Music Studio KH']['copied'], [])

    def test_bad_candidate_leaves_previous_working_pair_unchanged(self):
        baseline = self.run_flow(self.pins(['0.2.8', '0.2.7']))
        self.assertEqual(baseline.returncode, 0, baseline.stderr)
        before = self.snapshot()
        pins = self.pins(['0.2.9', '0.2.8'])
        pins['apps'][0]['releases'][0]['sha256'] = '0' * 64
        failed = self.run_flow(pins)
        self.assertNotEqual(failed.returncode, 0)
        self.assertIn('pinned', failed.stderr)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse((self.project / '_trash').exists())


if __name__ == '__main__':
    unittest.main()
