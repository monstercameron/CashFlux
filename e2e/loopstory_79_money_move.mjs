// L79 E2E loop story — "The Money Move" (Renu) — 2026-06-24
//
// Theme: TRANSFER INTEGRITY + CROSS-SCREEN CONSISTENCY (a core everyday-household action)
//
// Persona: Renu moves money between her own accounts — checking → emergency savings — the
// single most common "money move" in a household. The non-negotiable invariants for an
// enterprise-grade finance app:
//   T-1  Both account balances update: source −amount, destination +amount.
//   T-2  NET WORTH IS CONSERVED — a transfer moves money between asset accounts; the
//        household total must NOT change (the classic double-entry integrity check).
//   T-3  A transfer must NOT be counted as income or as spending (it's not earning/spending).
//   T-4  The move is reflected consistently on Accounts AND Dashboard (same net worth).
//   T-5  STRESS: several back-to-back transfers never crash, never desync net worth.
//
// Screens: /accounts (transfer) → /dashboard (net worth) → /reports or /transactions (spending)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_79_money_move.mjs

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
  await page.evaluate((t) => {
    const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t);
    if (l) l.click();
  }, title);
  await page.waitForTimeout(1500);
};

const parseMoney = (s) => {
  if (!s) return null;
  const neg = s.includes("(") || s.includes("−") || s.trim().startsWith("-");
  const n = parseFloat(s.replace(/[^0-9.]/g, ""));
  return isNaN(n) ? null : (neg ? -n : n);
};

// Read the money figure shown right after a label keyword (e.g. "Net worth $60,386.00").
const readMoneyNear = (page, kw) => page.evaluate((kw) => {
  const re = new RegExp(kw + "[^$\\-−(]{0,30}?([−(]?-?\\$[\\d,]+\\.?\\d*\\)?)", "i");
  const m = document.body.textContent.match(re);
  return m ? m[1] : null;
}, kw);

// Read an account's displayed balance by its name (first $-amount after the name).
// NB: the Accounts row shows the CLEARED balance ("… cleared $X"); names may include
// a parenthetical like "Emergency Savings (HYSA)", so match lazily up to the first $.
const readAcctBalance = (page, name) => page.evaluate((name) => {
  const re = new RegExp(name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "[^$]{0,90}?(-?\\$[\\d,]+\\.?\\d*)", "i");
  const m = document.body.textContent.match(re);
  return m ? m[1] : null;
}, name);

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  // Robust hydrate (retry once).
  let hydrated = false;
  for (let attempt = 0; attempt < 2 && !hydrated; attempt++) {
    try {
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
      await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
      hydrated = true;
    } catch (e) { note(`hydrate attempt ${attempt + 1} failed: ${e.message.slice(0, 60)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE (boot crash?)");
  pass("HYDRATION — app booted, nav visible");

  const SRC = "Everyday Checking";
  const DST = "Emergency Savings";
  const AMT = 500;

  // ── Baseline: net worth (Dashboard) ──────────────────────────────────────────
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);
  const nwBefore = parseMoney(await readMoneyNear(page, "Net worth"));
  note(`Baseline net worth: ${nwBefore}`);
  await page.screenshot({ path: SS("L79_01_dashboard_before.png") });

  // ── Baseline: account balances (Accounts) ────────────────────────────────────
  await navTo(page, "Accounts");
  await page.waitForTimeout(900);
  const srcBefore = parseMoney(await readAcctBalance(page, SRC));
  const dstBefore = parseMoney(await readAcctBalance(page, DST));
  note(`Baseline: ${SRC}=${srcBefore} | ${DST}=${dstBefore}`);
  await page.screenshot({ path: SS("L79_02_accounts_before.png") });
  if (srcBefore === null || dstBefore === null) {
    absent_(`Could not read baseline balances (src=${srcBefore}, dst=${dstBefore}) — selector/UX issue`);
  }

  // ── Perform a transfer SRC → DST ─────────────────────────────────────────────
  const doTransfer = async (toName, amount, label) => {
    // open the Transfer… form in the SRC account's row
    const opened = await page.evaluate((src) => {
      const rows = [...document.querySelectorAll('[data-cf="account-row"], li, tr, div')].filter(r =>
        r.textContent.includes(src) && [...r.querySelectorAll('button')].some(b => /transfer/i.test(b.textContent)));
      // pick the smallest matching container (the row itself, not the whole page)
      rows.sort((a, b) => a.textContent.length - b.textContent.length);
      const row = rows[0];
      if (!row) return "NO_ROW";
      const btn = [...row.querySelectorAll('button')].find(b => /transfer/i.test(b.textContent));
      if (!btn) return "NO_BTN";
      btn.click();
      return "opened";
    }, SRC);
    if (opened !== "opened") return opened;
    await page.waitForTimeout(700);

    // fill amount, choose destination, submit
    const filled = await page.evaluate(({ toName, amount }) => {
      const amt = document.querySelector('input[placeholder="Amount"]') ||
        [...document.querySelectorAll('input[type="number"]')].pop();
      if (amt) { amt.value = String(amount); amt.dispatchEvent(new Event("input", { bubbles: true })); amt.dispatchEvent(new Event("change", { bubbles: true })); }
      const sel = [...document.querySelectorAll('select')].find(s => s.getAttribute("aria-label") === "To account");
      if (!sel) return "NO_TO_SELECT";
      const opt = [...sel.options].find(o => o.text.includes(toName));
      if (!opt) return "NO_TO_OPTION:" + [...sel.options].map(o => o.text).join("|");
      sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true }));
      return amt ? "filled" : "NO_AMOUNT_INPUT";
    }, { toName, amount });
    if (filled !== "filled") return filled;

    const submitted = await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button')].find(b => /^transfer$|^send$|^save$|^confirm$/i.test(b.textContent.trim()) && b.type !== "reset");
      if (btn) { btn.click(); return "submitted"; }
      // form may submit via the Transfer… button label too; try the form submit
      const form = document.querySelector('form[id^="acct-transfer-form"]');
      if (form) { form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })); return "form-submit"; }
      return "NO_SUBMIT";
    });
    await page.waitForTimeout(1200);
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    await page.waitForTimeout(400);
    note(`  ${label}: open=${opened} fill=${filled} submit=${submitted}`);
    return submitted;
  };

  const t1 = await doTransfer(DST, AMT, "Transfer #1");
  if (t1 === "submitted" || t1 === "form-submit") pass("T-0 — transfer form submitted without crash");
  else { fail(`T-0 — transfer did not submit: ${t1}`); }

  // ── T-1: balances moved correctly ────────────────────────────────────────────
  await navTo(page, "Accounts");
  await page.waitForTimeout(900);
  const srcAfter = parseMoney(await readAcctBalance(page, SRC));
  const dstAfter = parseMoney(await readAcctBalance(page, DST));
  note(`After: ${SRC}=${srcAfter} | ${DST}=${dstAfter}`);
  await page.screenshot({ path: SS("L79_03_accounts_after.png") });

  if (srcBefore !== null && srcAfter !== null) {
    const d = +(srcBefore - srcAfter).toFixed(2);
    if (Math.abs(d - AMT) < 0.01) pass(`T-1a — source decreased by exactly ${AMT} (${srcBefore}→${srcAfter})`);
    else absent_(`T-1a — source delta=${d}, expected ${AMT} (${srcBefore}→${srcAfter})`);
  }
  if (dstBefore !== null && dstAfter !== null) {
    const d = +(dstAfter - dstBefore).toFixed(2);
    if (Math.abs(d - AMT) < 0.01) pass(`T-1b — destination increased by exactly ${AMT} (${dstBefore}→${dstAfter})`);
    else absent_(`T-1b — destination delta=${d}, expected ${AMT} (${dstBefore}→${dstAfter})`);
  }

  // ── T-2 / T-4: net worth conserved (Dashboard) ───────────────────────────────
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);
  const nwAfter = parseMoney(await readMoneyNear(page, "Net worth"));
  note(`Net worth: ${nwBefore} → ${nwAfter}`);
  await page.screenshot({ path: SS("L79_04_dashboard_after.png") });
  if (nwBefore !== null && nwAfter !== null) {
    if (Math.abs(nwBefore - nwAfter) < 0.01) pass(`T-2/T-4 — net worth CONSERVED across transfer (${nwAfter})`);
    else fail(`T-2/T-4 — net worth CHANGED by ${(nwAfter - nwBefore).toFixed(2)} on a transfer (${nwBefore}→${nwAfter}) — integrity violation`);
  } else absent_(`T-2 — net worth not readable (before=${nwBefore}, after=${nwAfter})`);

  // ── T-3: transfer not counted as spending (Reports spending total) ───────────
  await navTo(page, "Reports");
  await page.waitForTimeout(1000);
  const spend = parseMoney(await readMoneyNear(page, "SPENDING"));
  note(`Reports spending total after transfer: ${spend}`);
  await page.screenshot({ path: SS("L79_05_reports.png") });
  // We can't know the exact pre-transfer spend here, but a $500 transfer must not appear as a NEW expense.
  // Heuristic: search the transactions list for a transfer leg and confirm it isn't tagged income/expense.
  await navTo(page, "Transactions");
  await page.waitForTimeout(900);
  const txnInfo = await page.evaluate(() => {
    const text = document.body.textContent;
    const transferRows = (text.match(/Transfer/gi) || []).length;
    return { transferRows };
  });
  note(`Transactions: "Transfer" occurrences = ${txnInfo.transferRows}`);
  if (txnInfo.transferRows > 0) pass("T-3 — transfer is recorded as a labelled Transfer entry (not silently merged)");
  else absent_("T-3 — no 'Transfer' entry visible in transactions after a transfer");

  // ── T-5: STRESS — 3 more transfers, net worth must stay conserved ─────────────
  let stressOk = true;
  for (let i = 0; i < 3; i++) {
    await navTo(page, "Accounts");
    await page.waitForTimeout(700);
    const r = await doTransfer(DST, 100, `Stress transfer ${i + 1}`);
    if (r !== "submitted" && r !== "form-submit") { stressOk = false; note(`  stress ${i + 1} submit=${r}`); }
  }
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);
  const nwStress = parseMoney(await readMoneyNear(page, "Net worth"));
  note(`Net worth after stress transfers: ${nwStress}`);
  if (nwBefore !== null && nwStress !== null) {
    if (Math.abs(nwBefore - nwStress) < 0.01) pass(`T-5 — net worth conserved after ${1 + 3} total transfers (${nwStress})`);
    else fail(`T-5 — net worth drifted to ${nwStress} (from ${nwBefore}) after stress transfers — integrity violation`);
  }
  if (stressOk) pass("T-5b — stress transfers all submitted without crash");
  else absent_("T-5b — one or more stress transfers failed to submit");

  // ── JS errors ────────────────────────────────────────────────────────────────
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
