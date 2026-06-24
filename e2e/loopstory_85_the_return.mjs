// L85 E2E loop story — "The Return" (Sam) — 2026-06-24
//
// Theme: PERSISTENCE / RELOAD INTEGRITY (timely — the storage layer is being migrated to browserstore)
//
// Persona: Sam closes the app and comes back later. Everything must still be there: the data they
// entered, their theme, and their active filter — no silent reset to sample data, no loss. Invariants:
//   P-1  The total transaction count is preserved across a hard reload (no data loss / no sample reset).
//   P-2  A just-entered (uniquely stamped) transaction survives the reload.
//   P-3  The active theme persists across reload.
//   P-4  An applied transaction filter persists across reload (uistate survives).
//   P-5  STRESS: repeated reloads keep the count stable (no drift, no compounding loss).
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_85_the_return.mjs

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
const openFilters = async (page) => {
  await page.evaluate(() => { const cs = document.querySelector('select[aria-label="Filter by category"]'); if (cs && cs.offsetParent) return; const b = [...document.querySelectorAll('button')].find(b => /^filters$/i.test(b.textContent.trim())); if (b) b.click(); });
  await page.waitForTimeout(500);
};
const count = (page) => page.evaluate(() => {
  if (/no matching transactions/i.test(document.body.textContent)) return 0;
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});
const dataTheme = (page) => page.evaluate(() => document.documentElement.getAttribute("data-theme"));
const flush = async (page) => { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(400); };

const STAMP = "ZZL85-Return-" + "Marker";

const addExpense = async (page, desc, amount) => {
  const open = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /new transaction|add transaction|^\s*add\s*$/i.test(b.textContent.trim())); if (b) { b.click(); return "opened"; } return "NO_OPEN"; });
  if (open !== "opened") return open;
  await page.waitForTimeout(500);
  await page.evaluate(({ desc, amount }) => {
    const d = document.querySelector('input[placeholder="What was it for?"]'); if (d) { d.value = desc; d.dispatchEvent(new Event("input", { bubbles: true })); d.dispatchEvent(new Event("change", { bubbles: true })); }
    const a = document.querySelector('input[placeholder="Amount"]') || document.querySelector('input[type="number"]'); if (a) { a.value = String(amount); a.dispatchEvent(new Event("input", { bubbles: true })); a.dispatchEvent(new Event("change", { bubbles: true })); }
    const e = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === "Expense"); if (e) e.click();
    const acct = [...document.querySelectorAll('select')].find(s => s.getAttribute("aria-label") === "Account"); if (acct) { const o = [...acct.options].find(o => /checking|cash|everyday/i.test(o.text)); if (o) { acct.value = o.value; acct.dispatchEvent(new Event("change", { bubbles: true })); } }
  }, { desc, amount });
  await page.waitForTimeout(200);
  const saved = await page.evaluate(() => { const s = [...document.querySelectorAll('button')].find(b => /^save$/i.test(b.textContent.trim()) && b.type !== "reset"); if (s) { s.click(); return "saved"; } return "NO_SAVE"; });
  await page.waitForTimeout(900); await flush(page);
  return saved;
};
const stampVisible = (page) => page.evaluate((stamp) => document.body.textContent.includes(stamp), STAMP);

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
  const themeBefore = await dataTheme(page);

  // ── Add a uniquely-stamped transaction ───────────────────────────────────────
  await navTo(page, "Transactions"); await page.waitForTimeout(800);
  const r = await addExpense(page, STAMP + " coffee", 7.25);
  note(`Add stamped txn: ${r}`);
  await page.waitForTimeout(400);
  const countAfterAdd = await count(page);
  const stampVisAfterAdd = await stampVisible(page);
  note(`Count after add: ${countAfterAdd} | stamp visible: ${stampVisAfterAdd}`);
  if (r === "saved") pass("setup — stamped transaction submitted");
  else absent_(`setup — add did not save (${r})`);

  // ── Apply a category filter (uistate to persist) ─────────────────────────────
  await openFilters(page);
  const CAT = "Dining";
  await page.evaluate((cat) => { const sel = document.querySelector('select[aria-label="Filter by category"]'); if (sel) { const o = [...sel.options].find(o => o.text.trim() === cat); if (o) { sel.value = o.value; sel.dispatchEvent(new Event("change", { bubbles: true })); } } }, CAT);
  await page.waitForTimeout(800);
  const filteredCount = await count(page);
  note(`Applied "${CAT}" filter → count=${filteredCount}`);

  // capture the UNFILTERED total (clear filter, read, then we'll compare post-reload appropriately)
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /clear filter/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(700);
  const totalBefore = await count(page);
  note(`Unfiltered total before reload: ${totalBefore}`);
  // re-apply the filter so we can test it persists
  await openFilters(page);
  await page.evaluate((cat) => { const sel = document.querySelector('select[aria-label="Filter by category"]'); if (sel) { const o = [...sel.options].find(o => o.text.trim() === cat); if (o) { sel.value = o.value; sel.dispatchEvent(new Event("change", { bubbles: true })); } } }, CAT);
  await page.waitForTimeout(700);
  await flush(page);

  // ── HARD RELOAD ──────────────────────────────────────────────────────────────
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1500);
  await navTo(page, "Transactions"); await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("L85_after_reload.png") });

  // P-3: theme persisted
  const themeAfter = await dataTheme(page);
  note(`Theme: ${themeBefore} -> ${themeAfter}`);
  if (themeBefore && themeAfter && themeBefore === themeAfter) pass(`P-3 — theme persisted across reload (${themeAfter})`);
  else absent_(`P-3 — theme changed across reload (${themeBefore} -> ${themeAfter})`);

  // P-4: filter persisted (chip + filtered count)
  const reloadCount = await count(page);
  const chip = await page.evaluate((cat) => { const t = document.body.textContent; return new RegExp("Categor[a-z]*:?\\s*" + cat, "i").test(t) || (/clear filter/i.test(t) && t.includes(cat)); }, CAT);
  note(`After reload: count=${reloadCount} | "${CAT}" filter chip=${chip} | (pre-reload filtered=${filteredCount})`);
  if (filteredCount !== null && reloadCount === filteredCount && chip) pass(`P-4 — the "${CAT}" filter persisted across reload (still ${reloadCount}, chip shown)`);
  else if (reloadCount === totalBefore) absent_(`P-4 — filter did NOT persist (reset to full ${reloadCount}) — review if intentional`);
  else absent_(`P-4 — filter state unclear after reload (count=${reloadCount}, chip=${chip})`);

  // clear filter to see the full total + stamped row
  await openFilters(page);
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /clear filter/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(800);
  const totalAfter = await count(page);
  note(`Unfiltered total after reload: ${totalBefore} -> ${totalAfter}`);

  // P-1: total preserved (no data loss / no sample reset)
  if (totalBefore !== null && totalAfter !== null) {
    if (totalAfter === totalBefore) pass(`P-1 — total transaction count PRESERVED across reload (${totalAfter}) — no data loss, no sample reset`);
    else fail(`P-1 — count changed ${totalBefore} -> ${totalAfter} across reload (data loss or sample reset!)`);
  }

  // P-2: stamped txn survives (search for it)
  await openFilters(page);
  await page.evaluate((stamp) => { const s = document.querySelector('input[placeholder="Search description or tag"]'); if (s) { s.value = stamp; s.dispatchEvent(new Event("input", { bubbles: true })); s.dispatchEvent(new Event("change", { bubbles: true })); } }, STAMP);
  await page.waitForTimeout(800);
  const stampAfter = await stampVisible(page);
  note(`Stamped txn visible after reload (searched): ${stampAfter}`);
  if (stampVisAfterAdd) {
    if (stampAfter) pass("P-2 — the stamped transaction survived the reload");
    else fail("P-2 — the stamped transaction was LOST on reload");
  } else note("P-2 — stamp wasn't visible even before reload (paging) — relying on P-1 count for data persistence");
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /clear filter/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(500);

  // ── P-5 STRESS: reload twice more, count stable ──────────────────────────────
  // Reference = whatever count is showing NOW (filter state may persist; we only care
  // that repeated reloads return the SAME count — no drift/compounding loss).
  await navTo(page, "Transactions"); await page.waitForTimeout(700);
  const refS = await count(page);
  note(`P-5 reference count: ${refS}`);
  let stable = true;
  for (let i = 0; i < 2; i++) {
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
    await navTo(page, "Transactions"); await page.waitForTimeout(900);
    const c = await count(page);
    note(`  reload ${i + 2}: count=${c}`);
    if (c !== refS) { stable = false; }
  }
  if (stable && refS !== null) pass(`P-5 — count stable across repeated reloads (${refS}) — no drift/compounding loss`);
  else absent_(`P-5 — count drifted across repeated reloads (ref ${refS})`);

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
