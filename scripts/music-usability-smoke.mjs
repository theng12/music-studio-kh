import { chromium, expect } from "@playwright/test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  writeFile,
  readFile,
  rename,
  stat,
} from "node:fs/promises";
import { tmpdir, homedir } from "node:os";
import { join, resolve, basename } from "node:path";
import { spawn } from "node:child_process";
import net from "node:net";

const socket = net.createServer();
await new Promise((r) => socket.listen(0, "127.0.0.1", r));
const port = socket.address().port;
await new Promise((r) => socket.close(r));
const temp = await mkdtemp(join(tmpdir(), "music-usability-"));
const output = process.env.MUSIC_PROOF_DIR || resolve("artifacts/music-025");
await mkdir(output, { recursive: true });
let logs = "";
const startServer = () => {
  const child = spawn(
    resolve("bin/fixture-server"),
    ["--data", temp, "--port", String(port)],
    { stdio: ["ignore", "pipe", "pipe"] },
  );
  child.stdout.on("data", (d) => (logs += d));
  child.stderr.on("data", (d) => (logs += d));
  return child;
};
let proc = startServer();
const stopServer = async () => {
  if (proc.exitCode === null && proc.signalCode === null) {
    const exited = new Promise((r) => proc.once("exit", r));
    proc.kill("SIGINT");
    await exited;
  }
};
let browser;
try {
  const origin = "http://127.0.0.1:" + port;
  let ready = false;
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(origin)).ok) {
        ready = true;
        break;
      }
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  assert(ready, "isolated fixture server did not start: " + logs);
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: { width: 1360, height: 920 },
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(origin);
  await page.locator("#username").fill("owner");
  await page.locator("#password").fill("123456");
  await page.getByRole("button", { name: /Open library/ }).click();
  await page.locator("#app").waitFor();
  await expect(
    page.getByRole("button", { name: "Play favourites", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Play most played", exact: true }),
  ).toBeDisabled();
  const wav = Buffer.alloc(44 + 64000);
  wav.write("RIFF");
  wav.writeUInt32LE(wav.length - 8, 4);
  wav.write("WAVEfmt ", 8);
  wav.writeUInt32LE(16, 16);
  wav.writeUInt16LE(1, 20);
  wav.writeUInt16LE(1, 22);
  wav.writeUInt32LE(8000, 24);
  wav.writeUInt32LE(16000, 28);
  wav.writeUInt16LE(2, 32);
  wav.writeUInt16LE(16, 34);
  wav.write("data", 36);
  wav.writeUInt32LE(64000, 40);
  const imported = [];
  for (let i = 0; i < 3; i++) {
    const audio = Buffer.from(wav);
    audio[audio.length - 1] = i + 1;
    const response = await page.request.post(origin + "/api/import", {
      multipart: {
        title: "Proof song " + i,
        artist: i === 2 ? "Second artist" : "First artist",
        album: i === 2 ? "Second album" : "First album",
        file: {
          name: "Proof song " + i + ".wav",
          mimeType: "audio/wav",
          buffer: audio,
        },
      },
    });
    assert(response.ok(), await response.text());
    imported.push(await response.json());
  }
  await page.reload();
  await page.locator("#app").waitFor();
  await expect(page.locator("#library-page .new-badge")).toHaveCount(3);
  const layouts = [];
  for (const width of [
    1720, 1360, 1250, 1150, 1000, 768, 681, 680, 480, 375, 320,
  ]) {
    await page.setViewportSize({ width, height: 920 });
    const layout = await page.evaluate(() => {
      const rect = (element) => {
        const r = element.getBoundingClientRect();
        return {
          x: r.x,
          y: r.y,
          right: r.right,
          bottom: r.bottom,
          width: r.width,
          height: r.height,
        };
      };
      const player = document.querySelector(".player");
      const buttons = [...player.querySelectorAll("button, select, input")]
        .filter(
          (element) =>
            element.getClientRects().length &&
            getComputedStyle(element).visibility !== "hidden",
        )
        .map((element) => ({ id: element.id, ...rect(element) }));
      const collisions = buttons.flatMap((a, i) =>
        buttons
          .slice(i + 1)
          .filter(
            (b) =>
              Math.min(a.right, b.right) - Math.max(a.x, b.x) > 1 &&
              Math.min(a.bottom, b.bottom) - Math.max(a.y, b.y) > 1,
          )
          .map((b) => [a.id, b.id]),
      );
      const labelsFit = ["lyrics-btn", "queue-btn"].every((id) => {
        const button = document.getElementById(id);
        const label = button.querySelector("span");
        if (!label || !label.getClientRects().length) return true;
        const a = rect(button),
          b = rect(label);
        return b.x >= a.x && b.right <= a.right;
      });
      const hintsInsideActions =
        document.querySelector(".quick-actions .muted") !== null;
      const heading = rect(document.querySelector("#library-page h2"));
      const add = rect(document.querySelector("#choose-music"));
      return {
        width: innerWidth,
        player: rect(player),
        buttons,
        collisions,
        labelsFit,
        hintsInsideActions,
        headingAligned:
          Math.abs(heading.y + heading.height / 2 - add.y - add.height / 2) < 2,
        overflow: document.documentElement.scrollWidth > innerWidth,
      };
    });
    assert(
      layout.labelsFit,
      `player labels escape their buttons at ${width}px`,
    );
    assert.deepEqual(
      layout.collisions,
      [],
      `player controls overlap at ${width}px`,
    );
    assert(!layout.overflow, `page overflows at ${width}px`);
    assert(
      !layout.hintsInsideActions,
      "empty-state hints disrupt the quick-play button row",
    );
    if (width >= 1360)
      assert(
        layout.headingAligned,
        "library heading and toolbar controls do not align",
      );
    assert(
      layout.player.height <= (width > 1250 ? 120 : width > 680 ? 160 : 212),
      `player is too tall at ${width}px`,
    );
    if (width > 680)
      assert(
        layout.buttons.every(
          (b) =>
            b.x >= layout.player.x - 1 && b.right <= layout.player.right + 1,
        ),
        `player controls escape surface at ${width}px`,
      );
    else
      assert.equal(
        layout.buttons.length,
        16,
        "phone player retains every control in its scrollable row",
      );
    layouts.push(layout);
    if ([1360, 1000, 375].includes(width))
      await page.screenshot({
        path: join(output, `layout-${width}.png`),
        fullPage: true,
      });
  }
  await writeFile(
    join(output, "layout-result.json"),
    JSON.stringify(layouts, null, 2),
  );
  await page.setViewportSize({ width: 1360, height: 920 });
  await page
    .getByRole("button", { name: "Add favourite: Proof song 0", exact: true })
    .click();
  await expect(
    page.getByRole("button", {
      name: "Remove favourite: Proof song 0",
      exact: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.reload();
  await page.locator("#app").waitFor();
  await expect(
    page.getByRole("button", {
      name: "Remove favourite: Proof song 0",
      exact: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await page
    .getByRole("button", { name: "Play favourites", exact: true })
    .click();
  await expect(page.locator("#playing-title")).toHaveText("Proof song 0");
  await page.waitForFunction(
    () => document.querySelector("audio")?.currentTime > 1,
  );
  await expect
    .poll(
      async () =>
        (await (await page.request.get(origin + "/api/tracks")).json()).find(
          (t) => t.id === imported[0].id,
        ).playCount,
      { timeout: 10000 },
    )
    .toBeGreaterThan(0);
  await page.locator("#play-pause").click();
  await page
    .getByRole("button", { name: "Play favourites", exact: true })
    .click();
  await page.waitForFunction(() => !document.querySelector("audio").paused);
  await page
    .getByRole("button", { name: "Play most played", exact: true })
    .click();
  await expect(page.locator("#playing-title")).toHaveText("Proof song 0");
  await page.getByRole("button", { name: "Shuffle all", exact: true }).click();
  await page.locator("#play-pause").click();
  await page.locator("#nav-albums").click();
  await page
    .getByRole("button", { name: "Play album First album", exact: true })
    .click();
  const firstTitle = await page.locator("#playing-title").textContent();
  await page.locator("#queue-btn").click();
  await expect(page.locator(".player-queue .album-song")).toHaveCount(2);
  assert.deepEqual(
    new Set(
      await page.locator(".player-queue .album-song strong").allTextContents(),
    ),
    new Set(["Proof song 0", "Proof song 1"]),
  );
  await page.locator(".player-queue .close-dialog").click();
  await page.locator("#next").click();
  await expect(page.locator("#playing-title")).not.toHaveText(firstTitle);
  await page.locator("#previous").click();
  await expect(page.locator("#playing-title")).toHaveText(firstTitle);
  await page.waitForFunction(
    () => document.querySelector("audio")?.currentTime > 0.5,
  );
  const beforeFavourite = await page
    .locator("#audio")
    .evaluate((e) => e.currentTime);
  await page.locator("#playing-favourite").click();
  await expect(page.locator("#playing-favourite")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  assert(
    (await page.locator("#audio").evaluate((e) => e.currentTime)) >=
      beforeFavourite,
    "favourite metadata must not reset audio",
  );
  await page.locator("#playing-favourite").click();
  await expect(page.locator("#playing-favourite")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  for (const [tab, label, count] of [
    ["nav-albums", "Shuffle album First album", 2],
    ["nav-artists", "Shuffle artist First artist", 2],
    ["nav-folders", "Shuffle folder Root library", 3],
  ]) {
    await page.locator("#" + tab).click();
    await page.getByRole("button", { name: label, exact: true }).click();
    await page.locator("#queue-btn").click();
    const rows = page.locator(".player-queue .album-song");
    await expect(rows).toHaveCount(count);
    const names = await rows.locator("strong").allTextContents();
    assert.equal(
      new Set(names).size,
      count,
      "scoped shuffle never duplicates songs",
    );
    if (count === 2)
      assert.deepEqual(
        new Set(names),
        new Set(["Proof song 0", "Proof song 1"]),
      );
    await page.locator(".player-queue .close-dialog").click();
  }
  await page.locator("#play-pause").click();
  for (const tab of [
    "nav-albums",
    "nav-artists",
    "nav-folders",
    "library-nav",
  ]) {
    await page.locator("#" + tab).click();
    assert.equal(await page.locator("#library-page").count(), 1);
    await page.screenshot({ path: join(output, tab + ".png"), fullPage: true });
  }
  await page.locator("#downloads-nav").click();
  await page
    .locator("#download-url")
    .fill("https://www.youtube.com/watch?v=A_HekkBbd1M");
  await page.getByRole("button", { name: "Add to queue", exact: true }).click();
  await page.locator('.queue-card[data-state="completed"]').waitFor();
  const tracksBefore = await (
    await page.request.get(origin + "/api/tracks")
  ).json();
  const storage = await (
    await page.request.get(origin + "/api/storage")
  ).json();
  let actualBytes = 0;
  for (const track of tracksBefore) {
    const info = await stat(join(temp, "music", track.path));
    assert.equal(track.size, info.size);
    actualBytes += info.size;
  }
  assert.equal(storage.indexedFiles, tracksBefore.length);
  assert.equal(storage.indexedBytes, actualBytes);
  assert.equal(storage.rootAvailable, true);
  assert(
    tracksBefore.some((t) => t.downloadedAt > 0),
    "real fixture download records provenance",
  );
  await page.locator("#library-nav").click();
  await expect(page.locator("#library-page .new-badge")).toHaveCount(4);
  await page.locator("#downloads-nav").click();
  await page.screenshot({
    path: join(output, "downloads-before-clear.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Clear finished", exact: true })
    .click();
  await expect(page.locator('.queue-card[data-state="completed"]')).toHaveCount(
    0,
  );
  await page.reload();
  await page.locator("#app").waitFor();
  await page.locator("#downloads-nav").click();
  await expect(page.locator(".queue-card")).toHaveCount(0);
  await stopServer();
  proc = startServer();
  await expect
    .poll(
      async () => {
        try {
          return (await fetch(origin)).status;
        } catch {
          return 0;
        }
      },
      { timeout: 10000 },
    )
    .toBe(200);
  await page.reload();
  await page.locator("#username").fill("owner");
  await page.locator("#password").fill("123456");
  await page.getByRole("button", { name: /Open library/ }).click();
  await page.locator("#app").waitFor();
  assert.deepEqual(
    await (await page.request.get(origin + "/api/downloads")).json(),
    [],
  );
  const after = await (await page.request.get(origin + "/api/tracks")).json();
  const savedFavourite = after.find((t) => t.id === imported[0].id);
  assert.equal(
    savedFavourite.favourite,
    true,
    "favourite survives server restart",
  );
  assert(
    savedFavourite.playCount > 0,
    "listening history survives server restart",
  );
  assert.deepEqual(
    after.map((t) => t.id).sort(),
    tracksBefore.map((t) => t.id).sort(),
    "clearing history preserves tracks",
  );
  for (const track of after)
    assert.equal(
      (await stat(join(temp, "music", track.path))).size,
      track.size,
      "clearing history preserves audio bytes",
    );
  await page.locator("#settings-nav").click();
  await expect(page.locator("#storage-summary")).toContainText(
    String(storage.indexedFiles) + " indexed music files",
  );
  for (const title of [
    "Profile",
    "Playback",
    "Appearance",
    "Streaming",
    "Session",
  ])
    await page
      .locator(".settings-tabs button")
      .filter({ hasText: title })
      .click();
  await page
    .locator(".settings-tabs button")
    .filter({ hasText: "Appearance" })
    .click();
  const saved = () =>
    page.waitForResponse(
      (r) =>
        r.url().endsWith("/api/settings") &&
        r.request().method() === "PUT" &&
        r.ok(),
    );
  await page.locator("#recentDays").fill("0");
  await Promise.all([saved(), page.locator("#recentDays").blur()]);
  await Promise.all([
    saved(),
    page.locator("#reduceTransparency").selectOption("opaque"),
  ]);
  await expect(page.locator("html")).toHaveAttribute(
    "data-reduce-transparency",
    "true",
  );
  await page.locator("#library-nav").click();
  await expect(page.locator("#library-page .new-badge")).toHaveCount(0);
  await page
    .getByRole("button", { name: "Play favourites", exact: true })
    .click();
  await page.waitForFunction(
    () => document.querySelector("audio")?.currentTime > 0,
  );
  await page.locator("#play-pause").click();
  await page.screenshot({
    path: join(output, "songs-opaque.png"),
    fullPage: true,
  });
  const metrics = await page.evaluate(() => ({
    sidebarIcons: [...document.querySelectorAll(".nav-item > svg")].map(
      (e) => ({
        width: e.getBoundingClientRect().width,
        height: e.getBoundingClientRect().height,
        stroke: getComputedStyle(e).strokeWidth,
      }),
    ),
    transport: [...document.querySelectorAll(".playback-actions button")].map(
      (e) => ({
        label: e.getAttribute("aria-label") || e.title,
        width: e.getBoundingClientRect().width,
        height: e.getBoundingClientRect().height,
      }),
    ),
  }));
  assert(metrics.sidebarIcons.length >= 6);
  assert(metrics.transport.length >= 5);
  assert(metrics.sidebarIcons.every((i) => i.width === 20 && i.height === 20));
  assert(metrics.transport.every((b) => b.width >= 32 && b.height >= 32));
  await page.setViewportSize({ width: 390, height: 844 });
  assert(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "mobile must not overflow",
  );
  await page.screenshot({
    path: join(output, "songs-mobile.png"),
    fullPage: true,
  });
  assert.deepEqual(errors, []);
  await writeFile(
    join(output, "usability-result.json"),
    JSON.stringify(
      {
        storage,
        actualIndexedBytes: actualBytes,
        files: after.length,
        favouriteSurvivedReload: true,
        favouriteAndHistorySurvivedRestart: true,
        realImportNewBadgeAndOffSetting: true,
        playbackCountRecorded: true,
        historyClearedPersistently: true,
        audioAndTracksPreserved: true,
        metrics,
        errors,
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    "PASS: real UI favourites/history/quick play, indexed bytes match files, persistent history clear preserves audio/library, page navigation, icons and mobile layout",
  );
} finally {
  if (browser) await browser.close();
  await stopServer();
  const trash = join(homedir(), "Developer/_trash/tuneharbor/test-fixtures");
  await mkdir(trash, { recursive: true });
  await rename(temp, join(trash, basename(temp)));
}
