import { chromium, expect } from "@playwright/test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { resolve } from "node:path";

const serial = process.env.ANDROID_SERIAL || "emulator-5554";
assert(
  serial.startsWith("emulator-"),
  "Run this check only on an owned Android emulator",
);
const output =
  process.env.MUSIC_PROOF_DIR ||
  resolve(
    homedir(),
    "Desktop/All YouTube Stuff/Music Studio KH/Android/Emulator proof",
  );
await mkdir(output, { recursive: true });
const adb = (...args) => execFileSync("adb", ["-s", serial, ...args]);
const screenshot = async (name) => {
  await page.evaluate(
    () =>
      new Promise((done) =>
        requestAnimationFrame(() => requestAnimationFrame(done)),
      ),
  );
  await writeFile(resolve(output, name), adb("exec-out", "screencap", "-p"));
};
const endpoint = process.env.ANDROID_CDP || "http://127.0.0.1:9223";
async function connectWebView() {
  await expect
    .poll(
      async () => {
        try {
          const response = await fetch(`${endpoint}/json/list`);
          return response.ok && (await response.json()).length > 0;
        } catch {
          return false;
        }
      },
      { timeout: 10000 },
    )
    .toBe(true);
  return chromium.connectOverCDP(endpoint, {
    noDefaults: true,
    isWebView: true,
  });
}
let browser = await connectWebView();
let context = browser.contexts()[0];
let page = context.pages()[0];
const errors = [];
page.on("pageerror", (error) => errors.push(error.message));
const result = {
  emulator: adb("shell", "getprop", "ro.build.version.release")
    .toString()
    .trim(),
  api: adb("shell", "getprop", "ro.build.version.sdk").toString().trim(),
  abi: adb("shell", "getprop", "ro.product.cpu.abi").toString().trim(),
  physicalHardwareTested: false,
  checks: [],
};
try {
  const cdp = await context.newCDPSession(page);
  await cdp.send("Network.clearBrowserCookies");
  await cdp.detach();
  await page.goto("http://tauri.localhost/android.html");
  await expect(
    page.getByRole("heading", { name: "Listen from your Mac" }),
  ).toBeVisible();
  await expect(page.locator(".version-badge").first()).toHaveText(
    "Android · v0.2.8",
  );
  await expect(
    page.getByRole("button", { name: "Connect", exact: true }),
  ).toBeEnabled();
  await screenshot("01-connection-phone.png");
  await page.getByRole("button", { name: "What’s new", exact: true }).click();
  await expect(page.locator("#whats-new-dialog")).toBeVisible();
  await expect(page.locator(".whats-new-entry").first()).toContainText("0.2.8");
  await screenshot("02-whats-new-phone.png");
  adb("shell", "input", "keyevent", "4");
  await expect(page.locator("#whats-new-dialog")).toHaveCount(0);
  result.checks.push("APK launch, version, What’s new and native Android Back");
  await page.locator("#server").fill("http://unlisted.invalid:4800");
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText(
    "not included in this APK",
  );
  result.checks.push("native exact-host HTTP rejection");
  await page
    .locator("#server")
    .fill(process.env.ANDROID_SERVER || "http://10.0.2.2:4800");
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.locator("#username").waitFor();
  await page.locator("#username").fill("owner");
  await page.locator("#password").fill("123456");
  await page.getByRole("button", { name: /Open library/ }).click();
  await page.locator("#app").waitFor();
  await expect(page.locator("#music-android-bar")).toContainText(
    "Android · v0.2.8",
  );
  result.checks.push(
    "Mac fixture connection and normal authenticated library login",
  );

  const wav = Buffer.alloc(44 + 480000);
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
  for (let i = 0; i < 240000; i++)
    wav.writeInt16LE(
      Math.round(Math.sin((i * 2 * Math.PI * 440) / 8000) * 5000),
      44 + i * 2,
    );
  const login = await fetch("http://127.0.0.1:4800/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: "owner", password: "123456" }),
  });
  assert(login.ok, "fixture seed login");
  const cookie = login.headers
    .getSetCookie()
    .map((value) => value.split(";")[0])
    .join("; ");
  const form = new FormData();
  form.set("title", "Android emulator playback proof");
  form.set("artist", "Isolated fixture");
  form.set("file", new Blob([wav], { type: "audio/wav" }), "android-proof.wav");
  const imported = await fetch("http://127.0.0.1:4800/api/import", {
    method: "POST",
    headers: { Cookie: cookie },
    body: form,
  });
  assert(imported.ok, await imported.text());
  await page.reload();
  await page.locator("#play-all").click();
  await expect
    .poll(() => page.locator("#audio").evaluate((audio) => audio.currentTime))
    .toBeGreaterThan(0.2);
  const state = await page.locator("#audio").evaluate((audio) => ({
    currentTime: audio.currentTime,
    readyState: audio.readyState,
    error: audio.error?.message || null,
    src: audio.currentSrc,
    paused: audio.paused,
  }));
  assert(!state.paused && !state.error && state.readyState >= 2);
  await page.locator("#play-pause").click();
  result.audio = state;
  result.checks.push(
    "actual Android WebView PCM WAV decode and advancing foreground playback (virtual output, no physical audibility claim)",
  );

  await page.locator("#audio").evaluate((audio) => {
    audio.currentTime = 12;
  });
  await page.locator("#shuffle-btn").focus();
  adb("shell", "input", "keyevent", "22");
  await expect(page.locator("#previous")).toBeFocused();
  assert(
    Math.abs(
      (await page.locator("#audio").evaluate((audio) => audio.currentTime)) -
        12,
    ) < 0.5,
    "remote arrow sought audio instead of moving focus",
  );
  adb("shell", "input", "keyevent", "22");
  await expect(page.locator("#seek-back")).toBeFocused();
  adb("shell", "input", "keyevent", "23");
  await expect
    .poll(() => page.locator("#audio").evaluate((audio) => audio.currentTime))
    .toBeLessThan(3);
  result.checks.push(
    "ADB D-pad focus moves without global seeking; OK activates explicit seek button",
  );
  const player = await page.locator(".player").evaluate((node) => ({
    width: node.clientWidth,
    content: node.scrollWidth,
  }));
  assert(
    player.content <= player.width + 1,
    "phone player overflows horizontally",
  );
  await expect(page.locator("#music-android-bar")).toHaveCount(1);
  await screenshot("03-library-phone.png");
  await page.locator("#help-nav").click();
  adb("shell", "input", "keyevent", "4");
  await expect(page.locator("#library-nav")).toHaveClass(/active/);
  adb("shell", "input", "keyevent", "4");
  await expect(page.locator("#server")).toHaveValue(/10\.0\.2\.2/);
  await expect(
    page.getByRole("button", { name: "Connect", exact: true }),
  ).toBeEnabled();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Connect", exact: true }),
  ).toBeEnabled();
  result.checks.push(
    "Back from Help to Songs and library to remembered connection",
  );
  adb("shell", "wm", "size", "1920x1080");
  adb("shell", "wm", "density", "160");
  await browser.close();
  adb("shell", "am", "force-stop", "app.tuneharbor.android");
  adb(
    "shell",
    "am",
    "start",
    "-W",
    "-n",
    "app.tuneharbor.android/.MainActivity",
  );
  const pid = adb("shell", "pidof", "app.tuneharbor.android").toString().trim();
  adb("forward", "tcp:9223", `localabstract:webview_devtools_remote_${pid}`);
  browser = await connectWebView();
  context = browser.contexts()[0];
  page = context.pages()[0];
  page.on("pageerror", (error) => errors.push(error.message));
  await expect
    .poll(() => page.evaluate(() => innerWidth))
    .toBeGreaterThan(1000);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.locator("#username, #app").first().waitFor();
  if (await page.locator("#username").isVisible()) {
    await page.locator("#username").fill("owner");
    await page.locator("#password").fill("123456");
    await page.getByRole("button", { name: /Open library/ }).click();
  }
  await page.locator("#app").waitFor();
  await page.locator("#play-all").click();
  await page.locator("#shuffle-btn").focus();
  adb("shell", "input", "keyevent", "22");
  await expect(page.locator("#previous")).toBeFocused();
  await screenshot("04-library-landscape.png");
  result.checks.push(
    "1920x1080 landscape layout and ADB D-pad navigation on the same ARM64 emulator (not TV hardware)",
  );
  result.pageErrors = errors;
  assert.deepEqual(errors, []);
  result.passed = true;
} finally {
  adb("shell", "wm", "size", "reset");
  adb("shell", "wm", "density", "reset");
  await writeFile(
    resolve(output, "emulator-result.json"),
    JSON.stringify(result, null, 2) + "\n",
  );
  await browser.close();
}
console.log(JSON.stringify(result, null, 2));
