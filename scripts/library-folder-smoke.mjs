import { chromium } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rename } from 'node:fs/promises';
import { tmpdir, homedir } from 'node:os';
import { join, resolve, basename } from 'node:path';
import { spawn, execFileSync } from 'node:child_process';
import net from 'node:net';

const socket = net.createServer();
await new Promise(r => socket.listen(0, '127.0.0.1', r));
const port = socket.address().port;
await new Promise(r => socket.close(r));
const temp = await mkdtemp(join(tmpdir(), 'music-folder-smoke-'));
const music = join(temp, 'music');
await mkdir(music);
const password = join(temp, 'password');
await writeFile(password, [1, 2, 3, 4, 5, 6].join('') + '\n', { mode: 0o600 });
const executable = resolve('bin/tuneharbor');
execFileSync(executable, ['--data', join(temp, 'profiles'), 'profile', 'create', '--name', 'Folder test', '--library', music, '--username', 'owner', '--port', String(port), '--password-file', password]);
const server = spawn(executable, ['--data', join(temp, 'profiles'), 'serve'], { stdio: ['ignore', 'pipe', 'pipe'] });
let logs = '';
server.stdout.on('data', data => logs += data);
server.stderr.on('data', data => logs += data);
let browser;
try {
  const origin = 'http://127.0.0.1:' + port;
  let ready = false;
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(origin)).ok) { ready = true; break; } } catch {}
    await new Promise(r => setTimeout(r, 100));
  }
  assert(ready, 'isolated server did not start: ' + logs);
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  await page.goto(origin);
  await page.locator('#username').fill('owner');
  await page.locator('#password').fill('123456');
  await page.getByRole('button', { name: 'Open library' }).click();
  await page.locator('#app').waitFor();
  const wav = Buffer.alloc(44 + 16000);
  wav.write('RIFF'); wav.writeUInt32LE(wav.length - 8, 4); wav.write('WAVEfmt ', 8);
  wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22);
  wav.writeUInt32LE(8000, 24); wav.writeUInt32LE(16000, 28); wav.writeUInt16LE(2, 32);
  wav.writeUInt16LE(16, 34); wav.write('data', 36); wav.writeUInt32LE(16000, 40);
  const album = join(music, 'Added outside the app');
  await mkdir(album);
  const started = Date.now();
  await writeFile(join(album, 'Folder topup.wav'), wav);
  await page.getByRole('button', { name: 'Play Folder topup', exact: true }).waitFor({ timeout: 45000 });
  const elapsed = Date.now() - started;
  const track = await page.evaluate(async () => (await (await fetch('/api/tracks')).json()).find(t => t.title === 'Folder topup'));
  assert(track, 'folder addition must be indexed without Import or Rescan');
  const stream = await page.request.get(origin + '/api/stream/' + track.id, { headers: { Range: 'bytes=0-43' } });
  assert.equal(stream.status(), 206, 'new folder file available for range streaming');
  assert.equal((await stream.body()).subarray(0, 4).toString(), 'RIFF');
  console.log('PASS: direct folder top-up auto-indexed and appeared in library in ' + elapsed + 'ms; authenticated range streaming served original WAV. No Import/Rescan used.');
} finally {
  if (browser) await browser.close();
  if (server.exitCode === null && server.signalCode === null) {
    const exited = new Promise(r => server.once('exit', r));
    server.kill('SIGTERM');
    await exited;
  }
  const trash = join(homedir(), 'Developer/_trash/tuneharbor/test-fixtures');
  await mkdir(trash, { recursive: true });
  await rename(temp, join(trash, basename(temp)));
}
