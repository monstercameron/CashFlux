// GLAMOR G3 — Accounts visual review ("The Net-Worth Check" / Theo).
// Captures screenshots at 1280/1440/768 × dark + light themes.
// Saves to e2e/screenshots/ with names glamor_03_accounts_<width>_<theme>.png.
// Also captures full-page shot at 1280 dark and DOM info.
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

  // Poll for data-theme attribute on <html> to confirm theme applied.
  let themeConfirmed = false;
  for (let i = 0; i < 30; i++) {
    const actual = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    if (theme === "light" && actual === "light") { themeConfirmed = true; break; }
    if (theme === "dark" && (actual === "dark" || actual === null)) { themeConfirmed = true; break; }
    await page.waitForTimeout(200);
  }
  if (!themeConfirmed) {
    console.warn(`Theme '${theme}' not confirmed on <html> data-theme — proceeding anyway.`);
  }

  // Reset "View as member" to Everyone.
  try {
    const memberSel = await page.$('select[aria-label*="member"], select[data-testid="member-switcher"]');
    if (memberSel) {
      await memberSel.selectOption({ index: 0 });
      await page.waitForTimeout(300);
    }
  } catch (_) {}

  // Navigate to /accounts.
  try {
    await page.locator('nav a[title="Accounts"]').first().click();
    await page.waitForTimeout(600);
  } catch (_) {
    try {
      await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(600);
    } catch (_2) {}
  }

  await page.waitForTimeout(800);
  return { page, ctx };
}

const browser = await chromium.launch({ headless: true });

try {
  // DARK theme first (default), then LIGHT.
  for (const theme of ["dark", "light"]) {
    for (const width of WIDTHS) {
      const errors = [];
      const { page, ctx } = await bootWithTheme(browser, width, theme);
      page.on("pageerror", (e) => errors.push(String(e)));

      const shotPath = path.join(SHOTS_DIR, `glamor_03_accounts_${width}_${theme}.png`);
      await page.screenshot({ path: shotPath, fullPage: false });
      console.log(`wrote ${path.basename(shotPath)}`);

      // Full-page shot at 1280 dark.
      if (width === 1280 && theme === "dark") {
        const fullPath = path.join(SHOTS_DIR, `glamor_03_accounts_${width}_${theme}_full.png`);
        await page.screenshot({ path: fullPath, fullPage: true });
        console.log(`wrote ${path.basename(fullPath)}`);

        // DOM info harvest.
        const domInfo = await page.evaluate(() => {
          const confirmed = document.documentElement.getAttribute("data-theme");
          const headings = Array.from(document.querySelectorAll("h1, h2, h3, .section-title, [class*='heading']")).map(el => el.innerText.trim()).filter(Boolean);
          const balances = Array.from(document.querySelectorAll("[class*='balance'], [class*='amount'], .td-amount")).slice(0, 20).map(el => el.innerText.trim()).filter(Boolean);
          const groups = Array.from(document.querySelectorAll("[class*='group'], [class*='section']")).map(el => el.className + ': ' + (el.innerText.trim().slice(0, 60))).slice(0, 10);
          const accountNames = Array.from(document.querySelectorAll("[class*='account-name'], .row-label, td:first-child")).slice(0, 15).map(el => el.innerText.trim()).filter(Boolean);
          const buttons = Array.from(document.querySelectorAll("button")).slice(0, 20).map(el => el.innerText.trim() || el.getAttribute("aria-label") || el.getAttribute("title")).filter(Boolean);
          const netWorth = document.querySelector("[class*='net-worth'], [class*='networth'], [data-testid*='net']");
          const netWorthText = netWorth ? netWorth.innerText.trim() : "(not found by selector)";
          const stale = Array.from(document.querySelectorAll("[class*='stale'], [class*='fresh'], [class*='outdated']")).map(el => el.innerText.trim());
          return { confirmed, headings, balances, groups, accountNames, buttons, netWorthText, stale };
        });
        fs.writeFileSync(path.join(SHOTS_DIR, "glamor_03_accounts_dom.json"), JSON.stringify(domInfo, null, 2));
        console.log("wrote glamor_03_accounts_dom.json");
        console.log("DOM info:", JSON.stringify(domInfo, null, 2));
      }

      if (errors.length) {
        console.warn(`page errors at ${width}/${theme}: ${errors.join(" | ")}`);
      }
      await ctx.close();
    }
  }
  console.log("GLAMOR G3: all screenshots captured.");
} finally {
  await browser.close();
}
