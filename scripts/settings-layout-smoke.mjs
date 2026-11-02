import { chromium, webkit, expect } from '@playwright/test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rename } from 'node:fs/promises';
import { tmpdir, homedir } from 'node:os';
import { join, resolve, basename } from 'node:path';
import { spawn, execFileSync } from 'node:child_process';
import net from 'node:net';

async function freePort() {
  const server = net.createServer();
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const port = server.address().port;
  await new Promise(r => server.close(r));
  return port;
}

export async function runSettingsLayoutSmoke({ baselineOnly = false } = {}) {
  const temp = await mkdtemp(join(tmpdir(), 'tuneharbor-layout-smoke-'));
  const data = join(temp, 'profiles');
  const music = join(temp, 'music');
  await mkdir(music, { recursive: true });
  const passwordFile = join(temp, 'password');
  await writeFile(passwordFile, '123456\n', { mode: 0o600 });

  const backendPort = await freePort();
  const executable = resolve('bin/tuneharbor');
  execFileSync(executable, [
    '--data', data,
    'profile', 'create',
    '--name', 'Personal',
    '--library', music,
    '--username', 'Owner',
    '--port', String(backendPort),
    '--password-file', passwordFile
  ]);

  const proc = spawn(executable, ['--data', data, 'serve'], { stdio: ['ignore', 'pipe', 'pipe'] });
  let logs = '';
  proc.stdout.on('data', d => logs += d);
  proc.stderr.on('data', d => logs += d);

  let browser;

  try {
    const backendUrl = 'http://127.0.0.1:' + backendPort;
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try {
        if ((await fetch(backendUrl)).ok) { ready = true; break; }
      } catch {}
      await new Promise(r => setTimeout(r, 100));
    }
    assert(ready, 'Backend did not start: ' + logs);

    const appUrl = backendUrl;

    // Detect browser engines to test
    const engines = [{ name: 'Chromium', type: chromium }];
    try {
      const testWk = await webkit.launch({ headless: true });
      await testWk.close();
      engines.push({ name: 'WebKit', type: webkit });
    } catch {
      console.log('Note: Playwright WebKit binary not present; testing with Chromium (WebKit tested when installed).');
    }

    const tabs = ['library', 'profile', 'playback', 'appearance', 'server', 'session'];

    for (const { name: engineName, type: engine } of engines) {
      console.log(`\nTesting ${engineName}...`);
      browser = await engine.launch({ headless: true, args: engineName === 'Chromium' ? ['--no-sandbox'] : [] });

      for (const isMobile of [false, true]) {
        const viewport = isMobile ? { width: 390, height: 844 } : { width: 1360, height: 920 };
        const label = `${engineName} ${isMobile ? 'Mobile (390x844)' : 'Desktop (1360x920)'}`;
        console.log(`\n--- Viewport: ${label} ---`);

        const context = await browser.newContext({ viewport });
        const page = await context.newPage();
        const errors = [];
        page.on('pageerror', e => errors.push(e.message));

        await page.goto(appUrl);
        await page.locator('#username').fill('OWNER');
        await page.locator('#password').fill('123456');
        await page.getByRole('button', { name: 'Open library' }).click();
        await page.locator('#app').waitFor({ state: 'visible' });

        // Navigate to settings
        await page.locator('#settings-nav').click();
        await page.locator('#settings-page').waitFor({ state: 'visible' });

        // Check horizontal overflow
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
        assert(!overflow, `${label}: page has horizontal scroll overflow`);

        const measurements = [];

        for (const tab of tabs) {
          // Click tab
          const tabBtn = page.locator('.settings-tabs button').filter({
            hasText: new RegExp(
              tab === 'library' ? 'Library' :
              tab === 'profile' ? 'Profile' :
              tab === 'playback' ? 'Playback' :
              tab === 'appearance' ? 'Appearance' :
              tab === 'server' ? 'Streaming' :
              'Session', 'i')
          }).first();
          await tabBtn.click();
          await expect(tabBtn).toHaveAttribute('aria-pressed', 'true');
          await expect(page.locator('.settings-tabs button.selected')).toHaveCount(1);
          await expect(page.locator(`#${tab}-settings-card`)).toBeVisible();
          await page.waitForTimeout(50);

          const titleBox = await page.locator('#page-title').boundingBox();
          const tabsBox = await page.locator('.settings-tabs').boundingBox();
          const firstCard = await page.locator('.settings-card').first().boundingBox();
          const serverCardInDom = await page.locator('#server-settings-card').isVisible();

          measurements.push({
            tab,
            titleY: titleBox ? Math.round(titleBox.y) : null,
            tabsY: tabsBox ? Math.round(tabsBox.y) : null,
            cardY: firstCard ? Math.round(firstCard.y) : null,
            serverCardInDom
          });

          console.log(`[${tab.padEnd(10)}] Title Y: ${measurements.at(-1).titleY}px | Tabs Y: ${measurements.at(-1).tabsY}px | First Card Y: ${measurements.at(-1).cardY}px | Server Card: ${serverCardInDom}`);
        }

        // Test switching from scrolled tab
        const libraryTabBtn = page.locator('.settings-tabs button').filter({ hasText: /Library/i }).first();
        await libraryTabBtn.click();
        await page.evaluate(() => window.scrollTo(0, 200));
        const profileTabBtn = page.locator('.settings-tabs button').filter({ hasText: /Profile/i }).first();
        await profileTabBtn.click();
        await page.waitForTimeout(50);
        const switchedTitle = await page.locator('#page-title').boundingBox();
        const initialTitleY = measurements.find(m => m.tab === 'profile').titleY;
        console.log(`After scroll switch -> Profile Title Y: ${Math.round(switchedTitle.y)}px (expected: ${initialTitleY}px)`);

        if (!baselineOnly) {
          assert(Math.abs(switchedTitle.y - initialTitleY) <= 2, `${label}: scrolled category switch moved heading`);
          // Assertions for FIXED state
          const titleYs = measurements.map(m => m.titleY);
          const maxTitleDiff = Math.max(...titleYs) - Math.min(...titleYs);
          assert(maxTitleDiff <= 2, `${label}: Header Y position moved across tabs: ${JSON.stringify(titleYs)}`);

          const tabsYs = measurements.map(m => m.tabsY);
          const maxTabsDiff = Math.max(...tabsYs) - Math.min(...tabsYs);
          assert(maxTabsDiff <= 2, `${label}: Settings tab strip Y position moved across tabs: ${JSON.stringify(tabsYs)}`);

          const cardYs = measurements.map(m => m.cardY);
          const maxCardDiff = Math.max(...cardYs) - Math.min(...cardYs);
          assert(maxCardDiff <= 2, `${label}: First settings card Y position moved across tabs: ${JSON.stringify(cardYs)}`);

          // Appearance tab must NOT show server card
          const appearanceMeasurement = measurements.find(m => m.tab === 'appearance');
          assert.equal(appearanceMeasurement.serverCardInDom, false, `${label}: appearance tab wrongly includes server card`);

          // Server tab MUST show server card
          const serverMeasurement = measurements.find(m => m.tab === 'server');
          assert.equal(serverMeasurement.serverCardInDom, true, `${label}: server tab should show server card`);

          // Check tab font size >= 13px
          const tabFontSize = await page.locator('.settings-tabs button').first().evaluate(el => {
            return parseFloat(window.getComputedStyle(el).fontSize);
          });
          assert(tabFontSize >= 13, `${label}: settings tab font size (${tabFontSize}px) is too small (< 13px)`);

          // Check input & select heights
          const appearanceTabBtn = page.locator('.settings-tabs button').filter({ hasText: /Appearance/i }).first();
          await appearanceTabBtn.click();
          const selectBox = await page.locator('#theme').boundingBox();

          await profileTabBtn.click();
          const inputBox = await page.locator('#pref-profile-name').boundingBox();

          assert(selectBox && selectBox.height >= 38, `${label}: select height (${selectBox?.height}px) is too short (< 38px)`);
          assert(inputBox && inputBox.height >= 38, `${label}: input height (${inputBox?.height}px) is too short (< 38px)`);
          assert(Math.abs(selectBox.height - inputBox.height) <= 6, `${label}: select height (${selectBox.height}px) does not match input height (${inputBox.height}px)`);
        }

        assert.deepEqual(errors, [], `${label}: browser errors`);
        // Take screenshots
        await expect(profileTabBtn).toHaveAttribute('aria-pressed', 'true');
        await page.screenshot({ animations: 'disabled', path: resolve(`artifacts/${baselineOnly ? 'baseline' : 'fixed'}-settings-${isMobile ? 'mobile' : 'desktop'}.png`) });

        await context.close();
      }

      await browser.close();
      browser = null;
    }

    console.log(`\nPASS: ${baselineOnly ? 'Baseline check complete' : 'Settings layout stability verification passed'}`);
  } finally {
    if (browser) await browser.close();
    if (proc.exitCode === null && proc.signalCode === null) {
      const exited = new Promise(r => proc.once('exit', r));
      proc.kill('SIGTERM');
      await exited;
    }
    const trash = join(homedir(), "Developer/_trash/tuneharbor/test-fixtures");
    await mkdir(trash, { recursive: true });
    await rename(temp, join(trash, basename(temp)));
  }
}

// When run directly
if (process.argv[1] && resolve(process.argv[1]) === resolve('scripts/settings-layout-smoke.mjs')) {
  const baselineOnly = process.argv.includes('--baseline');
  runSettingsLayoutSmoke({ baselineOnly }).catch(err => {
    console.error('FAIL:', err);
    process.exit(1);
  });
}
