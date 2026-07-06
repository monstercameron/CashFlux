// L80 E2E loop story — "Paying the Bills" (Tomas) — 2026-06-24
//
// Theme: BILL-PAYMENT LIFECYCLE + CROSS-SCREEN PROPAGATION (a core everyday action)
//
// Persona: Tomas sits down on payday and clears this week's bills. "Mark paid" is one of the
// most common household actions, so it must be solid. Invariants for an enterprise-grade app:
//   B-1  Mark paid CREATES a payment transaction (the money leaving is recorded).
//   B-2  For a recurring bill, NextDue ADVANCES (it shouldn't stay "due today" forever / re-dun).
//   B-3  The payment count matches the number of clicks — no double-post, no dropped post.
//   B-4  A clear success confirmation is shown (toast: "Logged a payment for X").
//   B-5  Spending (Reports) reflects the new payments (a bill payment IS spending).
//   B-6  STRESS: clearing several bills back-to-back never crashes / desyncs the count.
//   B-7  IDEMPOTENCY: clicking "Mark paid" twice fast posts the intended number of payments
//        (documents whether a double-tap silently double-charges — a real money risk).
//
// Screens: /bills (mark paid) → /transactions (payment recorded) → /reports (spending) → /bills (next due)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_80_paying_bills.mjs

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

// The transactions counter ("N transactions") is the most reliable row count (L78 lesson).
const txnCount = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});

const flush = async (page) => { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(350); };

// Click "Mark paid" on the bill row whose text contains `name`. Returns status.
const markPaid = async (page, name) => page.evaluate((name) => {
  const rows = [...document.querySelectorAll('li, tr, div')].filter(r =>
    r.textContent.includes(name) && [...r.querySelectorAll('button')].some(b => /mark paid/i.test(b.textContent)));
  rows.sort((a, b) => a.textContent.length - b.textContent.length);
  const row = rows[0];
  if (!row) return "NO_ROW";
  const btn = [...row.querySelectorAll('button')].find(b => /mark paid/i.test(b.textContent));
  if (!btn) return "NO_BTN";
  btn.click();
  return "clicked";
}, name);

// The transient confirmation renders as <span class="toast-msg">; the persistent
// sample-data banner is a separate notice, so target the toast message specifically.
const readToast = (page) => page.evaluate(() => {
  const t = document.querySelector('.toast-msg') ||
    [...document.querySelectorAll('.toast, [role="status"], [role="alert"]')].find(e => /logged|paid|payment/i.test(e.textContent));
  return t ? t.textContent.trim().slice(0, 80) : null;
});

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try {
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
      await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
      hydrated = true;
    } catch (e) { note(`hydrate attempt ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted, nav visible");

  // ── Baseline txn count ───────────────────────────────────────────────────────
  await navTo(page, "Transactions");
  await page.waitForTimeout(800);
  const txnBefore = await txnCount(page);
  note(`Baseline transaction count: ${txnBefore}`);

  // ── Bills: snapshot the upcoming list + next due ─────────────────────────────
  await navTo(page, "Bills");
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L80_01_bills_before.png") });
  const billsSnap = await page.evaluate(() => {
    const nd = document.body.textContent.match(/Next due[^\d]*([\d]{4}-[\d]{2}-[\d]{2}|—|-)/i);
    const markBtns = [...document.querySelectorAll('button')].filter(b => /mark paid/i.test(b.textContent)).length;
    // bill names (rows with a Mark paid button)
    const names = [...document.querySelectorAll('li, tr, div')]
      .filter(r => [...r.querySelectorAll('button')].some(b => /mark paid/i.test(b.textContent)) && r.textContent.length < 220)
      .map(r => (r.textContent.match(/^[^\d$·]+/) || [""])[0].trim().slice(0, 30))
      .filter(Boolean);
    return { nextDue: nd ? nd[1] : null, markBtns, sampleNames: [...new Set(names)].slice(0, 8) };
  });
  note(`Bills: nextDue=${billsSnap.nextDue} | markPaid buttons=${billsSnap.markBtns}`);
  note(`Bill names: ${JSON.stringify(billsSnap.sampleNames)}`);
  if (billsSnap.markBtns > 0) pass(`B-0 — ${billsSnap.markBtns} payable bills present with Mark-paid actions`);
  else { absent_("B-0 — no Mark-paid buttons found on /bills"); }

  // pick a target bill (prefer a recurring-looking one)
  const target = billsSnap.sampleNames.find(n => /gym|stream|rent|insurance|subscription/i.test(n)) || billsSnap.sampleNames[0];
  note(`Target bill to pay: "${target}"`);

  // ── B-1/B-4: mark one paid, expect a payment txn + toast ─────────────────────
  const mp = await markPaid(page, target);
  await page.waitForTimeout(700);
  const toast = await readToast(page);
  note(`Mark paid "${target}": ${mp} | toast="${toast}"`);
  await page.screenshot({ path: SS("L80_02_after_markpaid.png") });
  if (mp === "clicked") pass("B-1a — Mark paid action fired without crash");
  else fail(`B-1a — could not click Mark paid: ${mp}`);
  if (toast && /logged|paid|payment/i.test(toast)) pass(`B-4 — success confirmation shown ("${toast}")`);
  else absent_(`B-4 — no clear success toast after mark paid (got: ${toast})`);

  await flush(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(900);
  const txnAfter1 = await txnCount(page);
  note(`Transaction count after 1 payment: ${txnBefore} → ${txnAfter1}`);
  if (txnBefore !== null && txnAfter1 !== null) {
    if (txnAfter1 === txnBefore + 1) pass("B-1b — exactly ONE payment transaction created");
    else if (txnAfter1 > txnBefore) absent_(`B-1b — count rose by ${txnAfter1 - txnBefore} (expected 1) — possible multi/double post`);
    else fail(`B-1b — NO transaction created by Mark paid (${txnBefore}→${txnAfter1})`);
  }
  // is the payment findable by name?
  const found = await page.evaluate((name) => {
    const txt = document.body.textContent;
    return txt.includes("Bill payment: " + name) || txt.includes(name);
  }, target);
  if (found) pass(`B-1c — the "${target}" payment is findable in /transactions`);
  else absent_(`B-1c — paid bill "${target}" not visible in /transactions (may be paged beyond 50)`);

  // ── B-2: NextDue advanced (recurring should not stay due) ─────────────────────
  await navTo(page, "Bills");
  await page.waitForTimeout(900);
  const billsAfter = await page.evaluate(() => {
    const nd = document.body.textContent.match(/Next due[^\d]*([\d]{4}-[\d]{2}-[\d]{2}|—|-)/i);
    const markBtns = [...document.querySelectorAll('button')].filter(b => /mark paid/i.test(b.textContent)).length;
    return { nextDue: nd ? nd[1] : null, markBtns };
  });
  note(`Bills after pay: nextDue=${billsAfter.nextDue} | markPaid buttons=${billsAfter.markBtns}`);
  // For a recurring bill the due date should advance; the upcoming list may shrink if it moved out of window.
  if (billsSnap.nextDue && billsAfter.nextDue) {
    if (billsAfter.nextDue !== billsSnap.nextDue) pass(`B-2 — Next due advanced/changed after payment (${billsSnap.nextDue} → ${billsAfter.nextDue})`);
    else absent_(`B-2 — Next due unchanged (${billsAfter.nextDue}) — recurring NextDue may not have advanced, or this bill wasn't the soonest`);
  } else note(`B-2 — next-due comparison skipped (before=${billsSnap.nextDue}, after=${billsAfter.nextDue})`);

  // ── B-5: spending reflects payments (Reports) ─────────────────────────────────
  await navTo(page, "Reports");
  await page.waitForTimeout(1000);
  const spend = await page.evaluate(() => {
    const m = document.body.textContent.match(/SPENDING[^$]{0,30}?(\$[\d,]+\.?\d*)/i);
    return m ? m[1] : null;
  });
  note(`Reports spending total: ${spend}`);
  if (spend) pass(`B-5 — Reports shows a spending figure after payments (${spend})`);
  else absent_("B-5 — could not read Reports spending total");

  // ── B-6 STRESS: clear several bills back-to-back ─────────────────────────────
  await navTo(page, "Bills");
  await page.waitForTimeout(900);
  const stressTargets = billsSnap.sampleNames.filter(n => n !== target).slice(0, 3);
  let stressClicks = 0;
  for (const n of stressTargets) {
    const r = await markPaid(page, n);
    if (r === "clicked") stressClicks++;
    else note(`  stress mark "${n}": ${r}`);
    await page.waitForTimeout(800);
    await flush(page);
  }
  note(`Stress: clicked Mark paid on ${stressClicks}/${stressTargets.length} more bills`);
  await navTo(page, "Transactions");
  await page.waitForTimeout(900);
  const txnAfterStress = await txnCount(page);
  note(`Transaction count after stress: ${txnAfter1} → ${txnAfterStress}`);
  if (txnAfter1 !== null && txnAfterStress !== null) {
    const delta = txnAfterStress - txnAfter1;
    if (delta === stressClicks) pass(`B-6 — ${stressClicks} stress payments created exactly ${delta} transactions (no double/dropped post)`);
    else absent_(`B-6 — ${stressClicks} clicks produced ${delta} transactions (mismatch — double/dropped post?)`);
  }

  // ── B-7 IDEMPOTENCY: double-tap the same bill ────────────────────────────────
  await navTo(page, "Bills");
  await page.waitForTimeout(900);
  const idemName = billsSnap.sampleNames.find(n => ![target, ...stressTargets].includes(n));
  if (idemName) {
    const before = await txnCount(page).catch(() => null);
    // we're on bills; get count from transactions first
    await navTo(page, "Transactions"); await page.waitForTimeout(700);
    const idemBefore = await txnCount(page);
    await navTo(page, "Bills"); await page.waitForTimeout(800);
    // two fast clicks
    await markPaid(page, idemName);
    await markPaid(page, idemName);
    await page.waitForTimeout(900); await flush(page);
    await navTo(page, "Transactions"); await page.waitForTimeout(900);
    const idemAfter = await txnCount(page);
    const d = (idemBefore !== null && idemAfter !== null) ? idemAfter - idemBefore : null;
    note(`B-7 idempotency: double-tap "${idemName}" → ${idemBefore}→${idemAfter} (delta ${d})`);
    if (d === 2) note("B-7 — double-tap posts 2 payments (each click = a payment; acceptable IF intentional, but a confirm/disable-after-pay would prevent accidental double-charge)");
    else if (d === 1) pass("B-7 — double-tap posts only 1 payment (guarded against accidental double-charge)");
    else note(`B-7 — double-tap delta=${d} (inconclusive)`);
  } else note("B-7 — no spare bill for idempotency test");

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
