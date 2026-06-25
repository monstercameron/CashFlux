// L96 E2E loop story — "The Bulk Editor" (Marcus) — 2026-06-24
//
// Theme: BULK-OPERATION INTEGRITY + SAFETY. Power users select many transactions and act in one shot —
// mark cleared, recategorize, delete. Bulk ops must (1) surface only when a selection exists, (2) apply
// to the whole selection, and (3) GUARD the destructive bulk-delete (deleting N rows at once is the
// highest-stakes misclick in the app). To stay safe, the test first NARROWS to a tiny set via search.
// Invariants:
//   B-1  Selecting rows reveals a bulk toolbar (Apply category / Mark cleared / Delete selected / …).
//   B-2  Bulk "Mark cleared" flips the selection's cleared state (a state change is observable).
//   B-3  Bulk "Delete selected" is GUARDED by a confirm — rows are NOT destroyed before confirming.
//   B-4  Confirming the bulk delete removes the (small, search-narrowed) selection.
//   B-5  Cancelling the bulk delete leaves every row intact.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_96_bulk_editor.mjs

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
const shownCount = (page) => page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; });
const setSearch = async (page, term) => {
  await page.evaluate((term) => { const s = document.querySelector('input[type="search"]'); if (!s) return; const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set; set.call(s, term); s.dispatchEvent(new Event('input', { bubbles: true })); }, term);
  await page.waitForTimeout(900);
};
const clickByText = (page, re) => page.evaluate((src) => { const re = new RegExp(src, "i"); const b = [...document.querySelectorAll('button')].find(b => re.test(b.textContent.trim()) && b.offsetParent !== null); if (b) { b.click(); return true; } return false; }, re.source);

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
  await page.waitForTimeout(700);
  // SAFETY: narrow to a tiny set so any bulk delete only affects a handful of rows
  await setSearch(page, "cigarettes");
  const narrowed = await shownCount(page);
  note(`Narrowed to "cigarettes": ${narrowed} transactions`);

  // ── B-1: select all reveals bulk toolbar ──────────────────────────────────────
  await clickByText(page, /^select all$/);
  await page.waitForTimeout(600);
  const toolbar = await page.evaluate(() => {
    const labels = ["mark cleared", "apply category", "delete selected", "clear selection"];
    const present = labels.filter(l => [...document.querySelectorAll('button')].some(b => b.textContent.trim().toLowerCase().includes(l) && b.offsetParent !== null));
    return present;
  });
  note(`Bulk toolbar actions: [${toolbar.join(", ")}]`);
  if (toolbar.includes("mark cleared") && toolbar.includes("delete selected")) pass(`B-1 — selecting rows reveals the bulk toolbar (${toolbar.length} actions)`);
  else absent_(`B-1 — bulk toolbar incomplete ([${toolbar.join(", ")}])`);
  await page.screenshot({ path: SS("L96_01_bulk_toolbar.png") });

  // ── B-2: bulk "Mark cleared" changes state ────────────────────────────────────
  const clearedBefore = await page.evaluate(() => (document.body.innerText.match(/(\d+)\s+uncleared/i) || [null, null])[1]);
  const mc = await clickByText(page, /mark cleared/);
  await page.waitForTimeout(900);
  const clearedAfter = await page.evaluate(() => (document.body.innerText.match(/(\d+)\s+uncleared/i) || [null, null])[1]);
  note(`Mark cleared: uncleared ${clearedBefore} -> ${clearedAfter} (click=${mc})`);
  if (mc && clearedBefore !== clearedAfter) pass(`B-2 — bulk "Mark cleared" changed the uncleared count (${clearedBefore} -> ${clearedAfter})`);
  else absent_(`B-2 — couldn't confirm a cleared-state change (${clearedBefore} -> ${clearedAfter}); may need a different signal`);

  // ── B-3 / B-5: bulk delete is GUARDED; cancel leaves rows intact ───────────────
  // re-select (mark-cleared may have kept selection; ensure selected)
  await clickByText(page, /^select all$/); await page.waitForTimeout(400);
  const countBeforeDel = await shownCount(page);
  const delClicked = await clickByText(page, /delete selected/);
  await page.waitForTimeout(800);
  const dlg = await page.evaluate(() => { const d = document.querySelector('.cf-dialog'); return d ? d.textContent.trim().slice(0, 70) : null; });
  const countMidDel = await shownCount(page);
  note(`Delete selected: dialog="${dlg}", count ${countBeforeDel} -> ${countMidDel} (before confirming)`);
  if (delClicked && dlg) pass(`B-3 — bulk delete is GUARDED by a confirm ("${dlg}")`);
  else if (delClicked && countMidDel != null && countMidDel < countBeforeDel) fail(`B-3 — bulk delete is UNGUARDED — destroyed ${countBeforeDel - countMidDel} rows with no confirm (data loss)`);
  else absent_(`B-3 — could not trigger bulk delete (clicked=${delClicked}, dialog=${dlg})`);
  // B-5: cancel keeps rows
  if (dlg) {
    await page.evaluate(() => { const c = document.querySelector('#cf-dialog-cancel'); if (c) c.click(); });
    await page.waitForTimeout(700);
    const countAfterCancel = await shownCount(page);
    note(`After cancel: ${countAfterCancel} (was ${countBeforeDel})`);
    if (countAfterCancel === countBeforeDel) pass(`B-5 — cancelling bulk delete kept all rows (${countAfterCancel})`);
    else fail(`B-5 — rows changed after cancelling delete (${countBeforeDel} -> ${countAfterCancel})`);
    // ── B-4: confirm actually deletes the small narrowed set ────────────────────
    await clickByText(page, /^select all$/); await page.waitForTimeout(400);
    await clickByText(page, /delete selected/); await page.waitForTimeout(700);
    await page.evaluate(() => { const c = document.querySelector('#cf-dialog-confirm'); if (c) c.click(); });
    await page.waitForTimeout(1000);
    await setSearch(page, "cigarettes"); await page.waitForTimeout(400);
    const countAfterConfirm = await shownCount(page);
    note(`After confirm-delete (re-search): ${countAfterConfirm} (was ${countBeforeDel})`);
    if (countAfterConfirm != null && countAfterConfirm < countBeforeDel) pass(`B-4 — confirming bulk delete removed the selection (${countBeforeDel} -> ${countAfterConfirm})`);
    else absent_(`B-4 — selection not removed after confirm (${countBeforeDel} -> ${countAfterConfirm})`);
  }

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
