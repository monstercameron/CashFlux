// GLAMOR G20 — Artifacts page visual + structural review for "Keep the Receipt" (Lena).
// Reviews the upload card, artifact list, card titles, where-used badges, light-mode contrast,
// storage meter, empty/loading states, and 768px behaviour.
// Screenshots at 1280 / 1440 / 768 × dark + light.
// Writes into e2e/screenshots/glamor_20_artifacts_*.png and glamor_20_artifacts_dom*.json.
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

const shot = (name) => path.join(SHOTS, `glamor_20_artifacts_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];

// ---------------------------------------------------------------
// Navigation helpers
// ---------------------------------------------------------------
async function navToArtifacts(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(600);
  // Reset viewAsMember to Everyone
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        delete p.viewAsMember;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      } catch (_) {}
    }
  });
  // Navigate via nav link
  const link = page.locator('nav a[title="Artifacts"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /artifacts/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
}

// ---------------------------------------------------------------
// DOM audit
// ---------------------------------------------------------------
async function auditDOM(page) {
  return page.evaluate(() => {
    // Cards
    const cards = [...document.querySelectorAll(".card, section.card")];
    const cardTitles = cards.map(c => {
      const t = c.querySelector("h2,h3,.card-title");
      return t ? t.textContent.trim() : "(no title)";
    });
    const cardHeadingLevels = cards.map(c => {
      const t = c.querySelector("h1,h2,h3,h4");
      return t ? t.tagName : "none";
    });

    // Buttons
    const btns = [...document.querySelectorAll("button")];
    const btnTexts = btns.map(b => b.textContent.trim());
    const btnPrimaryCount = [...document.querySelectorAll("button.btn-primary")].length;

    // Inputs / selects / fields
    const fields = [...document.querySelectorAll('input.field,select.field,textarea.field')];
    const fieldCount = fields.length;

    // Rows / artifact list
    const rows = [...document.querySelectorAll(".row")];
    const rowCount = rows.length;
    const artifactNames = [...document.querySelectorAll('[data-testid="artifact-name"]')]
      .map(el => el.textContent.trim());
    const artifactRefs = [...document.querySelectorAll('[data-testid="artifact-refs"]')]
      .map(el => el.textContent.trim());
    const artifactPageRefs = [...document.querySelectorAll('[data-testid="artifact-page-refs"]')]
      .map(el => el.textContent.trim());

    // Empty state
    const emptyEls = [...document.querySelectorAll(".empty")];
    const emptyTexts = emptyEls.map(e => e.textContent.trim());

    // Storage meter
    const mutedEls = [...document.querySelectorAll(".muted")];
    const mutedTexts = mutedEls.map(e => e.textContent.trim());

    // Action buttons (rename + delete) vs href
    const actionLinks = [...document.querySelectorAll('a[href]')].filter(a =>
      /rename|delete|artifact/i.test(a.textContent + (a.getAttribute("aria-label") || ""))
    );
    const hrefActionCount = actionLinks.length;

    // btn-del count
    const delBtnCount = [...document.querySelectorAll(".btn-del")].length;
    const renameBtnCount = [...document.querySelectorAll("button[aria-label]")]
      .filter(b => /rename/i.test(b.getAttribute("aria-label") || "")).length;

    // Overflow check
    const overflowCount = [...document.querySelectorAll("*")].filter(el => {
      const s = window.getComputedStyle(el);
      return s.overflowX === "scroll" || (el.scrollWidth > el.clientWidth + 2 && s.overflow !== "hidden");
    }).length;

    // Page size
    const pageHeight = document.body.scrollHeight;
    const viewportH  = window.innerHeight;

    // Notice / quota nudge
    const noticeEls = [...document.querySelectorAll(".notice, .notice-warn")];
    const noticeTexts = noticeEls.map(e => e.textContent.trim());

    return {
      cardCount: cards.length,
      cardTitles,
      cardHeadingLevels,
      btnTexts,
      btnPrimaryCount,
      fieldCount,
      rowCount,
      artifactNames,
      artifactRefs,
      artifactPageRefs,
      emptyTexts,
      mutedTexts,
      hrefActionCount,
      delBtnCount,
      renameBtnCount,
      overflowCount,
      pageHeight,
      viewportH,
      noticeTexts,
    };
  });
}

// ---------------------------------------------------------------
// Contrast audit
// ---------------------------------------------------------------
async function auditContrast(page) {
  return page.evaluate(() => {
    function lum(rgb) {
      const [r, g, b] = rgb.match(/\d+/g).map(Number).map(v => {
        v /= 255;
        return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
      });
      return 0.2126 * r + 0.7152 * g + 0.0722 * b;
    }
    function cr(c1, c2) {
      const l1 = lum(c1), l2 = lum(c2);
      return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
    }
    const cardEl = document.querySelector(".card, section.card");
    const cardBg = cardEl ? window.getComputedStyle(cardEl).backgroundColor : "n/a";
    const titleEl = document.querySelector("h2.card-title, .card-title");
    const cardTitleColor = titleEl ? window.getComputedStyle(titleEl).color : "n/a";
    const mutedEl = document.querySelector(".muted");
    const mutedColor = mutedEl ? window.getComputedStyle(mutedEl).color : "n/a";
    const rowDescEl = document.querySelector(".row-desc");
    const rowDescColor = rowDescEl ? window.getComputedStyle(rowDescEl).color : "n/a";
    const rowMetaEl = document.querySelector(".row-meta");
    const rowMetaColor = rowMetaEl ? window.getComputedStyle(rowMetaEl).color : "n/a";
    const mainEl = document.querySelector("main, #app > div, .screen");
    const mainBg = mainEl ? window.getComputedStyle(mainEl).backgroundColor : "rgba(0,0,0,0)";
    const bodyBg = window.getComputedStyle(document.body).backgroundColor;
    const btnPrimaryEl = document.querySelector("button.btn-primary");
    const btnPrimaryBg = btnPrimaryEl ? window.getComputedStyle(btnPrimaryEl).backgroundColor : "n/a";
    const btnPrimaryColor = btnPrimaryEl ? window.getComputedStyle(btnPrimaryEl).color : "n/a";
    const emptyEl = document.querySelector(".empty");
    const emptyColor = emptyEl ? window.getComputedStyle(emptyEl).color : "n/a";

    return {
      cardBg, cardTitleColor, mutedColor, rowDescColor, rowMetaColor,
      mainBg, bodyBg, btnPrimaryBg, btnPrimaryColor, emptyColor,
      titleOnCard: (cardBg !== "n/a" && cardTitleColor !== "n/a" &&
        !cardBg.includes("0, 0, 0, 0")) ? cr(cardTitleColor, cardBg).toFixed(2) : "n/a",
    };
  });
}

// ---------------------------------------------------------------
// Main
// ---------------------------------------------------------------
let domDark = null, domLight = null, contrastDark = null, contrastLight = null;

// ---- DARK 1280 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`dark-1280: ${e.message}`));
  await page.setViewportSize({ width: 1280, height: 900 });
  await navToArtifacts(page);
  await page.screenshot({ path: shot("dark_1280"), fullPage: false });
  await page.screenshot({ path: shot("dark_1280_full"), fullPage: true });
  domDark = await auditDOM(page);
  contrastDark = await auditContrast(page);
  await page.close();
}

// ---- DARK 1440 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`dark-1440: ${e.message}`));
  await page.setViewportSize({ width: 1440, height: 900 });
  await navToArtifacts(page);
  await page.screenshot({ path: shot("dark_1440"), fullPage: false });
  await page.screenshot({ path: shot("dark_1440_full"), fullPage: true });
  await page.close();
}

// ---- DARK 768 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`dark-768: ${e.message}`));
  await page.setViewportSize({ width: 768, height: 1024 });
  await navToArtifacts(page);
  await page.screenshot({ path: shot("dark_768"), fullPage: false });
  await page.screenshot({ path: shot("dark_768_full"), fullPage: true });
  await page.close();
}

// ---- LIGHT 1280 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`light-1280: ${e.message}`));
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  // Apply light theme
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({theme:'light'})));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light');
  await page.waitForTimeout(600);
  // Reset viewAsMember
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        delete p.viewAsMember;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      } catch (_) {}
    }
  });
  // Navigate to Artifacts
  const link = page.locator('nav a[title="Artifacts"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /artifacts/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
  await page.screenshot({ path: shot("light_1280"), fullPage: false });
  await page.screenshot({ path: shot("light_1280_full"), fullPage: true });
  domLight = await auditDOM(page);
  contrastLight = await auditContrast(page);
  await page.close();
}

// ---- LIGHT 1440 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`light-1440: ${e.message}`));
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({theme:'light'})));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light');
  await page.waitForTimeout(600);
  const link = page.locator('nav a[title="Artifacts"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /artifacts/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
  await page.screenshot({ path: shot("light_1440"), fullPage: false });
  await page.screenshot({ path: shot("light_1440_full"), fullPage: true });
  await page.close();
}

// ---- LIGHT 768 ----
{
  const page = await browser.newPage();
  page.on("pageerror", e => errors.push(`light-768: ${e.message}`));
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.evaluate(() => localStorage.setItem('cashflux:prefs', JSON.stringify({theme:'light'})));
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute('data-theme') === 'light');
  await page.waitForTimeout(600);
  const link = page.locator('nav a[title="Artifacts"]').first();
  if (await link.count() > 0) {
    await link.click();
  } else {
    const fallback = page.locator('nav a').filter({ hasText: /artifacts/i }).first();
    if (await fallback.count() > 0) await fallback.click();
    else await page.goto(BASE + "/artifacts", { waitUntil: "domcontentloaded" });
  }
  await page.waitForSelector(".card", { timeout: 30000 });
  await page.waitForTimeout(1200);
  await page.screenshot({ path: shot("light_768"), fullPage: false });
  await page.screenshot({ path: shot("light_768_full"), fullPage: true });
  await page.close();
}

await browser.close();

// ---------------------------------------------------------------
// Write JSON audit files
// ---------------------------------------------------------------
fs.writeFileSync(
  path.join(__dirname, "glamor_20_artifacts_dom.json"),
  JSON.stringify({ dark: domDark, contrastDark }, null, 2)
);
fs.writeFileSync(
  path.join(__dirname, "glamor_20_artifacts_dom_light.json"),
  JSON.stringify({ light: domLight, contrastLight }, null, 2)
);

// ---------------------------------------------------------------
// Console report
// ---------------------------------------------------------------
console.log("=== G20 Artifacts DOM audit (dark) ===");
console.log(JSON.stringify(domDark, null, 2));
console.log("=== G20 Artifacts contrast (dark) ===");
console.log(JSON.stringify(contrastDark, null, 2));
console.log("=== G20 Artifacts DOM audit (light) ===");
console.log(JSON.stringify(domLight, null, 2));
console.log("=== G20 Artifacts contrast (light) ===");
console.log(JSON.stringify(contrastLight, null, 2));

if (errors.length) {
  console.error("PAGE ERRORS:", errors);
  process.exit(1);
}
console.log("G20 done — EXIT 0");
process.exit(0);
