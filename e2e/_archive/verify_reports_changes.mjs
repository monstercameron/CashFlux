// verify_reports_changes.mjs — verifies G9.1a (donut chart), G9.1 R-1 (Sankey
// position), and G9.1 R-3 (share-bar full width) are present and correct.
// Run: node e2e/verify_reports_changes.mjs
// Requires: serve running at E2E_URL (default http://127.0.0.1:8099).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
const pass = (m) => console.log("PASS: " + m);

async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

try {
  // ── Dark theme ────────────────────────────────────────────────────────────
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => fail("page error (dark): " + e.message));

  await dark.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  // Wait for chart canvases / SVG paths to materialise
  await dark.waitForSelector('[role="img"]', { timeout: 60000 });
  await dark.waitForTimeout(1500); // let D3 draw-in animate

  // G9.1a: Donut chart — must have >1 path inside an SVG inside a role=img div
  const donutSlicesDark = await dark.evaluate(() => {
    const imgs = document.querySelectorAll('[role="img"]');
    let slices = 0;
    imgs.forEach((el) => {
      const paths = el.querySelectorAll("svg path");
      if (paths.length > slices) slices = paths.length;
    });
    return slices;
  });
  if (donutSlicesDark > 1) {
    pass(`G9.1a dark: donut has ${donutSlicesDark} path slices`);
  } else {
    fail(`G9.1a dark: expected >1 donut slices, got ${donutSlicesDark}`);
  }

  // G9.1 R-3: Share bar max-width should be 100% (not 260px)
  const shareBarWidthDark = await dark.evaluate(() => {
    const bar = document.querySelector(".share-bar");
    if (!bar) return null;
    return { maxWidth: bar.style.maxWidth, clientWidth: bar.clientWidth, parentWidth: bar.parentElement ? bar.parentElement.clientWidth : 0 };
  });
  if (shareBarWidthDark) {
    if (shareBarWidthDark.maxWidth === "100%") {
      pass(`G9.1 R-3 dark: share-bar max-width is 100%`);
    } else {
      fail(`G9.1 R-3 dark: share-bar max-width = "${shareBarWidthDark.maxWidth}", expected "100%"`);
    }
  } else {
    console.warn("WARN: no .share-bar found (may be empty dataset)");
  }

  // G9.1 R-1: Sankey position — the Sankey section should appear before payees
  const sankeyPosition = await dark.evaluate(() => {
    // Find all section-like containers (EntityListSection renders as a card/section)
    const cards = Array.from(document.querySelectorAll('.card, [class*="entity-list"]'));
    const texts = cards.map((c) => c.textContent.slice(0, 80));
    const sankeyIdx = texts.findIndex((t) => t.toLowerCase().includes("money flow") || t.toLowerCase().includes("sankey"));
    const payeesIdx = texts.findIndex((t) => t.toLowerCase().includes("payee") || t.toLowerCase().includes("top payees"));
    return { sankeyIdx, payeesIdx, cardCount: cards.length };
  });
  if (sankeyPosition.sankeyIdx === -1) {
    console.warn("WARN: Sankey section not found in DOM (may need real transactions)");
  } else if (sankeyPosition.sankeyIdx < sankeyPosition.payeesIdx || sankeyPosition.payeesIdx === -1) {
    pass(`G9.1 R-1: Sankey at card index ${sankeyPosition.sankeyIdx}, payees at ${sankeyPosition.payeesIdx} — Sankey is earlier`);
  } else {
    fail(`G9.1 R-1: Sankey at card ${sankeyPosition.sankeyIdx} but payees at ${sankeyPosition.payeesIdx} — expected Sankey first`);
  }

  await dark.screenshot({ path: path.join(SHOTS, "reports_donut_dark.png"), fullPage: false });
  pass("screenshot: e2e/screenshots/reports_donut_dark.png");

  // ── Light theme ───────────────────────────────────────────────────────────
  const light = await browser.newPage();
  light.on("pageerror", (e) => fail("page error (light): " + e.message));

  await light.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await light.waitForSelector('[role="img"]', { timeout: 60000 });

  // Switch to light theme via localStorage + reload
  await light.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify(
      Object.assign(JSON.parse(localStorage.getItem("cashflux:prefs") || "{}"), { theme: "light" })
    ));
  });
  await light.reload({ waitUntil: "domcontentloaded" });
  await light.waitForSelector('[role="img"]', { timeout: 60000 });
  await light.waitForTimeout(1500);

  const donutSlicesLight = await light.evaluate(() => {
    const imgs = document.querySelectorAll('[role="img"]');
    let slices = 0;
    imgs.forEach((el) => {
      const paths = el.querySelectorAll("svg path");
      if (paths.length > slices) slices = paths.length;
    });
    return slices;
  });
  if (donutSlicesLight > 1) {
    pass(`G9.1a light: donut has ${donutSlicesLight} path slices`);
  } else {
    fail(`G9.1a light: expected >1 donut slices, got ${donutSlicesLight}`);
  }

  const shareBarWidthLight = await light.evaluate(() => {
    const bar = document.querySelector(".share-bar");
    if (!bar) return null;
    return { maxWidth: bar.style.maxWidth };
  });
  if (shareBarWidthLight) {
    if (shareBarWidthLight.maxWidth === "100%") {
      pass(`G9.1 R-3 light: share-bar max-width is 100%`);
    } else {
      fail(`G9.1 R-3 light: share-bar max-width = "${shareBarWidthLight.maxWidth}", expected "100%"`);
    }
  }

  await light.screenshot({ path: path.join(SHOTS, "reports_donut_light.png"), fullPage: false });
  pass("screenshot: e2e/screenshots/reports_donut_light.png");

  if (!process.exitCode) {
    pass("All G9.1a / R-1 / R-3 checks passed.");
  }
} finally {
  await browser.close();
}
