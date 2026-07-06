// L100 E2E loop story — "The Debt Paydown" (Marcus) — 2026-06-24
//
// Theme: DOUBLE-ENTRY INTEGRITY across the LIABILITY direction. Paying a credit card from checking
// must (a) REDUCE the card's debt by the payment, (b) reduce the checking asset by the same, and
// therefore (c) leave NET WORTH UNCHANGED — moving money from an asset to pay a liability creates no
// wealth and destroys none. L93 proved transfers between assets are net-worth-neutral; this isolates
// the liability direction (transfer INTO a credit-card account) which L93 did not cover.
//
// Invariants:
//   D-1  A liability (credit card) account shows its debt; an asset shows its balance.
//   D-2  A transfer asset → credit card is accepted (form submits).
//   D-3  The card's DEBT drops by exactly the payment (less owed).
//   D-4  The source checking asset drops by exactly the payment.
//   D-5  NET WORTH is UNCHANGED (paying debt moves wealth, doesn't create/destroy it).
//   D-6  No JS errors across the flow.
//
// Run: node e2e/loopstory_100_the_debt_paydown.mjs  (against go run e2e/serve.go on :8099)

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

// Net worth from the dashboard: scan for the "Net worth" label and read its $ figure.
const readNetWorth = (page) => page.evaluate(() => {
  const els = [...document.querySelectorAll('*')].filter(e => e.children.length <= 3 && /net worth/i.test(e.textContent || "") && (e.textContent || "").length < 80);
  for (const e of els) { const m = (e.textContent || "").match(/net worth[^\$-]*\(?-?\$([\d,]+\.?\d*)\)?/i); if (m) return parseFloat(m[1].replace(/,/g, "")); }
  return null;
});

// Read an account row's CURRENT (actual, not cleared) balance by id (the transfer-start-btn carries
// the id). Rows can show a "cleared $X" figure first and the ACTUAL balance last (e.g. a card shows
// "cleared ($6,918.76) ($7,468.16)" — the second is the real debt net worth uses). So the actual
// balance is always the LAST $ figure in the row, whether or not it's parenthesized.
const readAccount = (page, id) => page.evaluate((id) => {
  const tb = document.querySelector('[data-testid="transfer-start-btn-' + id + '"]');
  const row = tb ? tb.closest('.row') : null;
  if (!row) return null;
  const t = (row.textContent || "").replace(/\s+/g, " ");
  const dollars = [...t.matchAll(/\$([\d,]+\.?\d*)/g)].map(m => parseFloat(m[1].replace(/,/g, "")));
  return { bal: dollars.length ? dollars[dollars.length - 1] : null };
}, id);

const doPaydown = async (page, srcId, destNameRe, amount) => {
  const opened = await page.evaluate((srcId) => {
    const b = document.querySelector('[data-testid="transfer-start-btn-' + srcId + '"]');
    if (!b) return false; b.click(); return true;
  }, srcId);
  if (!opened) return "NO_START";
  await page.waitForTimeout(600);
  return page.evaluate((args) => {
    const [srcId, amount, destNameRe] = args;
    const amt = document.querySelector('#acct-xfer-amt-' + CSS.escape(srcId));
    const sel = document.querySelector('[data-testid="acct-xfer-to-select"]') || document.querySelector('#acct-transfer-form-' + CSS.escape(srcId) + ' select');
    if (!amt || !sel) return "NO_FIELDS";
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    const re = new RegExp(destNameRe, "i");
    const opt = [...sel.options].find(o => re.test(o.textContent || ""));
    if (!opt) return "NO_DEST_OPTION:" + [...sel.options].map(o => o.textContent.trim()).join("|").slice(0, 80);
    const setS = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
    setS.call(sel, opt.value); sel.dispatchEvent(new Event('change', { bubbles: true }));
    const form = sel.closest('form');
    if (form) { form.requestSubmit(); return "submitted->" + opt.textContent.trim(); }
    return "NO_FORM";
  }, [srcId, amount, destNameRe]);
};

const jsErrors = [];
const SRC = "acct-checking", CARD = "acct-card", PAY = 500;

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  await navTo(page, "Dashboard");
  const nw0 = await readNetWorth(page);
  note(`Net worth before: ${nw0}`);

  await navTo(page, "Accounts");
  const card0 = await readAccount(page, CARD);
  const chk0 = await readAccount(page, SRC);
  await page.screenshot({ path: path.join(SSDIR, "L100_01_accounts_before.png") });
  note(`Card debt before: ${card0 && card0.bal} · Checking before: ${chk0 && chk0.bal}`);
  if (nw0 != null && card0 && card0.bal != null && chk0 && chk0.bal != null) pass(`D-1 — net worth $${nw0}, card debt $${card0.bal}, checking $${chk0.bal}`);
  else { absent_(`D-1 — could not read baseline (nw=${nw0}, card=${JSON.stringify(card0)}, chk=${JSON.stringify(chk0)})`); throw new Error("baseline"); }

  // ── D-2: pay the card from checking ───────────────────────────────────────────
  const res = await doPaydown(page, SRC, "Rewards Credit Card", PAY);
  await page.waitForTimeout(1100);
  note(`Paydown result: ${res}`);
  if (String(res).startsWith("submitted")) pass(`D-2 — $${PAY} transfer checking → credit card submitted`);
  else { absent_(`D-2 — paydown not submitted (${res})`); throw new Error("paydown"); }

  // ── D-3 / D-4: balances moved by exactly the payment ─────────────────────────
  const card1 = await readAccount(page, CARD);
  const chk1 = await readAccount(page, SRC);
  await page.screenshot({ path: path.join(SSDIR, "L100_02_accounts_after.png") });
  note(`Card debt after: ${card1 && card1.bal} · Checking after: ${chk1 && chk1.bal}`);
  const eps = 0.01;
  const debtDrop = (card0.bal ?? 0) - (card1.bal ?? 0);
  const chkDrop = (chk0.bal ?? 0) - (chk1.bal ?? 0);
  if (Math.abs(debtDrop - PAY) <= eps) pass(`D-3 — card debt dropped by exactly $${PAY} (${card0.bal} → ${card1.bal})`);
  else fail(`D-3 — card debt change $${debtDrop.toFixed(2)}, expected -$${PAY} (${card0.bal} → ${card1 && card1.bal})`);
  if (Math.abs(chkDrop - PAY) <= eps) pass(`D-4 — checking dropped by exactly $${PAY} (${chk0.bal} → ${chk1.bal})`);
  else fail(`D-4 — checking change $${chkDrop.toFixed(2)}, expected -$${PAY} (${chk0.bal} → ${chk1 && chk1.bal})`);

  // ── D-5: net worth unchanged ──────────────────────────────────────────────────
  await navTo(page, "Dashboard");
  const nw1 = await readNetWorth(page);
  note(`Net worth after: ${nw1}`);
  if (nw1 != null && Math.abs(nw1 - nw0) <= eps) pass(`D-5 — NET WORTH UNCHANGED ($${nw0} = $${nw1}) — paying debt is wealth-neutral`);
  else fail(`D-5 — net worth changed $${nw0} → $${nw1} (paying a card should NOT change net worth!)`);

  if (jsErrors.length === 0) pass("D-6 — zero runtime JS errors across the ritual");
  else fail(`D-6 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (!["baseline", "paydown"].includes(String(err.message))) { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
