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

    def trash(self):
        """The one trash, on the Desktop (owner 2026-11-18), inside this fixture.

        realpath: $TMPDIR is /var/folders/…, and the helper refuses a symlinked component."""
        return Path(os.path.realpath(self.root / 'Desktop' / '_trash' / 'music-studio-kh' / 'dist'))

    def pins(self, versions, pin_trash=True):
        releases = []
        for version in versions:
            data = ('mock accepted ' + version).encode()
            source = self.root / ('Music Studio KH-' + version + '.dmg')
            source.write_bytes(data)
            releases.append(dict(version=version, source=str(source), sha256=hashlib.sha256(data).hexdigest(), bytes=len(data), evidence='mock installed/native acceptance'))
        # `retirement_trash` must exist when the helper validates it, so make it before writing the pin.
        # Without this key the helper refuses a checkout (release-dist.py `_trash_dir`, d5a8c590) and
        # falls back to `<project>/_trash/dist` anywhere else — the folder the 2026-11-18 rule retires.
        app = dict(app='Music Studio KH', project=str(self.project), releases=releases)
        if pin_trash:
            trash = self.trash().parent
            trash.mkdir(parents=True, exist_ok=True)
            app['retirement_trash'] = str(trash)
        return dict(schema=1, apps=[app])

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
        self.assertEqual(len(list(self.trash().glob('*.dmg'))), 2)
        # One trash, on the Desktop: nothing may be written inside the checkout.
        self.assertFalse((self.project / '_trash').exists(), 'retired an installer into the project')
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

    def test_a_checkout_without_a_pin_is_refused_and_retires_nothing_into_it(self):
        """The class that used to build a trash inside a live checkout.

        The accepted copy is saved to the ledger before the retire step, so only the trash
        claim is honest here — asserting a clean tree would be asserting something the
        helper does not do (measured 2026-11-26)."""
        # Two accepted releases first, so the next run has one obsolete managed file to retire —
        # the helper only reaches `_trash_dir` when there is something to move.
        self.assertEqual(self.run_flow(self.pins(['0.2.6', '0.2.5'])).returncode, 0)
        (self.project / '.git').mkdir()
        refused = self.run_flow(self.pins(['0.2.7', '0.2.6'], pin_trash=False))
        self.assertNotEqual(refused.returncode, 0, refused.stdout)
        self.assertIn('retirement_trash', refused.stderr)
        self.assertFalse((self.project / '_trash').exists(), 'built a trash inside a checkout')
        self.assertTrue((self.dist / 'Music Studio KH-0.2.5.dmg').is_file(), 'retired an installer with no pin')


if __name__ == '__main__':
    unittest.main()
