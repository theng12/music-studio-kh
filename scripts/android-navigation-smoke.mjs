import { chromium, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
const script = await readFile(
  new URL("../src-tauri/src/android-navigation.js", import.meta.url),
  "utf8",
);
const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  await page.goto("about:blank");
  await page.setContent(
    `<style>button,input{width:100px;height:50px} .row{display:flex;gap:30px} [hidden]{display:none}</style><div class="row"><button id="one">One</button><button id="two">Two</button></div><div><input id="text" value="typing"></div><div><button id="after-input">Connect</button></div><input id="range" type="range"><button id="hidden" hidden>Hidden</button>`,
  );
  await page.evaluate(script);
  await page.locator("#one").focus();
  let steals = 0;
  await page.exposeFunction("stoleArrow", () => steals++);
  await page.evaluate(() =>
    window.addEventListener("keydown", (e) => {
      if (e.key.startsWith("Arrow")) window.stoleArrow();
    }),
  );
  await page.keyboard.press("ArrowRight");
  await expect(page.locator("#two")).toBeFocused();
  if (steals !== 0) throw new Error("D-pad reached global seek shortcuts");
  await page.locator("#text").focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.locator("#text")).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(page.locator("#after-input")).toBeFocused();
  await page.locator("#range").focus();
  const value = await page.locator("#range").inputValue();
  await page.keyboard.press("ArrowRight");
  await expect(page.locator("#range")).not.toHaveValue(value);
  await page.evaluate(() => {
    const modal = document.createElement("div");
    modal.setAttribute("aria-modal", "true");
    modal.innerHTML =
      '<button id="close" class="close-dialog">Close</button><button id="inside">Inside</button>';
    modal.querySelector("button").onclick = () => modal.remove();
    document.body.append(modal);
  });
  await page.locator("#one").focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.locator("#close")).toBeFocused();
  await page.evaluate(() => window.__MUSIC_ANDROID_BACK__());
  await expect(page.locator('[aria-modal="true"]')).toHaveCount(0);
  const secured = await browser.newPage();
  await secured.route("https://music.test/**", async (route) => {
    const css = route.request().url().endsWith("app.css");
    await route.fulfill({
      contentType: css ? "text/css" : "text/html",
      headers: {
        "Content-Security-Policy":
          "default-src 'self'; style-src 'self'; script-src 'self'",
      },
      body: css
        ? ".player{--control-size:32px}.player button{width:var(--control-size);height:var(--control-size)}"
        : '<link rel="stylesheet" href="/app.css"><div class="player"><button id="target">Play</button></div>',
    });
  });
  await secured.addInitScript(script);
  await secured.goto("https://music.test/");
  await expect(secured.locator("#target")).toHaveCSS("width", "48px");
  await expect(secured.locator("#music-android-bar")).toHaveCSS(
    "height",
    "52px",
  );
  console.log(
    "PASS: directional focus, seek isolation, editable/range keys, modal Back and CSP-preserving Android styling",
  );
} finally {
  await browser.close();
}
