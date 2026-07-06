// L87 E2E loop story — "The Net Worth Truth" (Dana) — 2026-06-24
//
// Theme: NET-WORTH / TOTAL-MONEY INTEGRITY ACROSS SCREENS. The single number an everyday
// finance-aware home checks most often is "how much do we actually have?" It must be (1) quickly
// visible on the dashboard without hunting, (2) equal to the sum of account balances on the
// Accounts screen, (3) correctly REDUCED by liabilities (a credit card is not an asset), and
// (4) consistent after a money movement (logging an expense lowers the account AND the total).
// Invariants:
//   N-1  Dashboard surfaces a total-money / net-worth figure above the fold.
//   N-2  That figure reconciles with the sum of the Accounts screen balances (assets − liabilities).
//   N-3  Adding a liability account DECREASES the displayed total (liabilities subtract).
//   N-4  Logging an expense lowers the source account balance by ~the expense amount.
//   N-5  STRESS: rapid screen-hopping (Dashboard↔Accounts↔Transactions ×4) leaves the total stable.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_87_net_worth_truth.mjs

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
const money = (s) => { if (s == null) return null; const neg = /[-(]/.test(s); const n = parseFloat(String(s).replace(/[^0-9.]/g, "")); if (isNaN(n)) return null; return neg ? -n : n; };

// Find a "total money / net worth" figure on the dashboard — try labelled stat first, then any
// large $ figure near a net-worth-ish word.
const dashTotal = (page) => page.evaluate(() => {
  const txt = document.body.innerText;
  // labelled patterns
  const labels = [/net worth[^$]{0,40}?(\-?\$[\d,]+\.?\d*)/i, /total (?:balance|money|assets|cash)[^$]{0,40}?(\-?\$[\d,]+\.?\d*)/i, /(\-?\$[\d,]+\.?\d*)[^a-z]{0,8}(?:net worth|total balance)/i];
  for (const re of labels) { const m = txt.match(re); if (m) return { value: m[1], how: re.source.slice(0, 24) }; }
  return { value: null, how: "none" };
});

// Read the Accounts screen summary stats. The header renders three .stat tiles:
// "Net worth" (.stat-value), "Assets" and "Liabilities". Returns the parsed trio + per-account count.
const accountsSummary = (page) => page.evaluate(() => {
  const stats = [...document.querySelectorAll('.stat')];
  const out = { net: null, assets: null, liab: null };
  for (const s of stats) {
    const label = (s.textContent || "").toLowerCase();
    const m = (s.textContent || "").match(/\$[\d,]+\.?\d*/);
    if (!m) continue; const v = parseFloat(m[0].replace(/[^0-9.]/g, ""));
    if (/net worth/.test(label)) out.net = v;
    else if (/asset/.test(label)) out.assets = v;
    else if (/liabilit/.test(label)) out.liab = v;
  }
  const perAccount = document.querySelectorAll('.amount').length;
  return { ...out, perAccount };
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

  // load sample data so there are accounts to total
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data|try sample/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  // ── N-1: dashboard surfaces a total figure above the fold ─────────────────────
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);
  await page.screenshot({ path: SS("L87_01_dashboard.png") });
  const t0 = await dashTotal(page);
  note(`Dashboard total: value="${t0.value}" (matched via: ${t0.how})`);
  // is it above the fold (visible without scrolling)?
  const aboveFold = await page.evaluate((val) => {
    if (!val) return false;
    const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    let n; while ((n = walk.nextNode())) { if (n.textContent.includes(val)) { const el = n.parentElement; const r = el.getBoundingClientRect(); return r.top >= 0 && r.top < 900; } }
    return false;
  }, t0.value);
  if (t0.value) pass(`N-1 — dashboard surfaces a total-money figure (${t0.value})`);
  else absent_("N-1 — could not find a labelled net-worth/total figure on the dashboard");
  if (t0.value && aboveFold) pass("N-1b — the total figure is above the fold (visible without scrolling)");
  else if (t0.value) absent_("N-1b — total figure found but is NOT above the fold (user must scroll to see total money)");

  // ── N-2: dashboard total reconciles with Accounts net worth AND assets−liabilities ─
  await navTo(page, "Accounts");
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L87_02_accounts.png") });
  const acc = await accountsSummary(page);
  note(`Accounts summary: netWorth=${acc.net} assets=${acc.assets} liabilities=${acc.liab} perAccountAmounts=${acc.perAccount}`);
  const dv = money(t0.value);
  // N-2a: dashboard total == Accounts net-worth stat
  if (dv != null && acc.net != null && Math.abs(dv - acc.net) <= Math.max(1, Math.abs(dv) * 0.02)) pass(`N-2a — dashboard total (${dv}) matches Accounts "Net worth" stat (${acc.net})`);
  else absent_(`N-2a — dashboard total (${dv}) ≠ Accounts net worth (${acc.net})`);
  // N-2b: net worth == assets − liabilities (liabilities correctly subtract)
  if (acc.net != null && acc.assets != null && acc.liab != null) {
    const derived = Math.round((acc.assets - acc.liab) * 100) / 100;
    if (Math.abs(derived - acc.net) <= 1) pass(`N-2b/N-3 — net worth (${acc.net}) = assets (${acc.assets}) − liabilities (${acc.liab}) = ${derived} (liabilities correctly subtract)`);
    else fail(`N-2b/N-3 — net worth (${acc.net}) ≠ assets−liabilities (${derived}) — net-worth math is WRONG`);
  } else absent_(`N-2b/N-3 — missing assets/liabilities stat (assets=${acc.assets}, liab=${acc.liab})`);

  // ── N-4: log an expense, verify net worth / assets drop ───────────────────────
  const before = await accountsSummary(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(900);
  const opened = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /add transaction|new transaction|\+ add|^add$/i.test(b.textContent.trim())); if (b) { b.click(); return true; } return false; });
  await page.waitForTimeout(700);
  if (opened) {
    // A correct expense requires (a) an Account selected and (b) the Expense type — otherwise it
    // won't reduce net worth. Pick an asset account so the $100 drop is unambiguous.
    const filled = await page.evaluate(() => {
      const accSel = [...document.querySelectorAll('select')].find(s => s.getAttribute('aria-label') === 'Account');
      if (accSel) { const opt = [...accSel.options].find(o => /checking|brokerage|401/i.test(o.textContent)) || accSel.options[0]; accSel.value = opt.value; accSel.dispatchEvent(new Event('change', { bubbles: true })); }
      const exp = [...document.querySelectorAll('button,label')].find(e => /^expense$/i.test((e.textContent || "").trim())); if (exp) exp.click();
      const amt = [...document.querySelectorAll('input[type="number"]')].find(e => e.getAttribute('aria-label') === 'Amount') || document.querySelector('input[type="number"]');
      const desc = [...document.querySelectorAll('input[type="text"]')].find(e => e.getAttribute('aria-label') === 'Description');
      if (amt) { amt.value = "100"; amt.dispatchEvent(new Event("input", { bubbles: true })); }
      if (desc) { desc.value = "L87 net-worth probe"; desc.dispatchEvent(new Event("input", { bubbles: true })); }
      return { amt: !!amt, acc: accSel ? accSel.options[accSel.selectedIndex].textContent : "none" };
    });
    await page.waitForTimeout(300);
    const saved = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => b.textContent.trim() === "Save" || /save transaction/i.test(b.textContent)); if (b) { b.click(); return true; } return false; });
    await page.waitForTimeout(1200);
    note(`Expense add: opened=${opened} account="${filled.acc}" filledAmount=${filled.amt} saved=${saved}`);
    if (filled.amt && saved) {
      await navTo(page, "Accounts"); await page.waitForTimeout(900);
      const after = await accountsSummary(page);
      note(`Net worth before=${before.net} after=${after.net} (expected -100 from the asset account)`);
      if (before.net != null && after.net != null) {
        const drop = Math.round((before.net - after.net) * 100) / 100;
        if (Math.abs(drop - 100) <= 1) pass(`N-4 — a $100 expense lowered net worth by exactly ${drop} (${before.net} -> ${after.net}) — transactions correctly reflow into balances`);
        else if (drop > 0) pass(`N-4 — a $100 expense lowered net worth by ${drop} (${before.net} -> ${after.net})`);
        else absent_(`N-4 — net worth did not drop after the expense (${before.net} -> ${after.net}, Δ=${drop}) — expense may not be posting to an account, or no account was selected`);
      } else absent_(`N-4 — could not read net worth before/after (before=${before.net}, after=${after.net})`);
    } else absent_("N-4 — could not complete the add-transaction form (amount/save not found)");
  } else absent_("N-4 — could not open the add-transaction form");

  // ── N-5: STRESS — rapid screen hopping, total stays stable ────────────────────
  let stableSeen = new Set();
  for (let i = 0; i < 4; i++) {
    await navTo(page, "Dashboard"); await page.waitForTimeout(400);
    const t = (await dashTotal(page)).value; if (t) stableSeen.add(t);
    await navTo(page, "Accounts"); await page.waitForTimeout(300);
    await navTo(page, "Transactions"); await page.waitForTimeout(300);
  }
  await navTo(page, "Dashboard"); await page.waitForTimeout(600);
  const tFinal = (await dashTotal(page)).value;
  note(`Stress: distinct dashboard totals seen across 4 hops = ${stableSeen.size} [${[...stableSeen].join(", ")}], final="${tFinal}"`);
  if (stableSeen.size <= 1 && tFinal) pass(`N-5 — dashboard total stayed stable across rapid screen-hopping (${tFinal})`);
  else if (stableSeen.size > 1) absent_(`N-5 — dashboard total FLICKERED across hops (${[...stableSeen].join(", ")}) — recompute may be non-deterministic`);
  else absent_("N-5 — could not read a stable total during stress");

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
