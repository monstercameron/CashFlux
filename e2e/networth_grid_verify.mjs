// Reports → Net worth tab layout contract (updated for the bento /reports
// surface): the canonical NW panel (#networth) renders full-width ABOVE the two
// trend tiles; at 1440px the cash-flow + savings-rate tiles sit side-by-side
// (span-2 bento pair, each with a chart); at a phone width they stack.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed = 0; const fail = m => { console.error("FAIL: " + m); failed++; process.exitCode = 1; }; const pass = m => console.log("PASS: " + m);
const errs = [];
try {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const p = await ctx.newPage();
  p.on("pageerror", e => errs.push(String(e))); p.on("console", m => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector("#app", { timeout: 60000 });
  await p.waitForTimeout(4500);
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1400);
  await p.evaluate(() => { const b = [...document.querySelectorAll('[role="radio"],button')].find(x => /^Net worth$/i.test((x.textContent || "").trim())); if (b) b.click(); });
  await p.waitForTimeout(900);

  const layout = await p.evaluate(() => {
    const nw = document.querySelector("#networth");
    const cash = document.querySelector('[data-widget="rpt-cashtrend"]');
    const save = document.querySelector('[data-widget="rpt-savingstrend"]');
    if (!nw || !cash || !save) return null;
    const r = el => el.getBoundingClientRect();
    return {
      charts: cash.querySelectorAll("svg").length + save.querySelectorAll("svg").length,
      sideBySide: Math.abs(r(cash).top - r(save).top) < 40 && r(save).left > r(cash).right - 5,
      nwAbove: r(nw).bottom <= r(cash).top + 5,
      nwWide: r(nw).width > r(cash).width * 1.5,
    };
  });
  if (!layout) { fail("missing #networth or the trend tiles on the Net worth tab"); }
  else {
    console.log("  wide(1440): " + JSON.stringify(layout));
    if (layout.sideBySide) pass("trend tiles pair side-by-side at 1440px"); else fail("trend tiles not side-by-side");
    if (layout.charts >= 2) pass(layout.charts + " trend charts render"); else fail("only " + layout.charts + " charts");
    if (layout.nwAbove) pass("NW panel sits above the trend pair"); else fail("NW panel not above the trend pair");
    if (layout.nwWide) pass("NW panel is full-width vs the half-width tiles"); else fail("NW panel not full-width");
  }

  await p.setViewportSize({ width: 700, height: 1000 }); await p.waitForTimeout(600);
  const stacked = await p.evaluate(() => {
    const cash = document.querySelector('[data-widget="rpt-cashtrend"]');
    const save = document.querySelector('[data-widget="rpt-savingstrend"]');
    if (!cash || !save) return null;
    const r = el => el.getBoundingClientRect();
    return r(save).top >= r(cash).bottom - 5;
  });
  if (stacked === true) pass("trend tiles stack at a phone width"); else fail("trend tiles did not stack at 700px (" + stacked + ")");

  await p.setViewportSize({ width: 1440, height: 1000 }); await p.waitForTimeout(400);
  await p.screenshot({ path: "e2e/screenshots/networth_grid.png", fullPage: true });
  console.log("errors: " + errs.length); if (errs.length) { errs.slice(0, 4).forEach(e => console.log("  ERR:" + e)); fail("console errors"); }
} catch (e) { fail("exception: " + e.message); } finally { await browser.close(); }
console.log(failed ? "RESULT: FAILED" : "RESULT: PASSED");
process.exit(failed ? 1 : 0);
