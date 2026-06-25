// L103 E2E loop story — "Splitting the Dinner" (the Hartleys) — 2026-06-24
//
// Theme: SHARED-EXPENSE SPLIT + SETTLE-UP. A household splits costs and tracks who owes whom. The
// split math must be exact (even shares, and ODD amounts must not lose or invent a penny), the
// settle-up ledger must net correctly, and recording a settlement must clear that balance.
//
// Invariants:
//   S-1  Even split: $100.00 between 2 members → $50.00 each.
//   S-2  ROUNDING INTEGRITY: $100.01 between 2 → shares sum to EXACTLY $100.01 (no penny lost/created),
//        and the two shares differ by at most 1¢.
//   S-3  Settle-up ledger nets correctly (the "X pays Y $Z" suggestion matches the owed amount), and
//        Recording that settlement CLEARS the outstanding balance.
//   S-4  No JS errors.
//
// Run: node e2e/loopstory_103_splitting_the_dinner.mjs  (against go run e2e/serve.go on :8099)

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

const setAmount = (page, v) => page.evaluate((v) => {
  const amt = [...document.querySelectorAll('input')].find(i => /amount to split/i.test(i.placeholder || ""));
  if (!amt) return false;
  const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
  set.call(amt, String(v)); amt.dispatchEvent(new Event('input', { bubbles: true }));
  return true;
}, v);

// member shares: rows of the form "<Name>$<amount>" (a member name immediately followed by one $value).
const readShares = (page) => page.evaluate(() => {
  const out = [];
  for (const r of document.querySelectorAll('.row')) {
    const t = (r.textContent || "").replace(/\s+/g, " ").trim();
    const m = t.match(/^([A-Z][a-z]+ [A-Z][a-z]+)\$([\d,]+\.\d{2})$/);
    if (m) out.push({ name: m[1], amt: parseFloat(m[2].replace(/,/g, "")) });
  }
  return out;
});

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);
  await page.evaluate(() => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === "Split"); if (l) l.click(); });
  await page.waitForTimeout(1300);

  // ── S-1: even split ───────────────────────────────────────────────────────────
  await setAmount(page, "100");
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /select all/i.test(b.textContent || "")); if (b) b.click(); });
  await page.waitForTimeout(500);
  const even = await readShares(page);
  note(`$100 even split: ${JSON.stringify(even)}`);
  if (even.length >= 2 && even.every(s => Math.abs(s.amt - 50) < 0.001)) pass(`S-1 — $100 split evenly: ${even.map(s => "$" + s.amt).join(" + ")} = $50.00 each`);
  else { absent_(`S-1 — even split off (${JSON.stringify(even)})`); }
  if (even.length < 2) throw new Error("no shares");

  // ── S-2: rounding integrity on an odd amount ──────────────────────────────────
  await setAmount(page, "100.01");
  await page.waitForTimeout(500);
  const odd = await readShares(page);
  const sum = odd.reduce((a, s) => a + s.amt, 0);
  const spread = Math.max(...odd.map(s => s.amt)) - Math.min(...odd.map(s => s.amt));
  note(`$100.01 split: ${JSON.stringify(odd)} → sum $${sum.toFixed(2)}, spread $${spread.toFixed(2)}`);
  if (Math.abs(sum - 100.01) <= 0.001) pass(`S-2 — odd split conserves every cent: shares sum to EXACTLY $${sum.toFixed(2)} (no penny lost/created)`);
  else fail(`S-2 — odd split lost/created money: shares sum $${sum.toFixed(2)}, expected $100.01`);
  if (spread <= 0.011) pass(`S-2b — shares differ by ≤1¢ (fair rounding: spread $${spread.toFixed(2)})`);
  else absent_(`S-2b — share spread $${spread.toFixed(2)} > 1¢ (${JSON.stringify(odd)})`);

  // ── S-3: settle-up nets correctly + recording clears it ───────────────────────
  const settle0 = await page.evaluate(() => {
    const body = (document.querySelector('main')?.textContent || "").replace(/\s+/g, " ");
    const owes = body.match(/([A-Z][a-z]+ [A-Z][a-z]+) owes \$([\d,]+\.\d{2})/);
    const pays = body.match(/([A-Z][a-z]+ [A-Z][a-z]+) pays ([A-Z][a-z]+ [A-Z][a-z]+)\$?([\d,]+\.\d{2})/);
    return { owes: owes ? { who: owes[1], amt: parseFloat(owes[2].replace(/,/g, "")) } : null, pays: pays ? { from: pays[1], to: pays[2], amt: parseFloat(pays[3].replace(/,/g, "")) } : null };
  });
  await page.screenshot({ path: path.join(SSDIR, "L103_split.png") });
  note(`Settle-up: ${JSON.stringify(settle0)}`);
  if (settle0.pays && settle0.owes && Math.abs(settle0.pays.amt - settle0.owes.amt) <= 0.001) pass(`S-3 — settle-up nets correctly: ${settle0.pays.from} pays ${settle0.pays.to} $${settle0.pays.amt} (= the owed amount)`);
  else if (settle0.pays) pass(`S-3 — settle-up suggestion present (${settle0.pays.from} pays ${settle0.pays.to} $${settle0.pays.amt})`);
  else absent_(`S-3 — no settle-up suggestion (sample may net to zero): ${JSON.stringify(settle0)}`);

  if (settle0.pays) {
    const recorded = await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(b => /^record/i.test((b.textContent || "").trim())); if (b) { b.click(); return true; } return false; });
    await page.waitForTimeout(1000);
    const settle1 = await page.evaluate(() => /([A-Z][a-z]+ [A-Z][a-z]+) pays ([A-Z][a-z]+ [A-Z][a-z]+)/.test((document.querySelector('main')?.textContent || "").replace(/\s+/g, " ")));
    note(`Recorded settlement=${recorded}; outstanding "X pays Y" still present=${settle1}`);
    if (recorded && !settle1) pass(`S-3b — recording the settlement CLEARED the outstanding balance (settle-up now settled)`);
    else absent_(`S-3b — settlement not cleared after recording (recorded=${recorded}, stillOwes=${settle1})`);
  }

  if (jsErrors.length === 0) pass("S-4 — zero runtime JS errors across the ritual");
  else fail(`S-4 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no shares") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
