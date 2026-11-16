// A missing or unplugged music folder must say so on the library screen, never read as an empty library.
// Headless: temp data dir, loopback port, no window. BIN and OUT env vars pick the binary and evidence folder.
import { chromium } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, rename } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn, execFileSync } from 'node:child_process';
import net from 'node:net';

const executable = resolve(process.env.BIN || 'bin/music-studio-kh');
const out = process.env.OUT || '';
const socket = net.createServer();
await new Promise(r => socket.listen(0, '127.0.0.1', r));
const port = socket.address().port;
await new Promise(r => socket.close(r));
const temp = await mkdtemp(join(tmpdir(), 'music-folder-missing-'));
const data = join(temp, 'profiles'), music = join(temp, 'music'), password = join(temp, 'password');
await mkdir(music);
await writeFile(password, '123456\n', { mode: 0o600 });
execFileSync(executable, ['--data', data, 'profile', 'create', '--name', 'Missing', '--library', music, '--username', 'owner', '--port', String(port), '--password-file', password]);
const origin = 'http://127.0.0.1:' + port;
let server, logs = '';
const start = async () => {
  server = spawn(executable, ['--data', data, 'serve'], { stdio: ['ignore', 'pipe', 'pipe'] });
  server.stdout.on('data', d => logs += d);
  server.stderr.on('data', d => logs += d);
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(origin)).ok) return; } catch {}
    await new Promise(r => setTimeout(r, 100));
  }
  throw new Error('server did not start\n' + logs);
};
const stop = async () => { server.kill('SIGTERM'); await new Promise(r => server.once('exit', r)); };
const profileFile = execFileSync('find', [data, '-name', 'profile.json']).toString().trim();
const observations = {};
let browser;
try {
  await start(); // first start writes the folder's identity, like a profile that once worked
  await stop();
  const drive = 'No Such Drive ' + port;
  const profile = JSON.parse(await readFile(profileFile, 'utf8'));
  profile.library = '/Volumes/' + drive + '/Music';
  await writeFile(profileFile, JSON.stringify(profile));
  await start();
  browser = await chromium.launch({ headless: true });
  const open = async desktop => {
    const context = await browser.newContext({ viewport: { width: 1200, height: 800 } });
    if (desktop) await context.addInitScript(() => {
      window.__TUNEHARBOR_DESKTOP__ = true;
      window.__titles = [];
      window.__lastTitle = document.title;
      // A 10 ms poll catches the app's 60 ms desktop command title, which a head-only observer misses.
      setInterval(() => {
        if (document.title !== window.__lastTitle) {
          window.__lastTitle = document.title;
          window.__titles.push(document.title);
        }
      }, 10);
    });
    const page = await context.newPage();
    await page.goto(origin);
    await page.locator('#username').fill('owner');
    await page.locator('#password').fill('123456');
    await page.getByRole('button', { name: 'Open library' }).click();
    await page.locator('#app').waitFor({ state: 'visible' });
    await page.locator('#folder-problem, #empty-library').first().waitFor({ state: 'visible' });
    return page;
  };
  const look = async (page, name) => {
    const seen = {
      folderProblem: await page.locator('#folder-problem').count(),
      emptyLibrary: await page.locator('#empty-library').count(),
      text: (await page.locator('#folder-problem, #empty-library').first().innerText()).replace(/\s+/g, ' '),
      changeButton: await page.locator('#folder-problem-change').count(),
    };
    observations[name] = seen;
    if (out) await page.screenshot({ path: join(out, name + '.png') });
    return seen;
  };

  // A: the drive is not connected (browser window: no folder picker here, so it says where to go).
  const browserPage = await open(false);
  const a = await look(browserPage, 'unplugged-drive-browser');
  // B: the same in the desktop window, which can open the folder picker.
  const desktopPage = await open(true);
  const b = await look(desktopPage, 'unplugged-drive-desktop');
  if (b.changeButton) {
    await desktopPage.locator('#folder-problem-change').click();
    observations.changeSends = await desktopPage.waitForFunction(() => window.__titles.find(t => t.startsWith('__TH_CMD__:')), null, { timeout: 2000 }).then(h => h.jsonValue()).catch(() => '');
  }
  if (out) await writeFile(join(out, 'folder-missing-smoke.json'), JSON.stringify(observations, null, 2));
  // INVERT records the old code's wrong behaviour as evidence instead of failing on it.
  if (process.env.INVERT) {
    assert.equal(a.folderProblem, 0, 'old code: no folder message');
    assert.equal(a.emptyLibrary, 1, 'old code: a missing folder reads as the empty library');
    assert(a.text.includes('Your next favorite starts here'), 'old code: shows the empty-library wording');
    console.log('folder-missing smoke: OLD BEHAVIOUR CONFIRMED (missing folder reads as an empty library)');
    process.exitCode = 0;
  } else {
  assert.equal(a.emptyLibrary, 0, 'a missing folder must not read as an empty library');
  assert.equal(a.folderProblem, 1, 'the folder problem must be shown');
  assert(a.text.includes('/Volumes/' + drive + '/Music'), 'the folder is named');
  assert(a.text.includes('The drive “' + drive + '” is not connected'), 'the unplugged drive is named: ' + a.text);
  assert(a.text.includes('Profiles'), 'the browser window says where to change the folder');
  assert.equal(a.changeButton, 0, 'no dead folder button in a plain browser');
  assert.equal(b.changeButton, 1, 'the desktop window offers the folder picker');
  assert.equal(observations.changeSends, '__TH_CMD__:relocate-profile', 'the folder button reaches the relocate flow');
  await browserPage.locator('#folder-problem-retry').click();
  await browserPage.waitForFunction(() => document.getElementById('toast')?.textContent?.includes("Still can't open the music folder"));
  await stop();

  // C: a folder that went away comes back; Try again recovers without a restart.
  profile.library = music;
  await writeFile(profileFile, JSON.stringify(profile));
  await rename(music, music + '-away');
  await start();
  const page = await open(false);
  const c = await look(page, 'moved-folder');
  assert.equal(c.folderProblem, 1, 'a moved folder is reported');
  assert(c.text.includes('not there any more'), 'a moved folder is explained: ' + c.text);
  await rename(music + '-away', music);
  await page.locator('#folder-problem-retry').click();
  await page.waitForFunction(() => document.getElementById('toast')?.textContent?.includes('Music folder found'));
  await page.locator('#empty-library').waitFor({ state: 'visible' });
  assert.equal(await page.locator('#folder-problem').count(), 0, 'the problem clears once the folder is back');
  observations.recovered = true;
  if (out) await writeFile(join(out, 'folder-missing-smoke.json'), JSON.stringify(observations, null, 2));
  console.log('folder-missing smoke passed');
  }
} finally {
  await browser?.close();
  if (server && server.exitCode === null) await stop();
}
