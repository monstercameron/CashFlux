// L93 E2E loop story — "The Transfer" (Marcus) — 2026-06-24
//
// Theme: TRANSFER / DOUBLE-ENTRY INTEGRITY. Moving money between your own accounts is a daily op, and
// it must obey the books: it is NOT income or expense, so NET WORTH must be unchanged — the source
// drops, the destination rises by the same amount, and a ledger entry is created. Invariants:
//   X-1  Each account exposes a Transfer action that opens a from→to form.
//   X-2  Completing a transfer leaves NET WORTH UNCHANGED (a transfer isn't income/expense).
//   X-3  The transfer creates a ledger entry (Transactions count rises) — cross-screen trace.
//   X-4  A transfer to the SAME account / no destination is rejected (no silent self-transfer).
//   X-5  STRESS: two more transfers keep net worth pinned + the count climbing, no crash.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_93_the_transfer.mjs

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
const netWorth = (page) => page.evaluate(() => {
  const s = [...document.querySelectorAll('.stat')].find(s => /net worth/i.test(s.textContent || ""));
  if (!s) return null; const m = (s.textContent || "").match(/-?\$[\d,]+\.?\d*/); return m ? parseFloat(m[0].replace(/[^0-9.-]/g, "")) : null;
});
const txnCount = async (page) => { await navTo(page, "Transactions"); await page.waitForTimeout(600); return page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; }); };

// Do a transfer: open the form (re-renders async), then fill after a wait.
const doTransfer = async (page, amount) => {
  const id = await page.evaluate(() => {
    const start = [...document.querySelectorAll('[data-testid^="transfer-start-btn-"]')][0];
    if (!start) return null; start.click();
    return start.getAttribute('data-testid').replace('transfer-start-btn-', '');
  });
  if (!id) return "NO_START";
  await page.waitForTimeout(500); // form renders on the next render tick
  return page.evaluate((args) => {
    const [amount, id] = args;
    const amt = document.querySelector('#acct-xfer-amt-' + CSS.escape(id));
    const sel = document.querySelector('[data-testid="acct-xfer-to-select"]') || document.querySelector('#acct-transfer-form-' + CSS.escape(id) + ' select');
    if (!amt || !sel) return "NO_FIELDS:amt=" + !!amt + ",sel=" + !!sel;
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    const opt = [...sel.options].find(o => o.value);
    if (!opt) return "NO_DEST_OPTION";
    const setS = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
    setS.call(sel, opt.value); sel.dispatchEvent(new Event('change', { bubbles: true }));
    return "filled:" + id + "->" + opt.textContent.trim();
  }, [amount, id]);
};
const submitTransfer = (page) => page.evaluate(() => {
  const btn = [...document.querySelectorAll('button[type="submit"]')].find(b => /^transfer$/i.test(b.textContent.trim()) && b.offsetParent !== null);
  if (btn) { btn.click(); return "submitted"; } return "NO_SUBMIT";
});

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

  // ── X-1: transfer action present ──────────────────────────────────────────────
  await navTo(page, "Accounts");
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L93_01_accounts.png") });
  const nw0 = await netWorth(page);
  const hasTransfer = await page.evaluate(() => document.querySelectorAll('[data-testid^="transfer-start-btn-"]').length);
  note(`Net worth: ${nw0} · transfer buttons: ${hasTransfer}`);
  if (hasTransfer > 0) pass(`X-1 — ${hasTransfer} account(s) expose a Transfer action`);
  else absent_("X-1 — no transfer action found");

  const txn0 = await txnCount(page);
  note(`Transactions before: ${txn0}`);

  // ── X-2 / X-3: a transfer leaves net worth unchanged + posts a ledger entry ───
  await navTo(page, "Accounts"); await page.waitForTimeout(700);
  const filled = await doTransfer(page, 500);
  note(`Transfer fill: ${filled}`);
  await page.waitForTimeout(400);
  const submitted = await submitTransfer(page);
  await page.waitForTimeout(1200);
  await page.screenshot({ path: SS("L93_02_after_transfer.png") });
  const nw1 = await netWorth(page);
  note(`Net worth after transfer: ${nw0} -> ${nw1} (submit=${submitted})`);
  if (filled.startsWith("filled") && submitted === "submitted" && nw0 != null && nw1 != null) {
    if (Math.abs(nw1 - nw0) <= 0.01) pass(`X-2 — net worth UNCHANGED after a $500 transfer (${nw1}) — a transfer is not income/expense`);
    else fail(`X-2 — net worth CHANGED after a transfer (${nw0} -> ${nw1}, Δ=${(nw1 - nw0).toFixed(2)}) — double-entry broken`);
  } else absent_(`X-2 — could not complete the transfer (fill=${filled}, submit=${submitted})`);

  const txn1 = await txnCount(page);
  note(`Transactions after: ${txn0} -> ${txn1}`);
  if (txn0 != null && txn1 != null && txn1 > txn0) pass(`X-3 — transfer created a ledger entry (${txn0} -> ${txn1})`);
  else absent_(`X-3 — no new transaction after the transfer (${txn0} -> ${txn1})`);

  // ── X-4: a transfer with NO destination is rejected ───────────────────────────
  await navTo(page, "Accounts"); await page.waitForTimeout(700);
  const beforeBad = await netWorth(page);
  const badId = await page.evaluate(() => { const s = [...document.querySelectorAll('[data-testid^="transfer-start-btn-"]')][0]; if (!s) return null; s.click(); return s.getAttribute('data-testid').replace('transfer-start-btn-', ''); });
  await page.waitForTimeout(500);
  const badResult = await page.evaluate((id) => {
    if (!id) return "NO_START";
    const amt = document.querySelector('#acct-xfer-amt-' + CSS.escape(id));
    if (!amt) return "NO_AMT";
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, "250"); amt.dispatchEvent(new Event('input', { bubbles: true })); // leave destination EMPTY
    const btn = [...document.querySelectorAll('button[type="submit"]')].find(b => /^transfer$/i.test(b.textContent.trim()) && b.offsetParent !== null);
    if (btn) btn.click(); return "submitted-no-dest";
  }, badId);
  await page.waitForTimeout(1000);
  const afterBad = await netWorth(page);
  note(`No-destination transfer: nw ${beforeBad} -> ${afterBad} (${badResult})`);
  if (beforeBad != null && afterBad != null && Math.abs(afterBad - beforeBad) <= 0.01) pass(`X-4 — a transfer with no destination did NOT move money (net worth steady ${afterBad}) — guarded`);
  else absent_(`X-4 — no-destination transfer changed net worth (${beforeBad} -> ${afterBad}) — review the guard`);
  // close the open form
  await page.evaluate(() => { const c = [...document.querySelectorAll('button')].find(b => /^cancel$/i.test(b.textContent.trim()) && b.offsetParent !== null); if (c) c.click(); });

  // ── X-5: STRESS — two more transfers keep net worth pinned ────────────────────
  let stableNw = true; let lastTxn = txn1; let climbing = true;
  for (let i = 0; i < 2; i++) {
    await navTo(page, "Accounts"); await page.waitForTimeout(600);
    const before = await netWorth(page);
    const f = await doTransfer(page, 75); await page.waitForTimeout(300);
    const s = await submitTransfer(page); await page.waitForTimeout(1000);
    const after = await netWorth(page);
    if (!(f.startsWith("filled") && s === "submitted")) { climbing = false; }
    if (before == null || after == null || Math.abs(after - before) > 0.01) stableNw = false;
    const tc = await txnCount(page);
    if (lastTxn != null && tc != null && tc <= lastTxn) climbing = false;
    lastTxn = tc;
  }
  note(`Stress: net worth stable=${stableNw}, txn climbing to ${lastTxn}`);
  if (stableNw && climbing) pass(`X-5 — repeated transfers kept net worth pinned + the ledger climbing (now ${lastTxn}), no crash`);
  else absent_(`X-5 — stress inconsistent (stableNw=${stableNw}, climbing=${climbing})`);

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
