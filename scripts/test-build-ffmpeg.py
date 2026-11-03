#!/usr/bin/env python3
"""Exercise Windows-style cleanup locks without compiling or downloading tools."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().with_name('build-ffmpeg.sh')


class CleanupTest(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp(prefix='music-ffmpeg-test-'))
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.shim('uname', '#!/bin/bash\nprintf "MINGW64_NT-10.0\\n"\n')
        self.shim('tar', '''#!/usr/bin/env python3
from pathlib import Path
import sys
p = Path(sys.argv[sys.argv.index('-C') + 1])
p.mkdir(parents=True, exist_ok=True)
(p / 'configure').write_text('#!/bin/bash\\necho "configure diagnostic marker" >&2\\nexit "${FAKE_CONFIG_EXIT:-0}"\\n')
(p / 'configure').chmod(0o755)
(p / 'COPYING').write_text('fixture license')
''')
        self.shim('make', '''#!/bin/bash
if [[ "$PWD" == */ffmpeg ]]; then
  for name in ffmpeg.exe ffprobe.exe config.h COPYING.LGPLv2.1; do
    printf 'fixture\\n' > "$name"
  done
fi
''')
        self.shim('mv', '''#!/bin/bash
case "$PWD/" in "$1/"*) echo 'Windows-style lock: current directory is inside source' >&2; exit 99;; esac
exec /bin/mv "$@"
''')
        for name in ('ffmpeg.tar.gz', 'lame.tar.gz'):
            (self.root / name).write_text('fixture archive handled by tar shim')

    def shim(self, name, source):
        path = self.bin / name
        path.write_text(source)
        path.chmod(0o755)

    def tearDown(self):
        trash = Path.home() / 'Developer/_trash/tuneharbor/tests'
        trash.mkdir(parents=True, exist_ok=True)
        shutil.move(str(self.root), str(trash / self.root.name))

    def run_build(self, code):
        env = os.environ.copy()
        env['PATH'] = str(self.bin) + os.pathsep + env['PATH']
        env['FAKE_CONFIG_EXIT'] = str(code)
        return subprocess.run(
            ['bash', str(SCRIPT), str(self.root / 'ffmpeg.tar.gz'),
             str(self.root / 'lame.tar.gz'), str(self.root / 'output')],
            cwd=self.root, env=env, text=True, capture_output=True, timeout=20,
        )

    def test_success_retires_build_outside_its_current_directory(self):
        result = self.run_build(0)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue((self.root / 'output/ffmpeg.exe').is_file())
        self.assertTrue((self.root / 'output/ffprobe.exe').is_file())

    def test_failure_keeps_original_status_and_prints_diagnostic(self):
        result = self.run_build(7)
        self.assertEqual(result.returncode, 7, result.stdout + result.stderr)
        self.assertIn('configure diagnostic marker', result.stderr)
        self.assertNotIn('Windows-style lock', result.stderr)


if __name__ == '__main__':
    unittest.main()
