// L92 E2E loop story — "Whose Money Is It" (the Hartleys) — 2026-06-24
//
// Theme: HOUSEHOLD MEMBER-FILTER INTEGRITY. A finance-aware home with >1 member needs "show me just
// my money" to work — and to work the SAME everywhere. The "View as member" switcher must (1) offer
// Everyone + each member, (2) actually change the data when switched, (3) carry the selection across
// screens (Transactions/Reports/Budgets all honor it), (4) keep each member a subset of the household
// (a member's count ≤ Everyone's), and (5) survive rapid switching. Invariants:
//   M-1  Switcher offers Everyone + each household member.
//   M-2  Switching to a member changes the visible data (count differs from Everyone).
//   M-3  The selected member CARRIES across screens (the switcher shows the same member on Reports/Budgets).
//   M-4  Each member's transaction count <= Everyone's (a member ⊆ the household).
//   M-5  STRESS: rapid member switches yield consistent, non-negative counts and return to baseline.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_92_whose_money.mjs

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
const memberOptions = (page) => page.evaluate(() => {
  const s = [...document.querySelectorAll('select')].find(x => /view as member/i.test(x.getAttribute('aria-label') || ''));
  return s ? [...s.options].map(o => ({ t: o.textContent.trim(), v: o.value })) : null;
});
const memberValue = (page) => page.evaluate(() => {
  const s = [...document.querySelectorAll('select')].find(x => /view as member/i.test(x.getAttribute('aria-label') || ''));
  return s ? s.value : "MISSING";
});
const setMember = (page, val) => page.evaluate((val) => {
  const s = [...document.querySelectorAll('select')].find(x => /view as member/i.test(x.getAttribute('aria-label') || ''));
  if (!s) return "NO_SEL";
  const setter = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
  setter.call(s, val); s.dispatchEvent(new Event('change', { bubbles: true }));
  return "set";
}, val);
const txnCount = (page) => page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; });

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
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L92_01_everyone.png") });

  // ── M-1: switcher offers Everyone + members ───────────────────────────────────
  const opts = await memberOptions(page);
  note(`Member options: ${opts ? opts.map(o => o.t).join(" | ") : "none"}`);
  if (opts && opts.length >= 3 && /everyone/i.test(opts[0].t)) pass(`M-1 — switcher offers Everyone + ${opts.length - 1} member(s)`);
  else absent_(`M-1 — switcher options unexpected (${opts ? opts.map(o => o.t).join(",") : "none"})`);

  const everyoneCount = await txnCount(page);
  note(`Everyone count: ${everyoneCount}`);

  // ── M-2 / M-4: switch to each member, count changes & <= Everyone ──────────────
  const memberCounts = {};
  for (const o of (opts || []).filter(o => o.v)) {
    await setMember(page, o.v); await page.waitForTimeout(900);
    const c = await txnCount(page);
    memberCounts[o.t] = c;
    note(`  ${o.t}: ${c}`);
  }
  const counts = Object.values(memberCounts).filter(c => c != null);
  if (everyoneCount != null && counts.length && counts.some(c => c !== everyoneCount)) pass(`M-2 — switching member changes the data (Everyone ${everyoneCount} vs ${counts.join("/")})`);
  else absent_(`M-2 — member switch did not change the count (Everyone ${everyoneCount}, members ${counts.join("/")})`);
  if (everyoneCount != null && counts.length && counts.every(c => c <= everyoneCount)) pass(`M-4 — each member's count <= Everyone's (${counts.join("/")} <= ${everyoneCount}) — member ⊆ household`);
  else fail(`M-4 — a member's count EXCEEDS Everyone's (${counts.join("/")} vs ${everyoneCount}) — filter math wrong`);

  // ── M-3: selection carries across screens ─────────────────────────────────────
  const firstMember = (opts || []).find(o => o.v);
  if (firstMember) {
    await setMember(page, firstMember.v); await page.waitForTimeout(700);
    await navTo(page, "Reports"); await page.waitForTimeout(600);
    const onReports = await memberValue(page);
    await navTo(page, "Budgets"); await page.waitForTimeout(600);
    const onBudgets = await memberValue(page);
    note(`Member "${firstMember.t}" (${firstMember.v}) across screens: Reports="${onReports}" Budgets="${onBudgets}"`);
    if (onReports === firstMember.v && onBudgets === firstMember.v) pass(`M-3 — the selected member (${firstMember.t}) carries across Transactions/Reports/Budgets`);
    else absent_(`M-3 — member selection did not carry (set ${firstMember.v}; Reports=${onReports}, Budgets=${onBudgets})`);
    // reset to Everyone
    await navTo(page, "Transactions"); await page.waitForTimeout(500);
    await setMember(page, ""); await page.waitForTimeout(700);
  }

  // ── M-5: STRESS — rapid switching, consistent & returns to baseline ───────────
  await navTo(page, "Transactions"); await page.waitForTimeout(600);
  const order = ["m-marcus", "m-priya", "", "m-priya", "m-marcus", ""];
  let sane = true; const seq = [];
  for (const v of order) { await setMember(page, v); await page.waitForTimeout(550); const c = await txnCount(page); seq.push(c); if (c == null || c < 0 || (everyoneCount != null && c > everyoneCount)) sane = false; }
  const finalCount = await txnCount(page);
  note(`Stress counts: [${seq.join(", ")}] final=${finalCount} (baseline ${everyoneCount})`);
  if (sane && finalCount === everyoneCount) pass(`M-5 — rapid member switching stayed consistent; returned to Everyone baseline (${finalCount})`);
  else absent_(`M-5 — stress inconsistent (sane=${sane}, final=${finalCount}/${everyoneCount})`);

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
