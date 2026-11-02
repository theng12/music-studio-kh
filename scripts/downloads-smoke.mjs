import { chromium } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, rename, realpath } from 'node:fs/promises';
import { tmpdir, homedir } from 'node:os';
import { join, resolve, basename } from 'node:path';
import { spawn } from 'node:child_process';
import net from 'node:net';

const socket = net.createServer();
await new Promise(r => socket.listen(0, '127.0.0.1', r));
const port = socket.address().port;
await new Promise(r => socket.close(r));
const temp = await mkdtemp(join(tmpdir(), 'tuneharbor-downloads-'));
const child = spawn(resolve('bin/fixture-server'), ['--data', temp, '--port', String(port)]);
child.stderr.on('data', d => process.stderr.write(d));
let browser;
try {
  const url = 'http://127.0.0.1:' + port;
  let ready = false;
  for (let i = 0; i < 100; i++) {
    try { if ((await fetch(url)).ok) { ready = true; break; } } catch {}
    await new Promise(r => setTimeout(r, 100));
  }
  assert(ready, 'fixture server unavailable');
  browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
  const page = await browser.newPage({ viewport: { width: 1360, height: 920 } });
  await page.goto(url);
  await page.locator('#username').fill('OWNER');
  await page.locator('#password').fill('123456');
  await page.getByRole('button', { name: 'Open library' }).click();
  await page.getByRole('button', { name: 'Downloads', exact: true }).click();
  await page.locator('#download-destination').waitFor({ timeout: 5000 });
  assert.equal(await page.locator('#download-destination').textContent(), await realpath(join(temp, 'music')), 'show the actual save location');
  await page.getByRole('heading', { name: 'Add from YouTube' }).waitFor();
  const add = page.getByRole('button', { name: 'Add to queue', exact: true });
  assert(await add.isDisabled(), 'empty link cannot be queued');
  await page.locator('#download-url').fill('https://youtu.be/abcdefghijk');
  await add.click();
  await page.locator('.queue-card[data-state="completed"]').waitFor();
  assert.equal(await page.locator('.queue-card').count(), 1);
  await page.locator('#library-nav').click();
  await page.getByRole('button', { name: 'Play Downloaded fixture', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Downloads', exact: true }).click();
  await page.locator('#download-url').fill('https://example.com/unsupported');
  await add.click();
  await page.waitForFunction(() => document.getElementById('toast').textContent.includes('YouTube'));
  assert.equal(await page.locator('.queue-card').count(), 1);

  // The runner fixture proves import/indexing above; this response exercises error presentation only.
  const reason = 'The video is unavailable. Check that the link opens on YouTube, then retry.';
  await page.route('**/api/downloads', async route => {
    if (route.request().method() !== 'GET') return route.continue();
    await route.fulfill({ json: [{ id: 'failed-fixture', title: 'Unavailable fixture', url: 'https://youtu.be/abcdefghijk', state: 'failed', progress: 0, error: reason, createdAt: Date.now() }] });
  });
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await page.getByText(reason, { exact: true }).waitFor();
  assert(await page.getByRole('button', { name: 'Retry', exact: true }).isVisible());
  await page.getByRole('button', { name: 'Needs attention', exact: true }).click();
  assert.equal(await page.locator('.queue-card').count(), 1);
  await page.getByRole('button', { name: 'Completed', exact: true }).click();
  assert.equal(await page.locator('.queue-card').count(), 0);
  await page.getByRole('button', { name: 'All downloads', exact: true }).click();
  await page.locator('#toast').evaluate(e => e.hidden = true);
  await mkdir('artifacts', { recursive: true });
  await page.screenshot({ path: 'artifacts/tuneharbor-downloads.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: 'artifacts/tuneharbor-downloads-mobile.png', fullPage: true });
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'downloads overflow');

  await page.route('**/api/me', route => route.fulfill({ status: 503, json: { error: 'Folder information unavailable' } }));
  await page.locator('#library-nav').click();
  await page.getByRole('button', { name: 'Downloads', exact: true }).click();
  await page.getByRole('alert').filter({ hasText: 'Could not load the save location: Folder information unavailable' }).waitFor();
  console.log('PASS: actual save location, honest destination errors, fixture queue completes and indexes automatically, unsupported links rejected, actionable failure/filter UI, mobile layout. Live YouTube is a separate check.');
} finally {
  if (browser) await browser.close();
  if (child.exitCode === null && child.signalCode === null) {
    const done = new Promise(r => child.once('exit', r));
    child.kill('SIGINT');
    await done;
  }
  const trash = join(homedir(), 'Developer/_trash/tuneharbor/test-fixtures');
  await mkdir(trash, { recursive: true });
  await rename(temp, join(trash, basename(temp)));
}
