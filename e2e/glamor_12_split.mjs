// GLAMOR G12 — Split page visual + structural review for "Who Owes Whom" (Priya).
// Takes screenshots at 1280, 1440, and 768 px in dark + light themes.
// Writes into e2e/screenshots/glamor_12_split_*.png and glamor_12_split_dom.json.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot = (name) => path.join(SHOTS, `glamor_12_split_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

async function navToSplit(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(600);
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try { const p = JSON.parse(raw); delete p.viewAsMember; localStorage.setItem("cashflux:prefs", JSON.stringify(p)); } catch (_) {}
    }
  });
  const splitLink = page.locator('nav a[title="Split"]').first();
  await splitLink.click();
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1000);
}

async function ensureMembers(page) {
  const cnt = await page.evaluate(() => document.querySelectorAll(".rows .row").length);
  if (cnt >= 3) return;
  const mlink = page.locator('nav a[title="Members"]').first();
  if (await mlink.count() === 0) return;
  await mlink.click();
  await page.waitForSelector(".card", { timeout: 10000 });
  await page.waitForTimeout(800);
  for (const name of ["Sam", "Lee"]) {
    const inp = page.locator('input[type="text"].field').first();
    if (await inp.count() === 0) break;
    await inp.fill(name);
    await page.waitForTimeout(200);
    const btn = page.locator('button[type="submit"]').first();
    if (await btn.count() > 0) { await btn.click(); await page.waitForTimeout(500); }
  }
  const slink = page.locator('nav a[title="Split"]').first();
  await slink.click();
  await page.waitForSelector(".card", { timeout: 10000 });
  await page.waitForTimeout(800);
}

async function fillExpense(page, shotName) {
  const amtInput = page.locator('input[type="number"]').first();
  await amtInput.fill("90");
  await page.waitForTimeout(200);
  const descInput = page.locator('input[type="text"]').first();
  await descInput.fill("Groceries for the week");
  await page.waitForTimeout(200);
  const saBtn = page.locator("button").filter({ hasText: /select all/i }).first();
  if (await saBtn.count() > 0) { await saBtn.click(); await page.waitForTimeout(400); }
  const sel = page.locator("select").first();
  const opts = await sel.locator("option").all();
  if (opts.length > 1) { const v = await opts[1].getAttribute("value"); if (v) await sel.selectOption(v); }
  await page.waitForTimeout(800);
  await page.screenshot({ path: shot(shotName) });
}

try {
  // DARK
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));
  await navToSplit(dark);
  await ensureMembers(dark);

  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("1280_dark_empty") });
  await dark.screenshot({ path: shot("1280_dark_empty_full"), fullPage: true });

  await fillExpense(dark, "1280_dark_filled");
  await dark.screenshot({ path: shot("1280_dark_filled_full"), fullPage: true });

  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("1440_dark_filled") });

  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(400);
  await dark.screenshot({ path: shot("768_dark_filled") });

  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.waitForTimeout(300);

  const domAudit = await dark.evaluate(() => {
    const cards = [...document.querySelectorAll(".card")];
    const cardTitles = cards.map(c => c.querySelector("h2,.card-title")?.textContent?.trim() || "(no title)");
    const memberRowCount = document.querySelectorAll(".rows .row").length;
    const settleUpCards = cards.filter(c => (c.querySelector("h2,.card-title")?.textContent || "").toLowerCase().includes("settle"));
    const hasSettleUp = settleUpCards.length > 0;
    const settleUpRowCount = settleUpCards.reduce((n, c) => n + c.querySelectorAll(".row").length, 0);
    const hasMermaid = !!document.querySelector(".mermaid,[class*=mermaid]");
    const hasForm = !!(document.querySelector("input[type=number]") && document.querySelector("select"));
    const btns = [...document.querySelectorAll("button")].map(b => b.textContent.trim());
    const hasSelectAll = btns.some(t => t === "Select all");
    const hasClear = btns.some(t => t === "Clear");
    const hasSaveBtn = btns.some(t => t.includes("Save split"));
    const hasCsvBtn = btns.some(t => t.includes("CSV") || t.includes("csv"));
    const errText = document.querySelector("#split-err")?.textContent?.trim() || "";
    const pageHeight = document.body.scrollHeight;
    const viewportH = window.innerHeight;
    const overflowCount = cards.filter(c => c.scrollWidth > c.clientWidth + 4).length;
    const hasStatGrid = !!document.querySelector(".stat-grid");
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const amtEl = document.querySelector(".budget-amount");
    const amtColor = amtEl ? getComputedStyle(amtEl).color : "N/A";
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const hasWeightedToggle = [...document.querySelectorAll("label")].some(l => l.textContent.toLowerCase().includes("weight"));
    const settleEl = settleUpCards[0];
    const settleAboveFold = settleEl ? settleEl.getBoundingClientRect().top < window.innerHeight : false;
    const owesRows = [...document.querySelectorAll(".rows .row .row-desc")].map(el => el.textContent?.trim());
    return { cardTitles, memberRowCount, hasSettleUp, settleUpRowCount, hasMermaid, hasForm,
             hasSelectAll, hasClear, hasSaveBtn, hasCsvBtn, errText, pageHeight, viewportH,
             overflowCount, hasStatGrid, cardTitleColor, amtColor, mutedColor, dataTheme,
             hasWeightedToggle, settleAboveFold, owesRows };
  });

  fs.writeFileSync(path.join(SHOTS, "glamor_12_split_dom.json"), JSON.stringify(domAudit, null, 2));

  // LIGHT
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));
  await light.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await light.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await light.waitForTimeout(400);
  await light.evaluate(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" })));
  await light.reload();
  await light.waitForFunction(() => document.documentElement.getAttribute("data-theme") === "light");
  await light.waitForTimeout(600);
  await light.locator('nav a[title="Split"]').first().click();
  await light.waitForSelector(".card", { timeout: 30000 });
  await light.waitForTimeout(1000);

  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("1280_light_empty") });

  await fillExpense(light, "1280_light_filled");
  await light.screenshot({ path: shot("1280_light_filled_full"), fullPage: true });

  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("1440_light_filled") });

  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(400);
  await light.screenshot({ path: shot("768_light_filled") });

  const lightContrast = await light.evaluate(() => {
    const cardTitleEl = document.querySelector("h2.card-title,.card-title");
    const cardTitleColor = cardTitleEl ? getComputedStyle(cardTitleEl).color : "N/A";
    const cardBg = cardTitleEl ? getComputedStyle(cardTitleEl.closest(".card") || cardTitleEl).backgroundColor : "N/A";
    const amtEl = document.querySelector(".budget-amount");
    const amtColor = amtEl ? getComputedStyle(amtEl).color : "N/A";
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? getComputedStyle(mutedEl).color : "N/A";
    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? getComputedStyle(rowDescEl).color : "N/A";
    const dataTheme = document.documentElement.getAttribute("data-theme") || "none";
    const statGridEl = document.querySelector(".stat-grid");
    const statGridBg = statGridEl ? getComputedStyle(statGridEl).backgroundColor : "N/A";
    return { cardTitleColor, cardBg, amtColor, mutedColor, rowDescColor, dataTheme, statGridBg };
  });

  fs.writeFileSync(path.join(SHOTS, "glamor_12_split_light_contrast.json"), JSON.stringify(lightContrast, null, 2));

  await dark.reload({ waitUntil: "domcontentloaded" });
  await dark.waitForTimeout(800);
  const themeAfterReload = await dark.evaluate(() => document.documentElement.getAttribute("data-theme"));

  console.log("=== DOM Audit ===");
  console.log(JSON.stringify(domAudit, null, 2));
  console.log("=== Light Contrast ===");
  console.log(JSON.stringify(lightContrast, null, 2));
  console.log("theme after reload:", themeAfterReload);
  console.log("errors:", errors.length === 0 ? "none" : errors);
  console.log("shots:", SHOTS);

} finally {
  await browser.close();
}
