// Verifies per-widget colors (B20): open a tile's settings via its gear, set a
// Tile color, and assert the tile gets a colored top strip and persists; then
// Clear and assert it reverts. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const WID = "kpi-networth";
const CELL = `.w[data-widget="${WID}"]`;

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(CELL, { timeout: 60000 });

  // Every tile now has a gear; open this one's settings.
  await page.locator(`${CELL} button.gear-inline`).first().click();
  await page.waitForSelector('input[aria-label="Tile color"]', { timeout: 8000 });

  // Set the tile color to red (color inputs need value+change dispatched).
  await page.locator('input[aria-label="Tile color"]').evaluate((el) => {
    el.value = "#ff0000";
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await page.waitForTimeout(300);

  const set = await page.evaluate((sel) => {
    const cell = document.querySelector(sel);
    return {
      shadow: getComputedStyle(cell).boxShadow,
      stored: (JSON.parse(localStorage.getItem("cashflux:widget-config") || "{}"))["kpi-networth"]?._accent,
    };
  }, CELL);
  if (!/255,\s*0,\s*0/.test(set.shadow)) fail(`box-shadow = ${set.shadow}, want a red inset strip`);
  if (!/inset/.test(set.shadow)) fail(`box-shadow should be inset, got ${set.shadow}`);
  if (set.stored !== "#ff0000") fail(`stored _accent = ${set.stored}, want #ff0000`);

  // Clear it.
  await page.getByRole("button", { name: "Clear", exact: true }).click();
  await page.waitForTimeout(300);
  const cleared = await page.evaluate((sel) => ({
    shadow: getComputedStyle(document.querySelector(sel)).boxShadow,
    stored: (JSON.parse(localStorage.getItem("cashflux:widget-config") || "{}"))["kpi-networth"]?._accent,
  }), CELL);
  if (/255,\s*0,\s*0/.test(cleared.shadow)) fail(`after Clear, box-shadow still red: ${cleared.shadow}`);
  if (cleared.stored) fail(`_accent should be cleared, got ${cleared.stored}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: per-widget color tints the tile, persists, and clears.");
} finally {
  await browser.close();
}
