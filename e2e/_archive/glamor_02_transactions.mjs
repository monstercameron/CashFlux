// GLAMOR G2 — Transactions visual review ("The Reconciler" / Nadia).
// Captures screenshots at 1280/1440/768 × light/dark for human/agent review.
// Saves to e2e/screenshots/ with names glamor_02_transactions_<width>_<theme>.png.
// Also captures a full-page shot at 1280 × dark and DOM text for structure analysis.
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
const THEMES = ["dark", "light"];

async function bootWithTheme(browser, width, theme) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 } });
  const page = await ctx.newPage();

  // Boot once to get WASM running, then inject theme + prefs + reload.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title], #app .bento, #app .w', { timeout: 60000 });

  // Inject theme via localStorage and full-prefs blob.
  await page.evaluate((theme) => {
    localStorage.setItem("cashflux:theme", JSON.stringify(theme));
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      if (raw) {
        const p = JSON.parse(raw);
        p.theme = theme;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      }
    } catch (_) {}
  }, theme);

  // Hard reload so WASM boots with new theme.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title], #app .bento, #app .w', { timeout: 60000 });

  // Reset "View as member" to Everyone before navigating.
  try {
    const memberSel = await page.$('select[aria-label*="member"], select[data-testid="member-switcher"]');
    if (memberSel) {
      await memberSel.selectOption({ index: 0 });
      await page.waitForTimeout(300);
    }
  } catch (_) {}

  // Navigate to /transactions.
  try {
    await page.locator('nav a[title="Transactions"]').first().click();
    await page.waitForTimeout(600);
  } catch (_) {
    await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(600);
  }

  await page.waitForTimeout(800);
  return { page, ctx };
}

const browser = await chromium.launch({ headless: true });

try {
  for (const theme of THEMES) {
    for (const width of WIDTHS) {
      const errors = [];
      const { page, ctx } = await bootWithTheme(browser, width, theme);
      page.on("pageerror", (e) => errors.push(String(e)));

      const shotPath = path.join(SHOTS_DIR, `glamor_02_transactions_${width}_${theme}.png`);
      await page.screenshot({ path: shotPath, fullPage: false });
      console.log(`wrote ${path.basename(shotPath)}`);

      // Full-page shot at 1280 dark for below-fold structure review.
      if (width === 1280 && theme === "dark") {
        const fullPath = path.join(SHOTS_DIR, `glamor_02_transactions_${width}_${theme}_full.png`);
        await page.screenshot({ path: fullPath, fullPage: true });
        console.log(`wrote ${path.basename(fullPath)}`);

        // DOM text harvest: column headers, row count, toolbar controls.
        const domInfo = await page.evaluate(() => {
          const headers = Array.from(document.querySelectorAll("th, .col-head")).map(el => el.innerText.trim()).filter(Boolean);
          const rows = document.querySelectorAll("tr[data-id]");
          const toolbar = document.querySelector(".toolbar, .filter-toolbar");
          const toolbarText = toolbar ? toolbar.innerText.trim() : "(no toolbar found)";
          const tableClass = document.querySelector("table") ? document.querySelector("table").className : "(no table)";
          const amountCells = Array.from(document.querySelectorAll(".td-amount")).slice(0, 5).map(el => el.innerText.trim());
          const chips = Array.from(document.querySelectorAll(".chip, [class*='chip']")).map(el => el.innerText.trim());
          const selectAllBtn = document.querySelector('[data-testid*="select-all"], button[title*="Select"]');
          return { headers, rowCount: rows.length, toolbarText, tableClass, amountCells, chips, selectAllBtnText: selectAllBtn?.innerText };
        });
        fs.writeFileSync(path.join(SHOTS_DIR, "glamor_02_transactions_dom.json"), JSON.stringify(domInfo, null, 2));
        console.log("wrote glamor_02_transactions_dom.json");
        console.log("DOM info:", JSON.stringify(domInfo, null, 2));
      }

      if (errors.length) {
        console.warn(`page errors at ${width}/${theme}: ${errors.join(" | ")}`);
      }
      await ctx.close();
    }
  }
  console.log("GLAMOR G2: all screenshots captured.");
} finally {
  await browser.close();
}
