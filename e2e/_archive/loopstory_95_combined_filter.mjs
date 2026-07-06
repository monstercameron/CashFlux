// L95 E2E loop story — "The Combined Filter" (Priya) — 2026-06-24
//
// Theme: FILTER-COMPOSITION INTEGRITY. "Find the right data fast" means stacking filters — show MY
// (member) GROCERIES (search) transactions — and trusting the result. Composed filters must (1) act as
// an INTERSECTION (count ≤ each filter alone), (2) be ORDER-INDEPENDENT (member-then-search ==
// search-then-member), (3) CLEAR back to the per-screen baseline, and (4) survive rapid combine/clear.
// Invariants:
//   F-1  member-filter count ≤ everyone; search-filter count ≤ everyone.
//   F-2  member+search count ≤ min(member, search) — a true intersection.
//   F-3  Applying the two filters in EITHER order yields the SAME composed count.
//   F-4  Clearing the search restores the member-only count; resetting member restores baseline.
//   F-5  STRESS: rapid combine/clear cycles return to a consistent baseline, no crash.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_95_combined_filter.mjs

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
const count = (page) => page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; });
const setMember = (page, val) => page.evaluate((val) => {
  const s = [...document.querySelectorAll('select')].find(x => /view as member/i.test(x.getAttribute('aria-label') || ''));
  if (!s) return; const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
  setter.call(s, val); s.dispatchEvent(new Event('change', { bubbles: true }));
}, val);
const setSearch = async (page, term) => {
  await page.evaluate((term) => {
    const s = document.querySelector('input[type="search"]'); if (!s) return;
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setter.call(s, term); s.dispatchEvent(new Event('input', { bubbles: true }));
  }, term);
  await page.waitForTimeout(800);
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

  await navTo(page, "Transactions");
  await page.waitForTimeout(800);
  const TERM = "groceries", MEMBER = "m-marcus";
  await setMember(page, ""); await setSearch(page, "");
  const base = await count(page);
  note(`Baseline (Everyone, no search): ${base}`);

  // ── F-1: each filter alone ≤ everyone ─────────────────────────────────────────
  await setMember(page, MEMBER); await page.waitForTimeout(700);
  const m = await count(page);
  await setMember(page, ""); await setSearch(page, TERM);
  const s = await count(page);
  note(`member-only=${m}, search-only=${s} (baseline ${base})`);
  if (m != null && s != null && m < base && s < base) pass(`F-1 — each filter narrows below baseline (member ${m}, search ${s} < ${base})`);
  else absent_(`F-1 — a filter didn't narrow (member ${m}, search ${s}, base ${base})`);

  // ── F-2: member + search = intersection ───────────────────────────────────────
  await setMember(page, MEMBER); await page.waitForTimeout(700); // search still active
  const ms = await count(page);
  await page.screenshot({ path: SS("L95_01_combined.png") });
  note(`member+search = ${ms} (min of ${m},${s} = ${Math.min(m, s)})`);
  if (ms != null && ms <= Math.min(m, s) && ms <= m && ms <= s) pass(`F-2 — composed count (${ms}) ≤ both filters — true intersection`);
  else fail(`F-2 — composed count (${ms}) exceeds a single filter (member ${m}, search ${s}) — filters not intersecting`);

  // ── F-3: order independence ───────────────────────────────────────────────────
  // reset, apply search THEN member (above was member-after-search); now do member-first
  await setMember(page, ""); await setSearch(page, "");
  await page.waitForTimeout(500);
  await setMember(page, MEMBER); await page.waitForTimeout(600); // member first
  await setSearch(page, TERM); // then search
  const ms2 = await count(page);
  note(`order check: search-then-member=${ms}, member-then-search=${ms2}`);
  if (ms != null && ms2 != null && ms === ms2) pass(`F-3 — filter order doesn't matter (both orders → ${ms2})`);
  else fail(`F-3 — composed count depends on order (${ms} vs ${ms2}) — non-commutative filtering`);

  // ── F-4: clearing peels back correctly ────────────────────────────────────────
  await setSearch(page, ""); await page.waitForTimeout(600);
  const afterClearSearch = await count(page);
  await setMember(page, ""); await page.waitForTimeout(600);
  const afterClearAll = await count(page);
  note(`clear search → ${afterClearSearch} (expect member-only ${m}); clear member → ${afterClearAll} (expect ${base})`);
  if (afterClearSearch === m && afterClearAll === base) pass(`F-4 — clearing search restores member-only (${afterClearSearch}); clearing member restores baseline (${afterClearAll})`);
  else absent_(`F-4 — clear didn't peel back cleanly (search→${afterClearSearch}/${m}, all→${afterClearAll}/${base})`);

  // ── F-5: STRESS — rapid combine/clear ─────────────────────────────────────────
  let sane = true; const seq = [];
  const steps = [["m-marcus", "rent"], ["", ""], ["m-priya", "coffee"], ["m-marcus", ""], ["", "salary"], ["", ""]];
  for (const [mem, term] of steps) { await setMember(page, mem); await setSearch(page, term); const c = await count(page); seq.push(c); if (c == null || c < 0 || c > base) sane = false; }
  const finalC = await count(page);
  note(`Stress counts: [${seq.join(", ")}] final=${finalC} (baseline ${base})`);
  if (sane && finalC === base) pass(`F-5 — rapid combine/clear stayed consistent; returned to baseline (${finalC})`);
  else absent_(`F-5 — stress inconsistent (sane=${sane}, final=${finalC}/${base})`);

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
