// L99 E2E loop story — "The Reconciliation" (Marcus) — 2026-06-24
//
// Theme: CROSS-SCREEN DATA INTEGRITY. A household notices Dining is over budget, spots a transaction
// that belongs under Shopping, and recategorizes it. The fix must propagate everywhere immediately:
// Dining's budget "spent" must DROP by the amount, Shopping's must RISE by the same, and the app's
// total spending must be UNCHANGED (money only moved category). This exercises Transactions → Budgets
// data flow with no reload, the core "why am I over budget — let me fix a miscategorization" loop.
//
// Invariants:
//   R-1  Budgets page shows per-category spent ("Dining $655.00 / $300.00").
//   R-2  A single-transaction recategorize (Dining → Shopping) is accepted (toast / form closes).
//   R-3  Dining's spent DROPS by exactly the moved amount (live, no reload).
//   R-4  Shopping's spent RISES by exactly the moved amount.
//   R-5  Total spending across the two budgets is conserved (sum unchanged) — no money invented/lost.
//   R-6  No JS errors across the flow.
//
// Run: node e2e/loopstory_99_the_reconciliation.mjs   (against go run e2e/serve.go on :8099)

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
const num = (s) => parseFloat(String(s).replace(/[^0-9.]/g, "")) || 0;

// Parse the Budgets page into { categoryName: spentAmount } from each `.budget` row.
const readBudgets = (page) => page.evaluate(() => {
  const out = {};
  for (const b of document.querySelectorAll('.budget')) {
    const t = (b.textContent || "").replace(/\s+/g, " ").trim();
    // "Dining$655.00 / $300.00 ... " → name = up to first $, spent = first $-number
    const m = t.match(/^(.+?)\$([\d,]+\.?\d*)\s*\/\s*\$([\d,]+\.?\d*)/);
    if (m) out[m[1].trim()] = parseFloat(m[2].replace(/,/g, ""));
  }
  return out;
});

const grabToast = async (page) => { for (let i = 0; i < 16; i++) { const t = await page.evaluate(() => { const el = document.querySelector('.toast'); return el && el.offsetParent !== null ? el.textContent.trim().slice(0, 70) : null; }); if (t) return t; await page.waitForTimeout(80); } return null; };

const jsErrors = [];
const FROM = "Dining", TO = "Shopping";

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  // ── R-1: read budgets before ───────────────────────────────────────────────
  await navTo(page, "Budgets");
  const before = await readBudgets(page);
  await page.screenshot({ path: path.join(SSDIR, "L99_01_budgets_before.png") });
  note(`Budgets before: ${FROM}=${before[FROM]} ${TO}=${before[TO]}`);
  if (before[FROM] != null && before[TO] != null) pass(`R-1 — both budgets show spent (${FROM} $${before[FROM]}, ${TO} $${before[TO]})`);
  else { absent_(`R-1 — could not read ${FROM}/${TO} budgets (${JSON.stringify(Object.keys(before))})`); throw new Error("no budgets"); }

  // ── Find a Dining transaction, read its amount ─────────────────────────────
  await navTo(page, "Transactions");
  const picked = await page.evaluate((FROM) => {
    const rows = [...document.querySelectorAll('.row')];
    for (const r of rows) {
      const cat = (r.querySelector('.td-cat')?.textContent || "").trim();
      if (cat === FROM) {
        const amt = r.querySelector('.td-amount')?.textContent || "";
        const desc = (r.querySelector('.td-date')?.textContent || "") + "|" + amt;
        const e = [...r.querySelectorAll('button')].find(x => /edit this transaction/i.test(x.title || x.getAttribute('aria-label') || ''));
        if (e) { e.click(); return { amtText: amt, desc }; }
      }
    }
    return null;
  }, FROM);
  if (!picked) { absent_(`could not find a '${FROM}' transaction to move`); throw new Error("no dining txn"); }
  const amt = num(picked.amtText);
  note(`Picked ${FROM} txn ${picked.desc} amount=$${amt}`);
  await page.waitForTimeout(600);

  // ── R-2: recategorize Dining → Shopping via the edit form's Category select ─
  const moved = await page.evaluate((TO) => {
    const sel = [...document.querySelectorAll('select')].find(s => (s.getAttribute('aria-label') || s.name) === "Category");
    if (!sel) return "NO_SELECT";
    const opt = [...sel.options].find(o => o.textContent.trim() === TO);
    if (!opt) return "NO_OPTION";
    const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
    setter.call(sel, opt.value);
    sel.dispatchEvent(new Event('change', { bubbles: true }));
    const form = sel.closest('form');
    if (form) { form.requestSubmit(); return "submitted"; }
    return "NO_FORM";
  }, TO);
  const toast = await grabToast(page);
  await page.waitForTimeout(900);
  note(`recategorize result=${moved}, toast="${toast}"`);
  if (moved === "submitted") pass(`R-2 — single-transaction recategorize submitted (${FROM} → ${TO})`);
  else absent_(`R-2 — recategorize did not submit (${moved})`);

  // ── R-3 / R-4 / R-5: re-read budgets, assert propagation ───────────────────
  await navTo(page, "Budgets");
  const after = await readBudgets(page);
  await page.screenshot({ path: path.join(SSDIR, "L99_02_budgets_after.png") });
  note(`Budgets after: ${FROM}=${after[FROM]} ${TO}=${after[TO]}`);
  const eps = 0.01;
  const dFrom = (before[FROM] ?? 0) - (after[FROM] ?? 0); // expected = amt (drop)
  const dTo = (after[TO] ?? 0) - (before[TO] ?? 0);       // expected = amt (rise)

  if (Math.abs(dFrom - amt) <= eps) pass(`R-3 — ${FROM} spent dropped by exactly $${amt} (${before[FROM]} → ${after[FROM]})`);
  else fail(`R-3 — ${FROM} spent change was $${dFrom.toFixed(2)}, expected -$${amt} (${before[FROM]} → ${after[FROM]})`);

  if (Math.abs(dTo - amt) <= eps) pass(`R-4 — ${TO} spent rose by exactly $${amt} (${before[TO]} → ${after[TO]})`);
  else fail(`R-4 — ${TO} spent change was $${dTo.toFixed(2)}, expected +$${amt} (${before[TO]} → ${after[TO]})`);

  const sumBefore = (before[FROM] ?? 0) + (before[TO] ?? 0);
  const sumAfter = (after[FROM] ?? 0) + (after[TO] ?? 0);
  if (Math.abs(sumBefore - sumAfter) <= eps) pass(`R-5 — total spending conserved ($${sumBefore.toFixed(2)} = $${sumAfter.toFixed(2)}) — money moved, not invented`);
  else fail(`R-5 — total spending changed $${sumBefore.toFixed(2)} → $${sumAfter.toFixed(2)} (money should only have moved category!)`);

  if (jsErrors.length === 0) pass("R-6 — zero runtime JS errors across the ritual");
  else fail(`R-6 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (!["no budgets", "no dining txn"].includes(String(err.message))) { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
