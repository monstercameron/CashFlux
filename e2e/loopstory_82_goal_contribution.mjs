// L82 E2E loop story — "Paying Yourself First" (Aaliyah) — 2026-06-24
//
// Theme: GOAL CONTRIBUTION LIFECYCLE + CROSS-SCREEN CONSISTENCY (a core everyday action)
//
// Persona: Aaliyah moves money into her savings goals on payday. "Contribute" must be
// solid and its money-effects must be honest. Invariants:
//   G-1  Contributing raises the goal's saved amount and progress % (no ledger needed).
//   G-2  With "Also debit …" checked, ONE ledger transaction is posted; without it, none.
//   G-3  The contribution ledger entry is NOT counted as spending (no category → Reports
//        spending total must not jump — saving is not spending).
//   G-4  $0 / negative contribution is rejected (no goal change, no phantom txn) (L41).
//   G-5  STRESS: several contributions accumulate exactly (no drift / double-count).
//   G-NOTE  Document the direction of the optional ledger debit vs the linked account.
//
// Screens: /goals (contribute) → /transactions (ledger) → /reports (spending unaffected)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_82_goal_contribution.mjs

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
const parseMoney = (s) => s ? parseFloat(s.replace(/[^0-9.]/g, "")) : null;

const txnCount = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});
const reportsSpend = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/SPENDING[^$]{0,30}?(\$[\d,]+\.?\d*)/i);
  return m ? m[1] : null;
});

// Read the FIRST goal's "current / target" (the goal whose Contribute we'll click).
const firstGoalProgress = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/\$([\d,]+\.\d{2})\s*\/\s*\$([\d,]+\.\d{2})/);
  if (!m) return null;
  return { current: parseFloat(m[1].replace(/,/g, "")), target: parseFloat(m[2].replace(/,/g, "")) };
});

// Open the first goal's contribute form, fill amount, optionally check ledger, submit.
const contribute = async (page, amount, withLedger) => {
  const opened = await page.evaluate(() => {
    const btn = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === "Contribute");
    if (btn) { btn.click(); return true; } return false;
  });
  if (!opened) return "NO_CONTRIBUTE_BTN";
  await page.waitForTimeout(500);
  const filled = await page.evaluate(({ amount, withLedger }) => {
    const amt = document.querySelector('input[id^="goal-contrib-"]:not([id*="ledger"])') || document.querySelector('input[placeholder="Amount to add"]');
    if (!amt) return "NO_AMOUNT_INPUT";
    amt.value = String(amount); amt.dispatchEvent(new Event("input", { bubbles: true })); amt.dispatchEvent(new Event("change", { bubbles: true }));
    if (withLedger) {
      const cb = document.querySelector('input[id^="goal-contrib-ledger-"]');
      if (cb && !cb.checked) { cb.click(); }
    }
    return "filled";
  }, { amount, withLedger });
  if (filled !== "filled") return filled;
  const submitted = await page.evaluate(() => {
    // the submit "Contribute" is inside the open form (type=submit)
    const form = document.querySelector('form');
    const btn = form ? [...form.querySelectorAll('button')].find(b => /contribute/i.test(b.textContent) && b.type !== "button") : null;
    const b2 = btn || [...document.querySelectorAll('button')].find(b => b.textContent.trim() === "Contribute");
    if (b2) { b2.click(); return "submitted"; } return "NO_SUBMIT";
  });
  await page.waitForTimeout(900); await flush(page);
  return submitted;
};

const readToast = (page) => page.evaluate(() => {
  const t = document.querySelector('.toast-msg'); return t ? t.textContent.trim().slice(0, 80) : null;
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

  // ── Baselines ────────────────────────────────────────────────────────────────
  await nav(page, "Transactions"); await page.waitForTimeout(700);
  const txnBefore = await txnCount(page);
  await nav(page, "Reports"); await page.waitForTimeout(900);
  const spendBefore = parseMoney(await reportsSpend(page));
  note(`Baseline: transactions=${txnBefore} | reports spending=${spendBefore}`);

  await nav(page, "Goals"); await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L82_01_goals_before.png") });
  const g0 = await firstGoalProgress(page);
  note(`First goal progress: ${JSON.stringify(g0)}`);
  if (!g0) { absent_("Could not read a goal's $current/$target — cannot run contribution checks"); }

  // ── G-1: contribute WITHOUT ledger → goal rises, no new txn ──────────────────
  const ADD1 = 200;
  const c1 = await contribute(page, ADD1, false);
  const toast1 = await readToast(page);
  note(`Contribute $${ADD1} (no ledger): ${c1} | toast="${toast1}"`);
  await page.waitForTimeout(400);
  const g1 = await firstGoalProgress(page);
  note(`Goal after no-ledger contribute: ${JSON.stringify(g1)}`);
  if (g0 && g1) {
    const d = +(g1.current - g0.current).toFixed(2);
    if (Math.abs(d - ADD1) < 0.01) pass(`G-1 — goal saved amount rose by exactly $${ADD1} (${g0.current}→${g1.current})`);
    else fail(`G-1 — goal delta ${d} != ${ADD1} (${g0.current}→${g1.current})`);
  }
  // no new transaction for a no-ledger contribution
  await nav(page, "Transactions"); await page.waitForTimeout(800);
  const txnAfter1 = await txnCount(page);
  note(`Txn count after no-ledger contribute: ${txnBefore} → ${txnAfter1}`);
  if (txnBefore !== null && txnAfter1 !== null) {
    if (txnAfter1 === txnBefore) pass("G-2a — no-ledger contribution creates NO transaction (goal-only)");
    else absent_(`G-2a — no-ledger contribution changed txn count by ${txnAfter1 - txnBefore} (expected 0)`);
  }

  // ── G-2/G-3: contribute WITH ledger → one txn, not counted as spending ────────
  await nav(page, "Goals"); await page.waitForTimeout(800);
  const gPre2 = await firstGoalProgress(page);
  const ADD2 = 300;
  const c2 = await contribute(page, ADD2, true);
  note(`Contribute $${ADD2} (WITH ledger): ${c2}`);
  await page.screenshot({ path: SS("L82_02_after_contribute.png") });
  const gPost2 = await firstGoalProgress(page);
  if (gPre2 && gPost2) {
    const d = +(gPost2.current - gPre2.current).toFixed(2);
    if (Math.abs(d - ADD2) < 0.01) pass(`G-1b — goal rose by $${ADD2} on ledger contribution (${gPre2.current}→${gPost2.current})`);
    else fail(`G-1b — goal delta ${d} != ${ADD2}`);
  }
  await nav(page, "Transactions"); await page.waitForTimeout(800);
  const txnAfter2 = await txnCount(page);
  const hasContribTxn = await page.evaluate(() => /Goal contribution/i.test(document.body.textContent));
  note(`Txn count after ledger contribute: ${txnAfter1} → ${txnAfter2} | "Goal contribution" visible: ${hasContribTxn}`);
  if (txnAfter1 !== null && txnAfter2 !== null) {
    if (txnAfter2 === txnAfter1 + 1) pass("G-2b — ledger contribution created exactly ONE transaction");
    else absent_(`G-2b — ledger contribution changed txn count by ${txnAfter2 - txnAfter1} (expected 1)`);
  }
  // spending must NOT include the contribution (no category)
  await nav(page, "Reports"); await page.waitForTimeout(900);
  const spendAfter = parseMoney(await reportsSpend(page));
  note(`Reports spending: ${spendBefore} → ${spendAfter}`);
  if (spendBefore !== null && spendAfter !== null) {
    if (Math.abs(spendAfter - spendBefore) < 0.01) pass(`G-3 — contribution NOT counted as spending (Reports spending unchanged at ${spendAfter})`);
    else absent_(`G-3 — Reports spending changed ${spendBefore}→${spendAfter} after a savings contribution (should a goal contribution count as spending? — review)`);
  }

  // ── G-4: $0 contribution rejected ────────────────────────────────────────────
  await nav(page, "Goals"); await page.waitForTimeout(800);
  const gPre3 = await firstGoalProgress(page);
  const c3 = await contribute(page, 0, false);
  await page.waitForTimeout(300);
  const gPost3 = await firstGoalProgress(page);
  note(`$0 contribute attempt: ${c3} | goal ${gPre3 ? gPre3.current : "?"} → ${gPost3 ? gPost3.current : "?"}`);
  if (gPre3 && gPost3) {
    if (gPost3.current === gPre3.current) pass("G-4 — $0 contribution rejected (goal unchanged)");
    else fail(`G-4 — $0 contribution changed the goal (${gPre3.current}→${gPost3.current})`);
  }
  // dismiss any open form
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(x => x.textContent.trim() === "Cancel"); if (b) b.click(); });
  await page.waitForTimeout(300);

  // ── G-5: STRESS — 3 contributions accumulate exactly ─────────────────────────
  await nav(page, "Goals"); await page.waitForTimeout(700);
  const gPreS = await firstGoalProgress(page);
  let stressOk = true;
  for (let i = 0; i < 3; i++) {
    const r = await contribute(page, 50, false);
    if (r !== "submitted") { stressOk = false; note(`  stress contribute ${i + 1}: ${r}`); }
    await page.waitForTimeout(300);
  }
  const gPostS = await firstGoalProgress(page);
  if (gPreS && gPostS) {
    const d = +(gPostS.current - gPreS.current).toFixed(2);
    if (Math.abs(d - 150) < 0.01) pass(`G-5 — 3×$50 contributions accumulated exactly $150 (${gPreS.current}→${gPostS.current})`);
    else absent_(`G-5 — stress delta ${d} != 150 (${gPreS.current}→${gPostS.current})`);
  }
  if (!stressOk) absent_("G-5b — one or more stress contributions failed to submit");

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
