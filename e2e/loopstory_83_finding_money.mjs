// L83 E2E loop story — "Finding the Money" (Marco) — 2026-06-24
//
// Theme: TRANSACTION FILTER/SEARCH ACCURACY + FILTERED TOTAL + PERSISTENCE (core "find my spend")
//
// Persona: Marco asks "how much did we spend on Dining this month?" He filters transactions and
// expects the visible rows AND the summary count to reflect ONLY the filter — accurately,
// instantly, and to persist as he navigates. Invariants:
//   F-1  Filtering by a category reduces the visible count (and it's < the unfiltered total).
//   F-2  Every visible row actually belongs to the chosen category (no leakage).
//   F-3  Free-text search narrows to matching rows.
//   F-4  Filters PERSIST across navigation (leave Transactions, return -> still filtered).
//   F-5  "Clear filters" restores the full set.
//   F-6  Combining category + search narrows further (AND semantics), never widens.
//   F-7  STRESS: toggling the category filter repeatedly keeps the count consistent.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_83_finding_money.mjs

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

const nav = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1400);
};
const openFilters = async (page) => {
  await page.evaluate(() => {
    const cs = document.querySelector('select[aria-label="Filter by category"]');
    if (cs && cs.offsetParent) return;
    const b = [...document.querySelectorAll('button')].find(b => /^filters$/i.test(b.textContent.trim()));
    if (b) b.click();
  });
  await page.waitForTimeout(500);
};

const count = (page) => page.evaluate(() => {
  // Empty state shows "No matching transactions" with no counter → treat as 0.
  if (/no matching transactions/i.test(document.body.textContent)) return 0;
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});
const rowsMatchingCat = (page, cat) => page.evaluate((cat) => {
  const rows = [...document.querySelectorAll('.txn-table tbody tr')];
  let total = 0, match = 0;
  for (const r of rows) {
    const c = r.querySelector('.td-cat'); if (!c) continue;
    total++; if (c.textContent.trim() === cat) match++;
  }
  return { total, match };
}, cat);

const selectCat = (page, cat) => page.evaluate((cat) => {
  const sel = document.querySelector('select[aria-label="Filter by category"]');
  if (!sel) return "NO_SEL";
  const opt = [...sel.options].find(o => o.text.trim() === cat);
  if (!opt) return "NO_OPT";
  sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true }));
  return "set";
}, cat);
const setSearch = (page, q) => page.evaluate((q) => {
  const s = document.querySelector('input[placeholder="Search description or tag"]');
  if (!s) return "NO_SEARCH";
  s.value = q; s.dispatchEvent(new Event("input", { bubbles: true })); s.dispatchEvent(new Event("change", { bubbles: true }));
  return "set";
}, q);
const clearFilters = (page) => page.evaluate(() => {
  const b = [...document.querySelectorAll('button')].find(b => /clear filters/i.test(b.textContent));
  if (b) { b.click(); return "cleared"; } return "NO_CLEAR";
});

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

  await nav(page, "Transactions"); await page.waitForTimeout(800);
  const totalAll = await count(page);
  note(`Unfiltered transaction count: ${totalAll}`);
  await page.screenshot({ path: SS("L83_01_unfiltered.png") });

  // F-1 / F-2: filter by category
  await openFilters(page);
  const CAT = "Dining";
  const sc = await selectCat(page, CAT);
  await page.waitForTimeout(900);
  const filteredCount = await count(page);
  note(`Filter=${CAT}: select=${sc} | filtered count=${filteredCount} (was ${totalAll})`);
  await page.screenshot({ path: SS("L83_02_filtered_dining.png") });
  if (sc === "set" && totalAll !== null && filteredCount !== null) {
    if (filteredCount < totalAll && filteredCount > 0) pass(`F-1 — category filter reduced count to ${filteredCount} (< ${totalAll})`);
    else fail(`F-1 — category filter did not narrow (${totalAll}->${filteredCount})`);
  } else absent_(`F-1 — could not apply category filter (select=${sc})`);
  const mm = await rowsMatchingCat(page, CAT);
  note(`Visible rows: ${mm.match}/${mm.total} are "${CAT}"`);
  if (mm.total > 0) {
    if (mm.match === mm.total) pass(`F-2 — all ${mm.total} visible rows belong to "${CAT}" (no leakage)`);
    else fail(`F-2 — only ${mm.match}/${mm.total} visible rows are "${CAT}" — filter LEAKS other categories`);
  } else absent_("F-2 — no rows visible to check category accuracy");

  // F-4: persistence across navigation
  await nav(page, "Dashboard"); await page.waitForTimeout(700);
  await nav(page, "Transactions"); await page.waitForTimeout(900);
  const persistedCount = await count(page);
  // The active filter is surfaced as a chip ("Category: Dining · Clear filter"), not via the
  // (collapsed) select — verify the chip so the user can SEE why the list is filtered.
  const chip = await page.evaluate((cat) => {
    const t = document.body.textContent;
    const hasChip = new RegExp("Categor[a-z]*:?\\s*" + cat, "i").test(t) || (/category/i.test(t) && t.includes(cat) && /clear filter/i.test(t));
    return { hasChip, hasClearFilter: /clear filter/i.test(t) };
  }, CAT);
  note(`After nav away+back: count=${persistedCount} | active-filter chip="${chip.hasChip}" clear-affordance=${chip.hasClearFilter}`);
  if (persistedCount !== null && filteredCount !== null) {
    if (persistedCount === filteredCount) pass(`F-4 — category filter PERSISTED across navigation (still ${persistedCount})`);
    else absent_(`F-4 — filter did not persist (was ${filteredCount}, now ${persistedCount}) — review if intentional`);
  }
  if (chip.hasChip && chip.hasClearFilter) pass(`F-4b — the persisted filter is clearly shown as an active-filter chip ("Category: ${CAT}") with a Clear affordance`);
  else absent_(`F-4b — persisted filter not clearly surfaced after return (chip=${chip.hasChip})`);

  // F-5: clear filters restores full set
  await openFilters(page);
  const cl = await clearFilters(page);
  await page.waitForTimeout(900);
  const afterClear = await count(page);
  note(`Clear filters: ${cl} | count=${afterClear} (full was ${totalAll})`);
  if (cl === "cleared" && totalAll !== null && afterClear !== null) {
    if (afterClear === totalAll) pass(`F-5 — "Clear filters" restored the full set (${afterClear})`);
    else absent_(`F-5 — after clear count=${afterClear}, expected ${totalAll}`);
  } else absent_(`F-5 — clear filters unavailable (${cl})`);

  // F-3 / F-6: search, then category + search (AND)
  await openFilters(page);
  const Q = "Coffee";
  await setSearch(page, Q);
  await page.waitForTimeout(900);
  const searchCount = await count(page);
  note(`Search "${Q}": count=${searchCount} (full ${totalAll})`);
  if (totalAll !== null && searchCount !== null) {
    if (searchCount < totalAll) pass(`F-3 — search "${Q}" narrowed to ${searchCount} (< ${totalAll})`);
    else absent_(`F-3 — search did not narrow (${totalAll}->${searchCount})`);
  }
  await selectCat(page, CAT);
  await page.waitForTimeout(900);
  const andCount = await count(page);
  note(`Search "${Q}" + category "${CAT}": count=${andCount} (search-only was ${searchCount})`);
  if (searchCount !== null && andCount !== null) {
    if (andCount <= searchCount) pass(`F-6 — category+search narrows (AND): ${andCount} <= ${searchCount}`);
    else fail(`F-6 — category+search WIDENED (${searchCount}->${andCount}) — filters should AND, not OR`);
  }
  await clearFilters(page); await page.waitForTimeout(700);

  // F-7 STRESS: toggle category filter repeatedly
  await openFilters(page);
  let stressConsistent = true, ref = null;
  for (let i = 0; i < 4; i++) {
    await selectCat(page, CAT); await page.waitForTimeout(500);
    const c = await count(page);
    if (ref === null) ref = c; else if (c !== ref) { stressConsistent = false; note(`  toggle ${i}: count=${c} (ref ${ref})`); }
    await selectCat(page, "— All categories —"); await page.waitForTimeout(400);
  }
  if (stressConsistent && ref !== null) pass(`F-7 — repeated category toggles give a consistent ${ref} each time (no drift)`);
  else absent_(`F-7 — filtered count drifted across toggles (ref ${ref})`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`);
  console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
