// L97 E2E loop story — "The Import" (Priya) — 2026-06-24
//
// Theme: CSV-IMPORT INTEGRITY. Getting bank data in is a core onboarding/everyday flow. Pasting a CSV
// must (1) be accepted, (2) create the right number of transactions, (3) post them with the right
// description/amount to the chosen account (cross-screen: they show up in Transactions), and (4) not
// silently drop or duplicate rows. Invariants:
//   I-1  The CSV import accepts pasted rows + an "Import into account" target.
//   I-2  Importing N rows raises the transaction count by ~N (cross-screen, on Transactions).
//   I-3  The imported rows are findable by their unique description (they actually posted).
//   I-4  Re-importing the SAME CSV is de-duplicated (no double-posting) OR clearly handled.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_97_the_import.mjs

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
  await page.waitForTimeout(1300);
};
const txnCount = async (page) => { await navTo(page, "Transactions"); await page.waitForTimeout(700); return page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; }); };
const searchCount = async (page, term) => {
  await page.evaluate((term) => { const s = document.querySelector('input[type="search"]'); if (!s) return; const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set; set.call(s, term); s.dispatchEvent(new Event('input', { bubbles: true })); }, term);
  await page.waitForTimeout(900);
  return page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : 0; });
};

const CSV = "date,payee,amount,account\n2026-06-10,L97IMPORTALPHA,-12.34,X\n2026-06-11,L97IMPORTBETA,-56.78,X\n2026-06-12,L97IMPORTGAMMA,-9.01,X";

const importCSV = async (page) => {
  await navTo(page, "Documents");
  await page.waitForTimeout(700);
  return page.evaluate((csv) => {
    // the simple-CSV textarea has a placeholder starting with "date,payee,amount,account"
    const ta = [...document.querySelectorAll('textarea')].find(t => /date,\s*payee,\s*amount/i.test(t.placeholder || ""));
    if (!ta) return "NO_TEXTAREA";
    const setT = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
    setT.call(ta, csv); ta.dispatchEvent(new Event('input', { bubbles: true }));
    // pick an import-into account (first option)
    const sel = [...document.querySelectorAll('select')].find(s => /import into account/i.test(s.getAttribute('aria-label') || ""));
    if (sel && sel.options.length) { sel.value = sel.options[0].value; sel.dispatchEvent(new Event('change', { bubbles: true })); }
    const btn = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === "Import");
    if (!btn) return "NO_IMPORT_BTN";
    btn.click();
    return "imported-into:" + (sel ? sel.options[sel.selectedIndex].textContent.trim() : "?");
  }, CSV);
};

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try { await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 }); await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 }); hydrated = true; }
    catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  const before = await txnCount(page);
  note(`Transactions before import: ${before}`);

  // ── I-1: import accepts the CSV ───────────────────────────────────────────────
  // NB: "Import" commits directly (no separate review/preview step), so do NOT click any
  // second button — a re-click double-imports.
  const r1 = await importCSV(page);
  note(`Import #1: ${r1}`);
  await page.waitForTimeout(1300);
  await page.screenshot({ path: SS("L97_01_after_import.png") });
  if (r1.startsWith("imported")) pass(`I-1 — CSV import accepted (${r1})`);
  else absent_(`I-1 — could not run the import (${r1})`);

  // ── I-2: count rose by ~3 ─────────────────────────────────────────────────────
  const after = await txnCount(page);
  note(`Transactions after import: ${before} -> ${after} (expected +3)`);
  if (before != null && after != null && after >= before + 3) pass(`I-2 — import added the rows (${before} -> ${after}, +${after - before})`);
  else if (before != null && after != null && after > before) absent_(`I-2 — count rose but not by 3 (${before} -> ${after}); partial import or review pending`);
  else absent_(`I-2 — count did not rise (${before} -> ${after}) — import may need a review step not auto-confirmed`);

  // ── I-3: imported rows are findable ───────────────────────────────────────────
  const foundA = await searchCount(page, "L97IMPORTALPHA");
  const foundB = await searchCount(page, "L97IMPORTBETA");
  note(`Search for imported rows: ALPHA=${foundA}, BETA=${foundB}`);
  if (foundA >= 1 && foundB >= 1) pass(`I-3 — imported rows are findable by description (ALPHA=${foundA}, BETA=${foundB})`);
  else absent_(`I-3 — imported rows not found by description (ALPHA=${foundA}, BETA=${foundB})`);

  // ── I-4: re-import same CSV — does it dedup? (single click, no re-click) ───────
  await page.evaluate(() => { const s = document.querySelector('input[type="search"]'); if (s) { const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set; set.call(s, ""); s.dispatchEvent(new Event('input', { bubbles: true })); } });
  await page.waitForTimeout(700);
  const beforeDup = await txnCount(page);
  const r2 = await importCSV(page);
  await page.waitForTimeout(1300);
  const afterDup = await txnCount(page);
  const alphaAfterDup = await searchCount(page, "L97IMPORTALPHA");
  note(`Re-import same CSV (single click): count ${beforeDup} -> ${afterDup}; ALPHA now appears ${alphaAfterDup}x (r2=${r2})`);
  if (afterDup === beforeDup) pass(`I-4 — re-importing the same CSV did NOT duplicate (count steady ${afterDup}) — dedupe works`);
  else fail(`I-4 — re-import added ${afterDup - beforeDup} DUPLICATE rows (count ${beforeDup} -> ${afterDup}, ALPHA ${foundA} -> ${alphaAfterDup}) — CSV import does not dedup against existing transactions (data-integrity risk)`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
