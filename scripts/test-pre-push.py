#!/usr/bin/env python3
"""Exercise the real hook in disposable Git repos; all build tools are stubs."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

HOOK = Path(__file__).resolve().parent.parent / '.githooks/pre-push'
GIT = shutil.which('git')
STUB = '''#!PYTHON
import json, os
from pathlib import Path
import subprocess, sys
root = Path.cwd()
name = Path(sys.argv[0]).name
args = sys.argv[1:]
control = json.loads((root / '.git/control.json').read_text())
with (root / '.git/calls.jsonl').open('a') as f:
    f.write(json.dumps([name] + args) + '\\n')
if [name] + args[:1] == control.get('fail'):
    sys.exit(17)
if name == 'git':
    os.execv(os.environ['FIXTURE_GIT'], [os.environ['FIXTURE_GIT']] + args)
if name == 'go':
    if args == ['version']:
        print('go fixture compiler')
    elif args == ['env', 'GOENV']:
        print('.git/goenv')
    elif args == ['env', 'GOTOOLDIR']:
        print('.git/toolchain/go')
    elif args == ['env', '-json']:
        print(json.dumps({'GOENV': '.git/goenv', 'GOWORK': control.get('gowork', '')}))
    elif args == ['mod', 'verify']:
        print('all modules verified')
    elif args[:1] == ['build']:
        Path(args[args.index('-o') + 1]).write_bytes(b'fixture sidecar')
    elif args[:1] == ['test'] and control.get('mutate'):
        (root / 'internal/value.go').write_text('changed while testing')
elif name == 'npm' and args[:2] == ['config', 'get']:
    print('.git/npm-' + args[2])
elif name == 'cargo' and args[:1] == ['test'] and control.get('generate'):
    p = root / 'src-tauri/gen/schemas/desktop-schema.json'
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text('{}')
elif name == 'cargo' and args[:1] == ['metadata']:
    print(json.dumps({'packages': [
        {'manifest_path': str(root / 'src-tauri/Cargo.toml'), 'source': None},
        {'manifest_path': str(root / '.git/registry/Cargo.toml'), 'source': 'registry'}]}))
elif name == 'rustc':
    if args == ['--print', 'sysroot']:
        print('.git/toolchain/rust')
    else:
        print('rustc fixture\\nhost: fixture-target')
elif args == ['--version']:
    print(name + ' fixture ' + control.get(name + '_version', '1'))
'''.replace('PYTHON', sys.executable)


class HookTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git('init', '-q')
        for name, data in {
            '.githooks/pre-push': HOOK.read_bytes(),
            'go.mod': b'module fixture', 'go.sum': b'fixture checksum',
            'internal/value.go': b'package fixture',
            'internal/server/web/index.html': b'fixture html',
            'ui/main.ts': b'fixture UI',
            'package.json': b'{}', 'package-lock.json': b'{}',
            'src-tauri/Cargo.toml': b'[package]', 'src-tauri/Cargo.lock': b'fixture lock',
            'AGENTS.md': b'foreign instructions',
            '.gitignore': b'node_modules/\nsrc-tauri/binaries/\nsrc-tauri/runtime-tools/\nsrc-tauri/target/\nsrc-tauri/gen/\n',
        }.items():
            self.write(name, data)
        self.git('add', '.')
        self.checkpoint()
        self.write('node_modules/dependency.js', b'fixture dependency')
        self.write('src-tauri/binaries/music-studio-kh-server-fixture-target', b'fixture sidecar')
        self.write('src-tauri/runtime-tools/fixture-tool', b'fixture runtime')
        self.write('.git/toolchain/go/compile', b'fixture Go compiler backend')
        self.write('.git/toolchain/rust/bin/rustc', b'fixture Rust compiler backend')
        self.write('.git/registry/Cargo.toml', b'[package]')
        self.write('.git/registry/lib.rs', b'fixture dependency')
        self.control = {}
        self.set_control()
        self.tools = self.root / '.git/tools'
        self.tools.mkdir()
        for name in ('git', 'go', 'npm', 'node', 'rustc', 'cargo', 'cc'):
            p = self.tools / name
            p.write_text(STUB)
            p.chmod(0o755)
        self.env = dict(os.environ, PATH=str(self.tools) + os.pathsep + os.environ['PATH'], FIXTURE_GIT=GIT)
        self.cache = self.root / '.git/music-pre-push'

    def write(self, name, data):
        p = self.root / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_bytes(data)

    def git(self, *args):
        return subprocess.run([GIT, *args], cwd=self.root, check=True, capture_output=True, text=True)

    def checkpoint(self):
        # Synthetic fixture history only; never commits the source worktree.
        self.git('-c', 'user.name=Fixture', '-c',
                 'user.email=112568204+theng12@users.noreply.github.com',
                 'commit', '-qm', 'fixture inputs')

    def set_control(self, **values):
        self.control.update(values)
        self.write('.git/control.json', json.dumps(self.control).encode())

    def run_hook(self, expected=0):
        self.write('.git/calls.jsonl', b'')
        result = subprocess.run(['bash', '.githooks/pre-push'], cwd=self.root, env=self.env,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        self.calls = [json.loads(line) for line in (self.root / '.git/calls.jsonl').read_text().splitlines()]
        return result

    def checks(self):
        return [c for c in self.calls if c[:2] in
                (['go', 'vet'], ['go', 'test'], ['npm', 'run'], ['cargo', 'test'])]

    def baseline(self):
        self.run_hook()
        self.assertEqual(len(self.checks()), 4)

    def test_valid_unchanged_skips_only_lanes_with_live_integrity_and_asset_checks(self):
        self.baseline()
        result = self.run_hook()
        self.assertEqual(self.checks(), [])
        self.assertEqual(result.stdout.count('Reused '), 3)
        self.assertIn(['git', 'fsck', '--full'], self.calls)
        self.assertIn(['git', 'diff', '--exit-code', '--', 'internal/server/web'], self.calls)

    def test_changed_committed_source_runs_affected_lanes_only(self):
        self.baseline()
        self.write('internal/value.go', b'package changed')
        self.git('add', 'internal/value.go')
        self.checkpoint()
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go', 'cargo'])
        self.run_hook()
        self.assertEqual(self.checks(), [])

    def test_dirty_source_executes_without_caching_and_foreign_agents_is_independent(self):
        self.baseline()
        self.write('AGENTS.md', b'foreign work in progress')
        self.run_hook()
        self.assertEqual(self.checks(), [])
        self.write('internal/value.go', b'dirty covered source')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go', 'cargo'])
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go', 'cargo'])

    def test_dependency_and_toolchain_changes_execute_only_matching_lane(self):
        self.baseline()
        self.write('node_modules/dependency.js', b'changed dependency with same lock')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        self.set_control(npm_version='2')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        with (self.tools / 'npm').open('a') as stream:
            stream.write('\n# changed executable bytes with identical version\n')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        self.write('.git/registry/lib.rs', b'changed Cargo dependency with same lock')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['cargo'])

    def test_changed_compiler_behind_unchanged_launcher_invalidates(self):
        self.baseline()
        self.write('.git/toolchain/go/compile', b'different Go compiler, same version')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go'])
        self.write('.git/toolchain/rust/bin/rustc', b'different Rust compiler, same version')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['cargo'])

    def test_missing_identity_or_failed_receipt_executes(self):
        self.baseline()
        p = self.cache / 'go.json'
        saved = json.loads(p.read_text())
        del saved['inputs']
        p.write_text(json.dumps(saved))
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go'])
        saved = json.loads(p.read_text())
        saved['exit'] = 1
        p.write_text(json.dumps(saved))
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go'])

    def test_missing_or_corrupt_receipt_or_log_never_reuses(self):
        self.baseline()
        (self.cache / 'web.json').unlink()
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        (self.cache / 'web.json').write_text('[]')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        (self.cache / 'web.log').write_text('changed evidence')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])

    def test_early_command_failure_propagates_and_invalidates_prior_pass(self):
        self.baseline()
        (self.cache / 'go.json').write_text('{}')
        self.set_control(fail=['go', 'test'])
        self.run_hook(17)
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go'])
        self.assertIsNone(json.loads((self.cache / 'go.json').read_text())['exit'])
        self.set_control(fail=None)
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go'])

    def test_live_asset_currentness_failure_blocks_rust_even_with_saved_pass(self):
        self.baseline()
        self.write('internal/server/web/untracked.js', b'drift')
        self.run_hook(1)
        self.assertNotIn('cargo', [c[0] for c in self.checks()])

    def test_integrity_failure_always_blocks_cached_lanes(self):
        self.baseline()
        self.set_control(fail=['git', 'fsck'])
        self.run_hook(1)
        self.assertEqual(self.checks(), [])

    def test_missing_tool_identity_executes_instead_of_guessing(self):
        self.baseline()
        self.set_control(fail=['npm', '--version'])
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])

    def test_generated_tauri_schemas_do_not_prevent_a_valid_rust_receipt(self):
        self.set_control(generate=True)
        self.baseline()
        self.run_hook()
        self.assertEqual(self.checks(), [])

    def test_missing_config_identity_executes_original_lane_without_reuse(self):
        self.baseline()
        self.set_control(fail=['npm', 'config'])
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])

    def test_environment_identity_changes_without_saving_private_values(self):
        self.baseline()
        self.env['FIXTURE_PRIVATE_VALUE'] = 'fixture-secret-do-not-store'
        self.run_hook()
        self.assertEqual(len(self.checks()), 4)
        self.assertNotIn(self.env['FIXTURE_PRIVATE_VALUE'], ''.join(
            p.read_text() for p in self.cache.glob('*.json')))
        self.run_hook()
        self.assertEqual(self.checks(), [])

    def test_changed_inputs_during_command_fail_without_saving_pass(self):
        self.baseline()
        (self.cache / 'go.json').write_text('{}')
        self.set_control(mutate=True)
        self.run_hook(1)
        self.assertIsNone(json.loads((self.cache / 'go.json').read_text())['exit'])
        self.assertNotIn('npm', [c[0] for c in self.checks()])

    def test_changed_config_and_ignored_covered_source_invalidate(self):
        self.baseline()
        self.write('.git/npm-userconfig', b'script-shell=changed')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['npm'])
        self.write('.gitignore', (self.root / '.gitignore').read_bytes() + b'internal/ignored.go\n')
        self.write('internal/ignored.go', b'package ignored_by_git_only')
        self.run_hook()
        self.assertEqual([c[0] for c in self.checks()], ['go', 'go', 'cargo'])


if __name__ == '__main__':
    unittest.main()
