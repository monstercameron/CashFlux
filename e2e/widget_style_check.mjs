// Widget Manager — tile styling (Phase 2): setting a tile style updates the live
// preview, persists, and overrides the global theme on the real dashboard tiles;
// Reset to theme clears it. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const PURPLE = "rgb(42, 26, 85)"; // #2a1a55
const setColor = (page, label, hex) =>
  page.locator(".wm-style-row").filter({ hasText: label }).locator('input[type=color]').first()
    .evaluate((el, v) => { el.value = v; el.dispatchEvent(new Event("change", { bubbles: true })); }, hex);
const cfgHas = (page, frag) => page.evaluate((f) => (localStorage.getItem("cashflux:widget-config") || "").includes(f), frag);

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(700);
  await page.locator('a[title="Widget manager"]').first().click();
  await page.waitForSelector(".wm-style", { timeout: 10000 });
  await page.waitForTimeout(300);

  // 1) Style "All widgets": set the background → live preview updates + persists.
  await setColor(page, "Background", "#2a1a55");
  await page.waitForTimeout(250);
  const previewBg = await page.evaluate(() => getComputedStyle(document.querySelector(".wm-preview-tile")).backgroundColor);
  if (previewBg !== PURPLE) fail(`preview did not update: bg=${previewBg}`);
  if (!(await cfgHas(page, '"_bg":"#2a1a55"'))) fail("background override was not persisted to the widget config");

  // 2) The override reaches the real dashboard tiles (global "_all" applies to all).
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento .w", { timeout: 10000 });
  await page.waitForTimeout(400);
  const tileBg = await page.evaluate(() => getComputedStyle(document.querySelector(".bento .w")).backgroundColor);
  if (tileBg !== PURPLE) fail(`dashboard tile did not pick up the global tile style: bg=${tileBg}`);

  // 3) Reset to theme clears it everywhere.
  await page.locator('a[title="Widget manager"]').first().click();
  await page.waitForSelector(".wm-style", { timeout: 10000 });
  await page.getByRole("button", { name: "Reset to theme" }).first().click();
  await page.waitForTimeout(300);
  if (await cfgHas(page, '"_bg":"#2a1a55"')) fail("Reset to theme did not clear the background override");
  const previewAfter = await page.evaluate(() => getComputedStyle(document.querySelector(".wm-preview-tile")).backgroundColor);
  if (previewAfter === PURPLE) fail("preview still shows the override after reset");

  if (!process.exitCode) console.log("PASS: tile style updates the live preview, overrides the dashboard tiles, and resets to theme.");
} finally {
  await browser.close();
}
