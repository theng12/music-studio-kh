// Capture the screenshots docs/screenshots needs for a release, from the build being released.
//
// Runs the app's own server helper out of the bundle, with a throwaway data dir and a demo library,
// then drives the UI in headless Chromium — never the owner's data and never his window.
//
// Usage: node scripts/capture-screenshots.mjs "<path to Music Studio KH.app>" [outDir]
import { chromium } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn, execFileSync } from 'node:child_process';
import net from 'node:net';

const bundle = process.argv[2];
assert(bundle, 'usage: capture-screenshots.mjs "<app bundle>" [outDir]');
const out = resolve(process.argv[3] ?? 'docs/screenshots');
const helper = resolve(bundle, 'Contents/Helpers/Music Studio KH Helper.app/Contents/MacOS/Music Studio KH Helper');

async function freePort() {
  const server = net.createServer();
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const { port } = server.address();
  await new Promise(r => server.close(r));
  return port;
}

const temp = await mkdtemp(join(tmpdir(), 'music-studio-kh-shots-'));
const data = join(temp, 'profiles');
const music = join(temp, 'music');
const passwordFile = join(temp, 'password');
await mkdir(music, { recursive: true });
await writeFile(passwordFile, 'demo-pass\n', { mode: 0o600 });
const port = await freePort();
execFileSync(helper, ['--data', data, 'profile', 'create', '--name', 'Personal Studio',
  '--library', music, '--username', 'owner', '--port', String(port),
  '--password-file', passwordFile], { stdio: 'ignore' });

const server = spawn(helper, ['--data', data, 'serve'], { stdio: ['ignore', 'pipe', 'pipe'] });
let logs = '';
server.stdout.on('data', d => (logs += d));
server.stderr.on('data', d => (logs += d));
const address = `http://127.0.0.1:${port}`;
let ready = false;
for (let i = 0; i < 100; i += 1) {
  try { if ((await fetch(address)).ok) { ready = true; break; } } catch {}
  await new Promise(r => setTimeout(r, 100));
}
assert(ready, 'server did not start: ' + logs);

await mkdir(out, { recursive: true });
const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
const DEMO = [
  ['Harbor Lights', 'The Night Ferries', 'Low Tide'],
  ['Paper Boats', 'The Night Ferries', 'Low Tide'],
  ['Slow Tuesday', 'Marble Room', 'Quiet Hours'],
  ['Northbound', 'Marble Room', 'Quiet Hours'],
  ['Copper Sky', 'Ada Fields', 'Field Notes'],
  ['Lantern Song', 'Ada Fields', 'Field Notes'],
];
const wav = Buffer.alloc(44 + 8000);
wav.write('RIFF'); wav.writeUInt32LE(wav.length - 8, 4); wav.write('WAVEfmt ', 8);
wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22);
wav.writeUInt32LE(8000, 24); wav.writeUInt32LE(8000, 28); wav.writeUInt16LE(2, 32);
wav.writeUInt16LE(16, 34); wav.write('data', 36); wav.writeUInt32LE(8000, 40);
for (let i = 44; i < wav.length; i += 2) wav.writeInt16LE(Math.round(Math.sin((i - 44) / 2 * 2 * Math.PI * 330 / 8000) * 600), i);

const written = [];
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  await page.goto(address);
  await page.locator('#username').fill('OWNER');
  await page.locator('#password').fill('demo-pass');
  await page.getByRole('button', { name: 'Open library' }).click();
  await page.locator('#app').waitFor({ state: 'visible' });
  for (const [title, artist, album] of DEMO) {
    const audio = Buffer.from(wav);
    const response = await page.request.post(address + '/api/import', { multipart: {
      title, artist, album, file: { name: title + '.wav', mimeType: 'audio/wav', buffer: audio } } });
    assert(response.ok(), `demo import failed for ${title}`);
  }
  const setTheme = async theme => {
    await page.locator('#settings-nav').click();
    await page.locator('.settings-tabs button').filter({ hasText: /Appearance/i }).click();
    await page.getByRole('group', { name: 'Theme' }).getByRole('button', { name: theme, exact: true }).click();
    await page.locator('html').waitFor();
    await page.waitForFunction(t => document.documentElement.dataset.theme === t, theme.toLowerCase());
    await page.locator('#library-nav').click();
  };
  const shot = async name => {
    const file = join(out, name);
    await page.screenshot({ path: file });
    written.push(file);
  };

  await setTheme('Light');
  await page.locator('#library-nav').click();
  await page.waitForFunction(() => document.querySelectorAll('.track-card').length >= 6);
  await shot('library-light.png');

  await page.getByRole('button', { name: 'Play Harbor Lights', exact: true }).click();
  await page.waitForFunction(() => document.getElementById('audio')?.currentTime > 0);
  await shot('playing-light.png');

  await setTheme('Dark');
  await page.waitForFunction(() => document.querySelectorAll('.track-card').length >= 6);
  await page.getByRole('button', { name: 'Play Harbor Lights', exact: true }).click();
  await page.waitForFunction(() => document.getElementById('audio')?.currentTime > 0);
  await shot('playing-dark.png');

  await page.locator('#settings-nav').click();
  await page.locator('.settings-tabs button').filter({ hasText: /Library/i }).click();
  await page.locator('#settings-data-folder').waitFor();
  await shot('settings-dark.png');
} finally {
  await browser.close();
  server.kill('SIGTERM');
}

for (const file of written) {
  const { size } = await stat(file);
  console.log(`${size} bytes  ${file}`);
  assert(size <= 400 * 1024, `${file} is ${size} bytes, over the 400 KB limit`);
}
console.log('captured', written.length, 'screenshots into', out);
