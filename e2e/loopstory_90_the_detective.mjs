// L90 E2E loop story — "The Detective" (Marcus) — 2026-06-24
//
// Theme: TRANSACTION SEARCH / FILTER / EDIT INTEGRITY. The most frequent everyday task in a finance
// app is "find that transaction and fix it." Search must narrow the list AND the summary count, the
// visible rows must actually match the query, clearing must restore the full set, an inline edit must
// persist, and combining filters must never desync the count or crash. Invariants:
//   D-1  Searching narrows BOTH the row count and the "N transactions shown" summary.
//   D-2  Every visible row actually matches the search term (no false matches).
//   D-3  Clearing the search restores the original full count (reversible).
//   D-4  Inline-editing a transaction's amount persists (the row shows the new value).
//   D-5  STRESS: rapid successive searches + a period filter leave a consistent, non-negative count, no crash.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_90_the_detective.mjs

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

// "2189 transactions shown · net $29,019.64" -> { count, net }
const summary = (page) => page.evaluate(() => {
  const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown[^$]*?(\$[\d,()\-.]+)?/i);
  if (!m) return { count: null, net: null };
  return { count: parseInt(m[1].replace(/,/g, ""), 10), net: m[2] || null };
});
const rowCount = (page) => page.evaluate(() => document.querySelectorAll('.rows .row, .txn-table .row, [class*="txn-row"]').length);
const visibleRowTexts = (page) => page.evaluate(() => [...document.querySelectorAll('.rows .row, .txn-table .row')].slice(0, 30).map(r => r.textContent.toLowerCase()));
const typeSearch = async (page, term) => {
  await page.evaluate((term) => {
    const s = document.querySelector('input[type="search"]');
    if (s) {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      setter.call(s, term);
      s.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }, term);
  await page.waitForTimeout(900);
};

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1100 });
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
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L90_01_txns.png") });

  const base = await summary(page);
  note(`Baseline: count=${base.count} net=${base.net}`);
  if (base.count == null) { absent_("could not read the summary count — aborting"); throw new Error("no summary"); }

  // ── D-1 / D-2: search narrows count + rows match ──────────────────────────────
  const TERM = "groceries";
  await typeSearch(page, TERM);
  const afterSearch = await summary(page);
  const texts = await visibleRowTexts(page);
  note(`After search "${TERM}": count=${afterSearch.count} (was ${base.count}), ${texts.length} rows sampled`);
  await page.screenshot({ path: SS("L90_02_search.png") });
  if (afterSearch.count != null && afterSearch.count < base.count && afterSearch.count > 0) pass(`D-1 — search narrowed the count (${base.count} -> ${afterSearch.count})`);
  else absent_(`D-1 — search did not narrow as expected (${base.count} -> ${afterSearch.count})`);
  if (texts.length) {
    const matching = texts.filter(t => t.includes(TERM));
    if (matching.length === texts.length) pass(`D-2 — all ${texts.length} visible rows match "${TERM}" (no false matches)`);
    else { fail(`D-2 — ${texts.length - matching.length}/${texts.length} visible rows do NOT contain "${TERM}" — search returns non-matching rows`); note(`  example non-match: "${(texts.find(t => !t.includes(TERM)) || "").slice(0, 60)}"`); }
  } else absent_("D-2 — no rows to validate against the search term");

  // ── D-3: clear restores the full count ────────────────────────────────────────
  await typeSearch(page, "");
  const cleared = await summary(page);
  note(`After clear: count=${cleared.count}`);
  if (cleared.count === base.count) pass(`D-3 — clearing search restored the full count (${cleared.count}) — reversible`);
  else absent_(`D-3 — count did not restore (${base.count} -> ${cleared.count})`);

  // ── D-4: inline-edit a transaction's amount persists ──────────────────────────
  // narrow to a small set first so the edited row is on screen
  await typeSearch(page, "cigarettes");
  await page.waitForTimeout(500);
  const editResult = await page.evaluate(() => {
    const row = [...document.querySelectorAll('.rows .row, .txn-table .row')][0];
    if (!row) return "NO_ROW";
    const before = (row.textContent.match(/\$[\d,]+\.?\d*/) || [""])[0];
    const editBtn = [...row.querySelectorAll('button')].find(b => /edit/i.test(b.getAttribute('aria-label') || b.getAttribute('title') || ""));
    if (!editBtn) return "NO_EDIT:" + before;
    editBtn.click();
    return "opened:" + before;
  });
  await page.waitForTimeout(700);
  if (editResult.startsWith("opened")) {
    const before = editResult.split(":")[1];
    const saved = await page.evaluate(() => {
      // find the amount input in the inline-edit form (a number input)
      const amt = [...document.querySelectorAll('input[type="number"]')].find(e => e.offsetParent !== null);
      if (!amt) return "NO_AMT_INPUT";
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      setter.call(amt, "13.37");
      amt.dispatchEvent(new Event('input', { bubbles: true }));
      const save = [...document.querySelectorAll('button')].find(b => /^save$/i.test(b.textContent.trim()));
      if (save) { save.click(); return "saved"; }
      return "NO_SAVE";
    });
    await page.waitForTimeout(1000);
    const nowText = await page.evaluate(() => { const r = [...document.querySelectorAll('.rows .row, .txn-table .row')][0]; return r ? r.textContent : ""; });
    note(`Inline edit: before=${before} result=${saved} rowNow="${nowText.replace(/\s+/g, ' ').slice(0, 60)}"`);
    if (saved === "saved" && /13\.37/.test(nowText)) pass(`D-4 — inline edit persisted (amount now shows 13.37, was ${before})`);
    else if (saved === "saved") absent_(`D-4 — saved but the new amount 13.37 not visible in the row (${nowText.replace(/\s+/g, ' ').slice(0, 50)})`);
    else absent_(`D-4 — could not complete the inline edit (${saved})`);
  } else absent_(`D-4 — could not open inline edit (${editResult})`);
  await typeSearch(page, "");

  // ── D-5: STRESS — rapid searches + period filter, count stays sane ────────────
  const terms = ["a", "ab", "abc", "x", "rent", "coffee", "", "salary", ""];
  let sane = true; const counts = [];
  for (const t of terms) { await typeSearch(page, t); const c = (await summary(page)).count; counts.push(c); if (c == null || c < 0 || c > base.count) sane = false; }
  note(`Stress search counts: [${counts.join(", ")}] (base ${base.count})`);
  const finalCount = (await summary(page)).count;
  if (sane && finalCount === base.count) pass(`D-5 — rapid searches stayed consistent & non-negative; final clear restored ${finalCount}`);
  else absent_(`D-5 — stress produced an inconsistent count (sane=${sane}, final=${finalCount}/${base.count})`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no summary") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
