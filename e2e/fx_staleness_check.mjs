// L4 gate — "FX rates show a staleness signal so net worth doesn't silently
// drift." Stamps a seeded rate (EUR) as last-updated 40 days ago, opens Settings,
// and asserts that rate's row shows a "Stale" badge. A one-shot addInitScript
// applies the stamp at document-start (after the reload's pagehide autosave).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getDS = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitDS(page, pred, timeoutMs = 10000) {
  let d = {};
  for (let w = 0; w < timeoutMs; w += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".topbar", { timeout: 60000 });
  await waitDS(page, (d) => d.settings && d.settings.fxRates && "EUR" in d.settings.fxRates);

  await page.evaluate(() => localStorage.setItem("e2e-fxstale", "1"));
  await page.addInitScript(() => {
    if (!localStorage.getItem("e2e-fxstale")) return;
    localStorage.removeItem("e2e-fxstale"); // one-shot
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.settings = ds.settings || {};
      ds.settings.fxUpdatedAt = ds.settings.fxUpdatedAt || {};
      ds.settings.fxUpdatedAt.EUR = "2026-05-01T00:00:00Z"; // ~7 weeks before "today"
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".topbar", { timeout: 60000 });
  await waitDS(page, (d) => d.settings && d.settings.fxUpdatedAt && d.settings.fxUpdatedAt.EUR);

  // Open the global Settings panel via the household (gear) button in the rail.
  await page.locator("button.hh").first().click();
  await page.waitForTimeout(500);

  // The EUR rate row should carry a "Stale" badge.
  const stale = page.locator('[data-testid="fx-stale"]');
  const seen = await stale.first().isVisible().catch(() => false);
  if (!seen) fail('no "Stale" FX badge shown for a rate last updated 40 days ago');

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: a 40-day-old FX rate is flagged Stale in Settings.");
} finally {
  await browser.close();
}
