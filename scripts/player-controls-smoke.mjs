import { chromium, expect } from "@playwright/test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, rename, writeFile } from "node:fs/promises";
import { tmpdir, homedir } from "node:os";
import { join, resolve, basename } from "node:path";
import { spawn } from "node:child_process";
import net from "node:net";

const socket = net.createServer();
await new Promise((r) => socket.listen(0, "127.0.0.1", r));
const port = socket.address().port;
await new Promise((r) => socket.close(r));
const data = await mkdtemp(join(tmpdir(), "music-player-027-"));
const output = process.env.MUSIC_PROOF_DIR || resolve("artifacts/player-027");
await mkdir(output, { recursive: true });
const proc = spawn(
  process.env.FIXTURE_SERVER || resolve("bin/fixture-server"),
  ["--data", data, "--port", String(port)],
  { stdio: ["ignore", "pipe", "pipe"] },
);
let logs = "",
  browser;
proc.stdout.on("data", (d) => (logs += d));
proc.stderr.on("data", (d) => (logs += d));
try {
  const origin = `http://127.0.0.1:${port}`;
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
  assert(ready, "fixture startup failed: " + logs);
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: { width: 1360, height: 900 },
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.addInitScript(() => {
    window.__mediaActions = {};
    if (!navigator.mediaSession) return;
    const original = navigator.mediaSession.setActionHandler.bind(
      navigator.mediaSession,
    );
    navigator.mediaSession.setActionHandler = (name, handler) => {
      window.__mediaActions[name] = handler;
      original(name, handler);
    };
  });
  await page.goto(origin);
  await page.locator("#username").fill("owner");
  await page.locator("#password").fill("123456");
  await page.getByRole("button", { name: /Open library/ }).click();
  await page.locator("#app").waitFor();
  await expect(page.locator(".player > .playback-actions")).toHaveCount(1, {
    timeout: 2000,
  });
  const wav = Buffer.alloc(44 + 960000);
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
  wav.writeUInt32LE(wav.length - 44, 40);
  for (let i = 0; i < 3; i++) {
    const audio = Buffer.from(wav);
    audio[audio.length - 1] = i + 1;
    const response = await page.request.post(origin + "/api/import", {
      multipart: {
        title: `Player proof ${i} with a long song title`,
        artist: "Keyboard and media proof",
        album: "Player proof",
        file: { name: `player-${i}.wav`, mimeType: "audio/wav", buffer: audio },
      },
    });
    assert(response.ok(), await response.text());
  }
  await page.reload();
  await page.getByRole("button", { name: "Play all", exact: true }).click();
  const audio = async (property) =>
    page.locator("#audio").evaluate((e, key) => e[key], property);
  const paused = (value) => expect.poll(() => audio("paused")).toBe(value);
  const near = (value) =>
    expect
      .poll(async () => Math.abs((await audio("currentTime")) - value))
      .toBeLessThan(0.5);
  const blur = () => page.evaluate(() => document.activeElement?.blur());
  await paused(false);
  await expect.poll(() => audio("currentTime")).toBeGreaterThan(0);
  await page.keyboard.press("Space");
  await paused(true);
  await page.keyboard.press("Space");
  await paused(false);
  await page.keyboard.press("Space");
  await paused(true);
  await page.locator("#audio").evaluate((e) => (e.currentTime = 20));
  await blur();
  await page.keyboard.press("ArrowRight");
  await near(30);
  await page.keyboard.press("ArrowLeft");
  await near(20);
  const first = await page.locator("#playing-title").textContent();
  await page.keyboard.press("Meta+ArrowRight");
  await expect(page.locator("#playing-title")).not.toHaveText(first);
  await page.keyboard.press("Meta+ArrowLeft");
  await expect(page.locator("#playing-title")).toHaveText(first);
  await page.locator("#audio").evaluate((e) => e.pause());
  await page.keyboard.press("Meta+ArrowUp");
  await expect.poll(() => audio("volume")).toBeCloseTo(0.85, 5);
  await page.keyboard.press("Meta+ArrowDown");
  await expect.poll(() => audio("volume")).toBeCloseTo(0.8, 5);
  await page.keyboard.press("m");
  await expect.poll(() => audio("muted")).toBe(true);
  await page.keyboard.press("m");
  await expect.poll(() => audio("muted")).toBe(false);
  await page.keyboard.press("s");
  await expect(page.locator("#shuffle-btn")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.keyboard.press("s");
  await expect(page.locator("#shuffle-btn")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  for (const mode of ["one", "off", "all"]) {
    await page.keyboard.press("r");
    await expect(page.locator("#repeat-btn")).toHaveAttribute(
      "aria-label",
      "Repeat mode: " + mode,
    );
  }
  await page.keyboard.press("l");
  await expect(page.locator("#lyrics-panel")).toBeVisible();
  await page.keyboard.press("l");
  await expect(page.locator("#lyrics-panel")).toHaveCount(0);
  await page.keyboard.press("q");
  await expect(page.locator(".player-queue")).toBeVisible();
  await page.keyboard.press("q");
  await expect(page.locator(".player-queue")).toHaveCount(0);

  await page.locator("#search").fill("typing ");
  await page.keyboard.type("mrs lqf ");
  await paused(true);
  assert.equal(await audio("muted"), false);
  await expect(page.locator("#shuffle-btn")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await expect(page.locator("#playing-title")).toHaveText(first);
  await expect(page.locator("#search")).toHaveValue("typing mrs lqf ");
  await page.locator("#search").fill("");
  await blur();
  await page.keyboard.press("l");
  await page.locator("#edit-lyrics-btn").click();
  await page.locator("#lyrics-textarea").fill("");
  await page.locator("#lyrics-textarea").pressSequentially("mrs lqf ");
  await expect(page.locator("#lyrics-textarea")).toHaveValue("mrs lqf ");
  await paused(true);
  await page.locator("#lyrics-btn").click();
  await page.locator("#playback-speed-btn").click();
  await page.locator("#playback-speed").selectOption("1.5");
  await expect.poll(() => audio("playbackRate")).toBe(1.5);
  await page.locator("#playback-speed").focus();
  await page.keyboard.press("m");
  assert.equal(await audio("muted"), false);
  await blur();
  await page.keyboard.press("Escape");
  await page.evaluate(() => {
    const e = document.createElement("div");
    e.id = "editable-guard";
    e.contentEditable = "true";
    document.body.append(e);
    e.focus();
  });
  await page.keyboard.type("mrs lqf ");
  await expect(page.locator("#editable-guard")).toHaveText("mrs lqf");
  await paused(true);
  await page.locator("#editable-guard").evaluate((e) => e.remove());
  await page.evaluate(() => {
    for (const options of [
      { key: "m", isComposing: true },
      { key: "s", repeat: true },
      { key: "m", altKey: true },
      { key: "s", ctrlKey: true },
    ])
      document.body.dispatchEvent(
        new KeyboardEvent("keydown", {
          ...options,
          bubbles: true,
          cancelable: true,
        }),
      );
  });
  assert.equal(await audio("muted"), false);
  await expect(page.locator("#shuffle-btn")).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await page.locator("#help-nav").click();
  await expect(
    page.getByRole("heading", { name: "Keyboard shortcuts" }),
  ).toBeVisible();
  await expect(page.locator(".keyboard-shortcuts > div")).toHaveCount(13);
  for (const [id, label] of [
    ["play-pause", "Space"],
    ["seek-back", "←"],
    ["next", "⌘→"],
    ["shuffle-btn", "S"],
    ["repeat-btn", "R"],
    ["lyrics-btn", "L"],
    ["queue-btn", "Q"],
    ["fullscreen-btn", "F"],
  ])
    assert(
      (await page.locator("#" + id).getAttribute("title")).includes(label),
      `missing shortcut tooltip: ${id}`,
    );
  await page.locator("#library-nav").click();
  await blur();
  await page.keyboard.press("f");
  await expect
    .poll(() => page.evaluate(() => !!document.fullscreenElement))
    .toBe(true);
  await page.keyboard.press("f");
  await expect
    .poll(() => page.evaluate(() => !!document.fullscreenElement))
    .toBe(false);

  for (const name of [
    "play",
    "pause",
    "nexttrack",
    "previoustrack",
    "seekbackward",
    "seekforward",
    "seekto",
  ])
    assert(
      await page.evaluate(
        (name) => typeof window.__mediaActions[name] === "function",
        name,
      ),
      `missing media action ${name}`,
    );
  const media = (action, extra = {}) =>
    page.evaluate(
      ({ action, extra }) =>
        window.__mediaActions[action]({ action, ...extra }),
      { action, extra },
    );
  await media("play");
  await paused(false);
  await media("pause");
  await paused(true);
  await media("nexttrack");
  await expect(page.locator("#playing-title")).not.toHaveText(first);
  await media("previoustrack");
  await expect(page.locator("#playing-title")).toHaveText(first);
  await page.locator("#audio").evaluate((e) => e.pause());
  await expect.poll(() => audio("duration")).toBe(60);
  await media("seekto", { seekTime: 20 });
  await near(20);
  await media("seekforward");
  await near(30);
  await media("seekbackward");
  await near(20);
  assert.equal(
    await page.evaluate(() => navigator.mediaSession.metadata.title),
    first,
  );
  assert.equal(
    await page.evaluate(() => navigator.mediaSession.playbackState),
    "paused",
  );
  await expect(
    page.getByText("Playback unavailable. Check the file and try again.", {
      exact: true,
    }),
  ).toHaveCount(0);
  await page.locator("#playing-favourite").click();
  await expect(page.locator("#playing-favourite")).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.locator("#volume-btn").click();
  await expect(page.locator("#volume")).toBeVisible();
  await page.locator("#mute-btn").click();
  assert.equal(await audio("muted"), true);
  await page.locator("#mute-btn").click();
  await page.keyboard.press("Escape");
  const layouts = [];
  for (const width of [1720, 1360, 1000, 760, 700, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await page.mouse.move(1, 1);
    const layout = await page.evaluate(() => {
      const rect = (e) => {
        const r = e.getBoundingClientRect();
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
      player.scrollLeft = 0;
      const buttons = [...player.querySelectorAll("button")].map((e) => ({
        id: e.id,
        ...rect(e),
      }));
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
      return {
        width: innerWidth,
        player: rect(player),
        buttons,
        collisions,
        overflow: document.documentElement.scrollWidth > innerWidth,
        scrollWidth: player.scrollWidth,
        clientWidth: player.clientWidth,
        transport: [
          ...document.querySelectorAll(".player > .playback-actions > button"),
        ].map((e) => e.id),
        extras: [
          ...document.querySelectorAll(".player > .player-extras > button"),
        ].map((e) => e.id),
        title: rect(document.querySelector("#playing-title")),
        progress: rect(document.querySelector("#seek")),
        elapsed: rect(document.querySelector("#elapsed")),
      };
    });
    assert.deepEqual(layout.transport, [
      "shuffle-btn",
      "previous",
      "seek-back",
      "play-pause",
      "seek-forward",
      "next",
      "repeat-btn",
    ]);
    assert.deepEqual(layout.extras, [
      "playing-favourite",
      "playback-speed-btn",
      "lyrics-btn",
      "queue-btn",
      "volume-btn",
      "fullscreen-btn",
      "pin-window-btn",
      "player-window-btn",
    ]);
    assert.deepEqual(layout.collisions, []);
    assert(!layout.overflow);
    assert(
      layout.buttons.every((b) => Math.abs(b.y - layout.buttons[0].y) < 1),
      `buttons must share one row at ${width}px: ${JSON.stringify(layout.buttons)}`,
    );
    assert(
      layout.buttons.every(
        (b) =>
          b.width >= (width > 680 ? 32 : 40) &&
          b.height >= (width > 680 ? 32 : 40),
      ),
    );
    assert(
      layout.elapsed.y >= layout.progress.bottom - 1,
      "times must sit under progress",
    );
    if (width > 680)
      assert(
        layout.buttons.every(
          (b) => b.x >= layout.player.x && b.right <= layout.player.right,
        ),
        "desktop controls must fit",
      );
    else {
      await page.locator("#player-window-btn").scrollIntoViewIfNeeded();
      assert(await page.locator("#player-window-btn").isVisible());
    }
    layouts.push(layout);
    if ([1360, 760].includes(width))
      await page.screenshot({
        path: join(output, `player-browser-${width}.png`),
        fullPage: true,
      });
  }
  await page.setViewportSize({ width: 420, height: 220 });
  await page.locator("#player-window-btn").click();
  await page.locator("#mini-player-btn").click();
  await expect(page.locator("html")).toHaveAttribute("data-mini", "true");
  await page.locator("#player-window-btn").click();
  await expect(page.locator("#mini-player-btn")).toBeInViewport();
  await page.locator("#mini-player-btn").click();
  await expect(page.locator("html")).toHaveAttribute("data-mini", "false");
  assert.deepEqual(errors, []);
  await writeFile(
    join(output, "player-result.json"),
    JSON.stringify(
      {
        keyboard: true,
        editableGuards: true,
        compositionAndRepeatGuards: true,
        fullscreen: true,
        mediaSession:
          "registered and dispatched callbacks; physical media keys not tested",
        miniExit: true,
        favourite: true,
        layouts,
        errors,
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    "PASS: player keyboard/typing guards, actual audio seek/volume, fullscreen, Media Session callback dispatch, Help/tooltips, favourite/options, mini exit and one-row responsive geometry",
  );
} finally {
  if (browser) await browser.close();
  if (proc.exitCode === null && proc.signalCode === null) {
    const exited = new Promise((r) => proc.once("exit", r));
    proc.kill("SIGINT");
    await exited;
  }
  const trash = join(homedir(), "Developer/_trash/tuneharbor/test-fixtures");
  await mkdir(trash, { recursive: true });
  await rename(data, join(trash, basename(data)));
}
