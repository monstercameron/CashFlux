// GLAMOR G1 — Dashboard visual review ("The 7am Glance").
// Captures screenshots at 1280/1440/768 × light/dark for human/agent review.
// Saves to e2e/screenshots/ with names glamor_01_dashboard_<width>_<theme>.png.
// Also captures a below-the-fold full-page shot at 1280 × light/dark.
// Not a pass/fail gate — purely a visual evidence harvest.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS_DIR = path.join(__dirname, "screenshots");
fs.mkdirSync(SHOTS_DIR, { recursive: true });

const WIDTHS = [1280, 1440, 768];
const THEMES = ["light", "dark"];

// The app persists prefs under cashflux:prefs (JSON) and theme override under
// cashflux:theme. Set both to ensure the correct theme loads on boot.
async function bootWithTheme(browser, width, theme) {
  const ctx = await browser.newContext({
    viewport: { width, height: 900 },
  });

  // Seed localStorage before the page loads so the app picks it up on boot.
  // Use a blank page first to set storage for the origin.
  const page = await ctx.newPage();
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });

  // Set the theme in localStorage and force a hard reload so WASM re-applies it.
  await page.evaluate((theme) => {
    // cashflux:theme is a standalone atom — the simplest toggle.
    localStorage.setItem("cashflux:theme", JSON.stringify(theme));
    // Also patch the full prefs blob if it exists, to keep them consistent.
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      if (raw) {
        const p = JSON.parse(raw);
        p.theme = theme;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      }
    } catch (_) {}
  }, theme);

  // Hard reload so the app boots fresh with the theme already set.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  // Wait for the dashboard bento grid (the app's home screen).
  await page.waitForSelector(
    'nav[aria-label="Main navigation"] a[title], #app .bento, #app .w',
    { timeout: 60000 }
  );
  // Navigate explicitly to Dashboard (in case localStorage had another route active).
  try {
    await page.locator('nav a[title="Dashboard"]').first().click();
    await page.waitForTimeout(600);
  } catch (_) {
    // Dashboard may already be active — that's fine.
  }

  await page.waitForTimeout(800); // let WASM render settle + FLIP animations finish

  return { page, ctx };
}

const browser = await chromium.launch({ headless: true });

try {
  for (const theme of THEMES) {
    for (const width of WIDTHS) {
      const errors = [];
      const { page, ctx } = await bootWithTheme(browser, width, theme);
      page.on("pageerror", (e) => errors.push(String(e)));

      const shotPath = path.join(
        SHOTS_DIR,
        `glamor_01_dashboard_${width}_${theme}.png`
      );
      await page.screenshot({ path: shotPath, fullPage: false });
      console.log(`wrote ${path.basename(shotPath)}`);

      // For 1280, also a full-page scroll to capture below-the-fold widgets.
      if (width === 1280) {
        const fullPath = path.join(
          SHOTS_DIR,
          `glamor_01_dashboard_${width}_${theme}_full.png`
        );
        await page.screenshot({ path: fullPath, fullPage: true });
        console.log(`wrote ${path.basename(fullPath)}`);
      }

      if (errors.length) {
        console.warn(`page errors at ${width}/${theme}: ${errors.join(" | ")}`);
      }

      await ctx.close();
    }
  }
  console.log("GLAMOR G1: all screenshots captured.");
} finally {
  await browser.close();
}
