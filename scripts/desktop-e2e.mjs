// Drives the installed desktop app through tauri-driver (WebDriver) on Linux:
// first-run profile creation, opening the library window, importing and
// playing a track, close-to-tray, and server cleanup when the app exits.
// Run through scripts/desktop-e2e.sh, which provides the display, D-Bus
// session, unlocked keyring and an isolated configuration directory.
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtemp, mkdir, readdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const driver = process.env.TAURI_DRIVER_URL || "http://127.0.0.1:4444";
const application = process.env.TUNEHARBOR_APP || "/usr/bin/tuneharbor";
const shots = process.env.TUNEHARBOR_SCREENSHOTS || "artifacts";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function wd(method, path, body) {
  const response = await fetch(driver + path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const value = (await response.json()).value;
  if (!response.ok) throw new Error(`${method} ${path}: ${value?.message || response.status}`);
  return value;
}
const ELEMENT = "element-6066-11e4-a52e-4f735466cecf";
async function until(what, check, timeout = 20000) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    try {
      const value = await check();
      if (value) return value;
    } catch (e) {
      last = e;
    }
    await sleep(200);
  }
  throw new Error(`Timed out waiting for ${what}${last ? ": " + last.message : ""}`);
}
function servers() {
  try {
    // Only this run's server: its data lives under the isolated config directory.
    const pattern = "tuneharbor-server --data " + (process.env.XDG_CONFIG_HOME || "") + ".* desktop";
    return execFileSync("pgrep", ["-f", pattern], { encoding: "utf8" }).trim().split("\n").filter(Boolean);
  } catch {
    return [];
  }
}

const temp = await mkdtemp(join(tmpdir(), "tuneharbor-desktop-"));
const music = join(temp, "Music");
await mkdir(music);
await mkdir(shots, { recursive: true });
const session = await wd("POST", "/session", {
  capabilities: { alwaysMatch: { "tauri:options": { application } } },
});
const s = "/session/" + session.sessionId;
const run = (script, args = []) => wd("POST", s + "/execute/sync", { script, args });
const find = (css) => wd("POST", s + "/element", { using: "css selector", value: css }).then((e) => e[ELEMENT]);
const click = async (css) => wd("POST", `${s}/element/${await until(css, () => find(css))}/click`, {});
const type = async (css, text) => {
  const id = await until(css, () => find(css));
  await wd("POST", `${s}/element/${id}/clear`, {});
  await wd("POST", `${s}/element/${id}/value`, { text });
};
const byText = (selector, text) =>
  run(
    "return [...document.querySelectorAll(arguments[0])].some(e => e.textContent.includes(arguments[1]))",
    [selector, text],
  );
const clickText = (selector, text) =>
  until(`${selector} "${text}"`, () =>
    run(
      "const e=[...document.querySelectorAll(arguments[0])].find(e=>e.textContent.includes(arguments[1])&&!e.disabled);if(e){e.click();return true}return false",
      [selector, text],
    ),
  );
const screenshot = async (name) =>
  writeFile(join(shots, name), Buffer.from(await wd("GET", s + "/screenshot"), "base64"));

let passed = false;
try {
  // First launch: the profile manager is the main window and starts its server sidecar.
  await until("profile manager", () => byText("h1", "A harbor for every collection."));
  await until("server sidecar", () => servers().length === 1, 30000);
  await screenshot("desktop-manager.png");

  // Choose the folder in the real native dialog by typing its path (GTK file chooser).
  await clickText("button", "Create");
  await until("create form", () => byText("h2", "Create a music profile"));
  await clickText("button", "Browse");
  const dialog = await until("folder dialog", () =>
    execFileSync("xdotool", ["search", "--onlyvisible", "--name", "Choose your music folder"], { encoding: "utf8" }).trim().split("\n")[0],
  );
  execFileSync("xdotool", ["windowactivate", "--sync", dialog]);
  // Typing "/" opens GTK's location bar but can drop the next keystroke, so retype the whole path.
  execFileSync("xdotool", ["type", "/"]);
  await sleep(500);
  execFileSync("xdotool", ["key", "ctrl+a"]);
  execFileSync("xdotool", ["type", "--delay", "20", music + "/"]);
  await sleep(500);
  execFileSync("xdotool", ["key", "Return"]);
  await until("chosen folder", () => run("return [...document.querySelectorAll('input[readonly]')].some(i => i.value === arguments[0])", [music]));
  await type("input[type=password]", "123456");
  await clickText("button[type=submit]", "Create & listen");

  // Creating a profile opens its library window, signed in through the private bootstrap.
  const manager = await wd("GET", s + "/window");
  const library = await until("library window", async () =>
    (await wd("GET", s + "/window/handles")).find((h) => h !== manager),
  );
  await wd("POST", s + "/window", { handle: library });
  await until("signed-in library", () => run("const a=document.getElementById('app');return !!a && !a.hidden && !!document.getElementById('music-files')"));
  const files = await readdir(music);
  assert.ok(files.some((f) => f.startsWith(".tuneharbor-library-id")), "library marker created in music folder");

  // Import a short WAV through the page's file input and play it.
  const wav = Buffer.alloc(44 + 16000);
  wav.write("RIFF"); wav.writeUInt32LE(wav.length - 8, 4); wav.write("WAVEfmt ", 8); wav.writeUInt32LE(16, 16);
  wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22); wav.writeUInt32LE(8000, 24); wav.writeUInt32LE(16000, 28);
  wav.writeUInt16LE(2, 32); wav.writeUInt16LE(16, 34); wav.write("data", 36); wav.writeUInt32LE(16000, 40);
  const fixture = join(temp, "Desktop test.wav");
  await writeFile(fixture, wav);
  const input = await find("#music-files");
  await wd("POST", `${s}/element/${input}/value`, { text: fixture });
  await until("imported track", () => run("return !!document.querySelector('[aria-label=\"Play Desktop test\"]')"));
  assert.ok((await readdir(music)).includes("Desktop test.wav"), "import copied the file into the music folder");
  await click('[aria-label="Play Desktop test"]');
  await until("playback", () => run("const a=document.querySelector('audio');return !!a && a.currentTime > 0"));
  await screenshot("desktop-library.png");

  // Closing the manager hides it to the tray; the server keeps running.
  // Close the manager as a user would, through the window manager (Alt+F4).
  const managerWindow = execFileSync("xdotool", ["search", "--onlyvisible", "--name", "^Music Studio KH$"], { encoding: "utf8" }).trim().split("\n")[0];
  execFileSync("xdotool", ["windowactivate", "--sync", managerWindow]);
  execFileSync("xdotool", ["key", "--clearmodifiers", "alt+F4"]);
  await until("tray explanation on first close", () =>
    execFileSync("xdotool", ["search", "--onlyvisible", "--name", "Music Studio KH keeps playing in the background"], { encoding: "utf8" }).trim(),
  );
  assert.equal(servers().length, 1, "server keeps running after closing the window");
  passed = true;
} finally {
  if (!passed) {
    try { execFileSync("import", ["-window", "root", join(shots, "desktop-failure.png")]); } catch {}
  }
  await wd("DELETE", s).catch(() => {});
}
// Ending the session terminates the app; its server must not outlive it.
await until("server shutdown", () => servers().length === 0, 30000);
assert.ok(passed);
console.log("PASS: desktop first run, folder selection, profile creation, signed-in library window, import, playback, close-to-tray, and server cleanup on exit.");
