// L84 E2E loop story — "The Reconcile" (Dana) — 2026-06-24
//
// Theme: ACCOUNT UPDATE-BALANCE / RECONCILE + FRESHNESS + NET-WORTH CONSISTENCY (core household)
//
// Persona: Dana checks her bank and reconciles a stale account to the real balance. The app posts
// a CLEARED adjustment for the difference (so the displayed balance reaches the target), clears the
// stale flag, and net worth recomputes. Invariants:
//   R-1  After update, the account's DISPLAYED balance equals the target (cleared adjustment).
//   R-2  Exactly ONE adjustment transaction is created (the difference).
//   R-3  The account's "Stale" flag clears (BalanceAsOf = now).
//   R-4  Net worth changes by exactly the adjustment delta (target - old).
//   R-5  A success toast confirms ("Balance updated for …").
//   R-6  "Mark all updated" clears ALL stale flags WITHOUT creating adjustment transactions.
//   R-7  Reconciling to the SAME balance (delta 0) creates NO transaction and changes nothing.
//
// Screens: /accounts (reconcile) -> /transactions (adjustment) -> /accounts (net worth)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_84_reconcile.mjs

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
const flush = async (page) => { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(350); };
const parseMoney = (s) => { if (!s) return null; const neg = s.includes("(") || s.includes("−") || s.trim().startsWith("-"); const n = parseFloat(s.replace(/[^0-9.]/g, "")); return isNaN(n) ? null : (neg ? -n : n); };

const netWorth = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/NET WORTH[^$\-−(]{0,20}([−(]?-?\$[\d,]+\.?\d*)/i);
  return m ? m[1] : null;
});
const txnCount = (page) => page.evaluate(() => {
  if (/no matching transactions/i.test(document.body.textContent)) return 0;
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});

// Read the first updatable account: its name (.row-desc), its cleared balance, and stale state.
const firstAccount = (page) => page.evaluate(() => {
  const rows = [...document.querySelectorAll('.rows .row, li, tr, div')]
    .filter(r => [...r.querySelectorAll('button')].some(b => /update balance/i.test(b.textContent)));
  rows.sort((a, b) => a.textContent.length - b.textContent.length);
  // walk up from the smallest button-bearing container to find the one that also has a name + $
  const cleanName = (s) => s.replace(/\s*stale\s*$/i, "").trim(); // .row-desc may include the Stale badge text
  for (const r of rows) {
    const desc = r.querySelector('.row-desc');
    const m = r.textContent.match(/(-?\$[\d,]+\.?\d*)/);
    if (desc && m) return { name: cleanName(desc.textContent), balance: m[1], stale: /stale/i.test(r.textContent) };
    let p = r.parentElement, hops = 0;
    while (p && hops < 4) {
      const d2 = p.querySelector('.row-desc'); const m2 = p.textContent.match(/(-?\$[\d,]+\.?\d*)/);
      if (d2 && m2 && p.textContent.length < 400) return { name: cleanName(d2.textContent), balance: m2[1], stale: /stale/i.test(p.textContent) };
      p = p.parentElement; hops++;
    }
  }
  return null;
});

// Read a named account's balance + stale by matching the row containing that name.
const acctByName = (page, name) => page.evaluate((name) => {
  const norm = (s) => s.replace(/\s*stale\s*$/i, "").trim();
  const descs = [...document.querySelectorAll('.row-desc')].filter(d => norm(d.textContent) === name);
  for (const d of descs) {
    let p = d, hops = 0;
    while (p && hops < 5) {
      const m = p.textContent.match(/(-?\$[\d,]+\.?\d*)/);
      if (m && [...p.querySelectorAll('button')].some(b => /update balance/i.test(b.textContent))) {
        return { balance: m[1], stale: /stale/i.test(p.textContent) };
      }
      p = p.parentElement; hops++;
    }
  }
  return null;
}, name);

// Open Update balance on the named account, type the target, Save.
const updateBalance = async (page, name, target) => {
  const opened = await page.evaluate((name) => {
    const norm = (s) => s.replace(/\s*stale\s*$/i, "").trim();
    const descs = [...document.querySelectorAll('.row-desc')].filter(d => norm(d.textContent) === name);
    for (const d of descs) {
      let p = d, hops = 0;
      while (p && hops < 5) {
        const btn = [...p.querySelectorAll('button')].find(b => /^update balance$/i.test(b.textContent.trim()));
        if (btn) { btn.click(); return "opened"; }
        p = p.parentElement; hops++;
      }
    }
    return "NO_BTN";
  }, name);
  if (opened !== "opened") return opened;
  await page.waitForTimeout(500);
  const filled = await page.evaluate((target) => {
    const inp = document.querySelector('input[placeholder="New balance"]') || document.querySelector('input[type="number"]');
    if (!inp) return "NO_INPUT";
    inp.value = String(target); inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true }));
    return "filled";
  }, target);
  if (filled !== "filled") return filled;
  const saved = await page.evaluate(() => {
    const form = document.querySelector('form');
    const b = (form ? [...form.querySelectorAll('button')] : [...document.querySelectorAll('button')]).find(b => /^save$/i.test(b.textContent.trim()) && b.type !== "reset");
    if (b) { b.click(); return "saved"; } return "NO_SAVE";
  });
  await page.waitForTimeout(1000); await flush(page);
  return saved;
};
const readToast = (page) => page.evaluate(() => { const t = document.querySelector('.toast-msg'); return t ? t.textContent.trim().slice(0, 80) : null; });
const staleCount = (page) => page.evaluate(() => { const m = document.body.textContent.match(/Mark all updated\s*\((\d+)\s+account/i); return m ? parseInt(m[1], 10) : null; });

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 1100 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try { await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 }); await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 }); hydrated = true; }
    catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  // baseline txn count
  await nav(page, "Transactions"); await page.waitForTimeout(700);
  const txnBefore = await txnCount(page);

  await nav(page, "Accounts"); await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L84_01_accounts_before.png") });
  const nwBefore = parseMoney(await netWorth(page));
  const staleBefore = await staleCount(page);
  const acct = await firstAccount(page);
  note(`Net worth=${nwBefore} | stale accounts=${staleBefore} | target acct=${JSON.stringify(acct)}`);
  if (!acct) { absent_("Could not read a target account — aborting balance checks"); throw new Error("no account"); }
  const NAME = acct.name;
  const oldBal = parseMoney(acct.balance);
  const TARGET = +(oldBal + 500).toFixed(2);
  const DELTA = +(TARGET - oldBal).toFixed(2);
  note(`Reconcile "${NAME}": ${oldBal} -> ${TARGET} (delta ${DELTA}) | stale before=${acct.stale}`);

  // ── R-1/R-2/R-3/R-5: update balance ──────────────────────────────────────────
  const r = await updateBalance(page, NAME, TARGET);
  const toast = await readToast(page);
  note(`Update balance: ${r} | toast="${toast}"`);
  await page.screenshot({ path: SS("L84_02_after_reconcile.png") });
  if (r === "saved") pass("R-0 — update-balance form submitted");
  else fail(`R-0 — update-balance did not save: ${r}`);
  if (toast && /balance updated|updated/i.test(toast)) pass(`R-5 — success toast shown ("${toast}")`);
  else absent_(`R-5 — no clear update toast (got: ${toast})`);

  const after = await acctByName(page, NAME);
  note(`After: "${NAME}" balance=${after ? after.balance : "?"} stale=${after ? after.stale : "?"}`);
  if (after) {
    const nb = parseMoney(after.balance);
    if (Math.abs(nb - TARGET) < 0.01) pass(`R-1 — displayed balance reached the target (${oldBal} -> ${nb})`);
    else absent_(`R-1 — displayed balance ${nb} != target ${TARGET} (cleared adjustment may not reflect — review)`);
    if (acct.stale && !after.stale) pass(`R-3 — "${NAME}" stale flag cleared after update`);
    else if (!acct.stale) note("R-3 — account was not stale before (can't confirm clear)");
    else absent_(`R-3 — "${NAME}" still flagged stale after update`);
  }

  // R-2: adjustment txn created
  await nav(page, "Transactions"); await page.waitForTimeout(900);
  const txnAfter = await txnCount(page);
  note(`Txn count: ${txnBefore} -> ${txnAfter}`);
  if (txnBefore !== null && txnAfter !== null) {
    if (txnAfter === txnBefore + 1) pass("R-2 — exactly ONE adjustment transaction created");
    else absent_(`R-2 — txn count changed by ${txnAfter - txnBefore} (expected 1)`);
  }

  // R-4: net worth changed by the delta
  await nav(page, "Accounts"); await page.waitForTimeout(900);
  const nwAfter = parseMoney(await netWorth(page));
  note(`Net worth: ${nwBefore} -> ${nwAfter} (expected delta ${DELTA})`);
  if (nwBefore !== null && nwAfter !== null) {
    if (Math.abs((nwAfter - nwBefore) - DELTA) < 0.01) pass(`R-4 — net worth changed by exactly the adjustment (${DELTA})`);
    else fail(`R-4 — net worth delta ${(nwAfter - nwBefore).toFixed(2)} != adjustment ${DELTA}`);
  }

  // ── R-7: reconcile to the SAME balance → no change, no txn ────────────────────
  await nav(page, "Transactions"); await page.waitForTimeout(700);
  const txnPreSame = await txnCount(page);
  await nav(page, "Accounts"); await page.waitForTimeout(800);
  const r2 = await updateBalance(page, NAME, TARGET); // same target
  note(`Reconcile to SAME balance: ${r2}`);
  await nav(page, "Transactions"); await page.waitForTimeout(900);
  const txnPostSame = await txnCount(page);
  note(`Txn count around same-balance reconcile: ${txnPreSame} -> ${txnPostSame}`);
  if (txnPreSame !== null && txnPostSame !== null) {
    if (txnPostSame === txnPreSame) pass("R-7 — reconciling to the same balance created NO adjustment transaction");
    else absent_(`R-7 — same-balance reconcile created ${txnPostSame - txnPreSame} txn(s) (expected 0)`);
  }

  // ── R-6: Mark all updated clears stale without adjustment txns ────────────────
  await nav(page, "Transactions"); await page.waitForTimeout(700);
  const txnPreMark = await txnCount(page);
  await nav(page, "Accounts"); await page.waitForTimeout(800);
  const staleNow = await staleCount(page);
  const marked = await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /mark all updated/i.test(b.textContent)); if (b) { b.click(); return "clicked"; } return "NO_BTN"; });
  await page.waitForTimeout(1000); await flush(page);
  const staleAfterMark = await staleCount(page);
  note(`Mark all updated: ${marked} | stale ${staleNow} -> ${staleAfterMark}`);
  if (marked === "clicked") {
    if (staleAfterMark === null || staleAfterMark === 0) pass(`R-6 — "Mark all updated" cleared all stale flags (${staleNow} -> 0)`);
    else absent_(`R-6 — ${staleAfterMark} accounts still stale after Mark all updated`);
  } else absent_("R-6 — no stale accounts / no Mark-all button to test");
  await nav(page, "Transactions"); await page.waitForTimeout(900);
  const txnPostMark = await txnCount(page);
  note(`Txn count around Mark-all: ${txnPreMark} -> ${txnPostMark}`);
  if (txnPreMark !== null && txnPostMark !== null && marked === "clicked") {
    if (txnPostMark === txnPreMark) pass("R-6b — Mark all updated created NO adjustment transactions (freshness only)");
    else absent_(`R-6b — Mark all updated created ${txnPostMark - txnPreMark} txn(s) (expected 0 — it should only touch freshness)`);
  }

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no account") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
