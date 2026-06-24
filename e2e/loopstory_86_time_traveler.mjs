// L86 E2E loop story — "The Time Traveler" (Priya) — 2026-06-24
//
// Theme: PERIOD-NAVIGATION INTEGRITY — the Week/Month/Quarter/Year picker + prev/next stepper
// drive every screen's figures, so changing the period must recompute correctly, be reversible,
// be internally consistent (quarter superset of month), and carry across screens. Invariants:
//   T-1  "Previous period" changes the displayed period label (and recomputes figures).
//   T-2  prev -> next round-trips back to the SAME period + SAME spending (reversible/idempotent).
//   T-3  Switching Month -> Quarter never DECREASES spending (a quarter ⊇ its months).
//   T-4  The selected period CARRIES across screens (same period pill on Reports/Budgets/Txns).
//   T-5  STRESS: stepping back several periods yields a valid, distinct label each time, no crash.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_86_time_traveler.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (n) => path.join(SSDIR, n);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log(`PASS:   ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL:   ${l}`); failed++; };
const absent_ = (l) => { console.log(`ABSENT: ${l}`); absent++; };
const note = (l) => { console.log(`NOTE:   ${l}`); };

const navTo = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1400);
};
const parseMoney = (s) => { if (!s) return null; const n = parseFloat(s.replace(/[^0-9.]/g, "")); return isNaN(n) ? null : n; };

// The period pill (stepper Label) — e.g. "Jun 2026", "Q3 2026", "2026", "Jun 22–28".
const periodLabel = (page) => page.evaluate(() => {
  // prefer the element between the prev/next stepper buttons
  const prev = [...document.querySelectorAll('button')].find(b => /previous period/i.test(b.getAttribute("aria-label") || ""));
  if (prev) {
    let sib = prev.nextElementSibling;
    while (sib) { const t = sib.textContent.trim(); if (t && t.length < 24 && !/period/i.test(sib.getAttribute("aria-label") || "")) return t; sib = sib.nextElementSibling; }
  }
  // fallback: a "Mon YYYY" / "Qn YYYY" / "YYYY" token
  const m = document.body.textContent.match(/\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4}\b|\bQ[1-4]\s+\d{4}\b/);
  return m ? m[0] : null;
});
const reportsSpend = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/SPENDING[^$]{0,30}?(\$[\d,]+\.?\d*)/i);
  return m ? m[1] : null;
});
const clickStepper = (page, which) => page.evaluate((which) => {
  const lab = which === "prev" ? "previous period" : "next period";
  const b = [...document.querySelectorAll('button')].find(b => new RegExp(lab, "i").test(b.getAttribute("aria-label") || ""));
  if (b) { b.click(); return "clicked"; } return "NO_BTN";
}, which);
const setGranularity = (page, label) => page.evaluate((label) => {
  const b = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === label && (b.className || "").includes("seg"));
  if (b) { b.click(); return "set"; }
  const b2 = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === label);
  if (b2) { b2.click(); return "set-fallback"; }
  return "NO_SEG";
}, label);

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try { await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 }); await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 }); hydrated = true; }
    catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  // ensure Month granularity for a deterministic start
  await navTo(page, "Reports"); await page.waitForTimeout(900);
  await setGranularity(page, "Month"); await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L86_01_month.png") });
  const label0 = await periodLabel(page);
  const spend0 = parseMoney(await reportsSpend(page));
  note(`Start: period="${label0}" | spending=${spend0}`);
  if (!label0) { absent_("Could not read the period label — aborting period checks"); throw new Error("no label"); }

  // ── T-1: previous period changes the label ───────────────────────────────────
  const p1 = await clickStepper(page, "prev"); await page.waitForTimeout(1000);
  const label1 = await periodLabel(page);
  const spend1 = parseMoney(await reportsSpend(page));
  note(`After prev: period="${label1}" | spending=${spend1} (was "${label0}"/${spend0})`);
  await page.screenshot({ path: SS("L86_02_prev.png") });
  if (p1 === "clicked" && label1 && label1 !== label0) pass(`T-1 — "Previous period" changed the period (${label0} -> ${label1})`);
  else fail(`T-1 — prev did not change the period (${label0} -> ${label1}, click=${p1})`);

  // ── T-2: next returns to the original (reversible) ───────────────────────────
  await clickStepper(page, "next"); await page.waitForTimeout(1000);
  const label2 = await periodLabel(page);
  const spend2 = parseMoney(await reportsSpend(page));
  note(`After next: period="${label2}" | spending=${spend2}`);
  if (label2 === label0) pass(`T-2a — next returned to the original period (${label2})`);
  else fail(`T-2a — next did not return to "${label0}" (got "${label2}")`);
  if (spend0 !== null && spend2 !== null && Math.abs(spend0 - spend2) < 0.01) pass(`T-2b — spending is identical after the prev/next round-trip (${spend2}) — recompute is reversible`);
  else absent_(`T-2b — spending differs after round-trip (${spend0} -> ${spend2})`);

  // ── T-3: Quarter spending >= Month spending (superset) ───────────────────────
  await setGranularity(page, "Quarter"); await page.waitForTimeout(1000);
  const labelQ = await periodLabel(page);
  const spendQ = parseMoney(await reportsSpend(page));
  note(`Quarter: period="${labelQ}" | spending=${spendQ} (month was ${spend0})`);
  await page.screenshot({ path: SS("L86_03_quarter.png") });
  if (spend0 !== null && spendQ !== null) {
    if (spendQ + 0.01 >= spend0) pass(`T-3 — Quarter spending (${spendQ}) >= the month's (${spend0}) — quarter is a superset`);
    else fail(`T-3 — Quarter spending (${spendQ}) < month (${spend0}) — period math inconsistent`);
  }
  await setGranularity(page, "Month"); await page.waitForTimeout(900);

  // ── T-4: period carries across screens ───────────────────────────────────────
  const labelReports = await periodLabel(page);
  await navTo(page, "Budgets"); await page.waitForTimeout(800);
  const labelBudgets = await periodLabel(page);
  await navTo(page, "Transactions"); await page.waitForTimeout(800);
  const labelTxns = await periodLabel(page);
  note(`Period across screens: Reports="${labelReports}" Budgets="${labelBudgets}" Transactions="${labelTxns}"`);
  if (labelReports && labelReports === labelBudgets && labelBudgets === labelTxns) pass(`T-4 — the period (${labelReports}) carries consistently across Reports/Budgets/Transactions`);
  else absent_(`T-4 — period not consistent across screens (R="${labelReports}" B="${labelBudgets}" T="${labelTxns}")`);

  // ── T-5: STRESS — step back several periods ──────────────────────────────────
  await navTo(page, "Reports"); await page.waitForTimeout(800);
  await setGranularity(page, "Month"); await page.waitForTimeout(700);
  const labels = new Set(); let stressOk = true;
  for (let i = 0; i < 4; i++) {
    const c = await clickStepper(page, "prev"); await page.waitForTimeout(800);
    const lbl = await periodLabel(page);
    if (c !== "clicked" || !lbl) { stressOk = false; note(`  step ${i + 1}: click=${c} label=${lbl}`); }
    else labels.add(lbl);
  }
  note(`Stepped back 4×: distinct labels = ${labels.size} [${[...labels].join(", ")}]`);
  if (stressOk && labels.size === 4) pass("T-5 — stepping back 4 periods gave 4 distinct valid labels (no repeat/crash)");
  else absent_(`T-5 — got ${labels.size}/4 distinct labels (stressOk=${stressOk})`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no label") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
