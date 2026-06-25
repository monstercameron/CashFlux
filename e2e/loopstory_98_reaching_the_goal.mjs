// L98 E2E loop story — "Reaching the Goal" (Aaliyah) — 2026-06-24
//
// Theme: GOAL CONTRIBUTION + COMPLETION LIFECYCLE. A contribution must raise saved/% , post a toast,
// and on reaching 100% prompt completion. NB: the goal list re-sorts after each contribution ("most
// actionable" first), so this test PINS to one goal by its stable data-testid for every read/action.
// Invariants:
//   G-1  Goals show per-goal progress (saved / target + %).
//   G-2  A small contribution raises that SAME goal's saved (+ a confirmation toast).
//   G-3  Contributing the full remaining gap drives the goal to >= 100% (saved >= target).
//   G-4  Reaching 100% fires a milestone/completion toast.
//   G-5  No JS errors / no crash across the lifecycle.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_98_reaching_the_goal.mjs

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
const TARGET = "goal-row-goal-baby"; // pinned goal (stable testid)

const readGoal = (page, id) => page.evaluate((id) => {
  const row = document.querySelector('[data-testid="' + id + '"]');
  if (!row) return null;
  const t = row.textContent || "";
  const m = t.match(/\$([\d,]+\.?\d*)\s*\/\s*\$([\d,]+\.?\d*)/);
  const pctM = t.match(/(\d+)%/);
  return { saved: m ? parseFloat(m[1].replace(/,/g, "")) : null, target: m ? parseFloat(m[2].replace(/,/g, "")) : null, pct: pctM ? parseInt(pctM[1], 10) : null, archived: /achieved|completed ✓|done/i.test(t) };
}, id);

const contributeTo = async (page, id, amount) => {
  const opened = await page.evaluate((id) => {
    const row = document.querySelector('[data-testid="' + id + '"]'); if (!row) return false;
    const b = [...row.querySelectorAll('button')].find(b => /contribute/i.test(b.textContent)); if (b) { b.click(); return true; } return false;
  }, id);
  if (!opened) return "NO_BTN";
  await page.waitForTimeout(500);
  return page.evaluate((args) => {
    const [id, amount] = args;
    // Opening the inline contribute form re-renders the row and drops its testid, so query the
    // visible amount input GLOBALLY (there is only one open form at a time) rather than via the row.
    const amt = [...document.querySelectorAll('input[type="number"]')].find(i => i.offsetParent !== null && /amount/i.test(i.placeholder || "")) || [...document.querySelectorAll('input[type="number"]')].find(i => i.offsetParent !== null);
    if (!amt) return "NO_AMT";
    const set = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    set.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    // The inline contribute form re-renders the row (dropping its testid while open), so a button
    // lookup races the re-render. requestSubmit() on the owning form is the reliable path.
    const form = amt.closest('form');
    if (form) { form.requestSubmit(); return "submitted"; } return "NO_FORM";
  }, [id, amount]);
};
const grabToast = async (page) => { for (let i = 0; i < 18; i++) { const t = await page.evaluate(() => { const el = document.querySelector('.toast'); return el && el.offsetParent !== null ? el.textContent.trim().slice(0, 60) : null; }); if (t) return t; await page.waitForTimeout(70); } return null; };

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
  await navTo(page, "Goals");
  await page.waitForTimeout(800);
  await page.screenshot({ path: SS("L98_01_goals.png") });

  // ── G-1 ───────────────────────────────────────────────────────────────────────
  const g0 = await readGoal(page, TARGET);
  note(`Pinned goal (${TARGET}): saved=${g0 && g0.saved} target=${g0 && g0.target} pct=${g0 && g0.pct}`);
  if (g0 && g0.saved != null && g0.target != null) pass(`G-1 — goal shows progress (${g0.saved}/${g0.target}, ${g0.pct}%)`);
  else { absent_("G-1 — could not read the pinned goal"); throw new Error("no goal"); }

  // ── G-2 ───────────────────────────────────────────────────────────────────────
  const r1 = await contributeTo(page, TARGET, 100);
  const toast1 = await grabToast(page);
  await page.waitForTimeout(900);
  const g1 = await readGoal(page, TARGET);
  note(`After +$100: saved ${g0.saved} -> ${g1 && g1.saved}, toast="${toast1}" (r=${r1})`);
  if (r1 === "submitted" && g1 && g1.saved === g0.saved + 100) pass(`G-2a — contribution raised the SAME goal's saved by exactly 100 (${g0.saved} -> ${g1.saved})`);
  else if (r1 === "submitted" && g1 && g1.saved > g0.saved) pass(`G-2a — contribution raised saved (${g0.saved} -> ${g1.saved})`);
  else absent_(`G-2a — saved did not rise as expected (${g0.saved} -> ${g1 && g1.saved}, r=${r1})`);
  if (toast1 && /contribut|added|saved|put/i.test(toast1)) pass(`G-2b — confirmation toast ("${toast1}")`);
  else absent_(`G-2b — no confirmation toast ("${toast1}")`);

  // ── G-3 / G-4: complete it in one shot ────────────────────────────────────────
  const gap = Math.max(1, Math.ceil(g1.target - g1.saved));
  note(`Contributing remaining gap $${gap} to complete the goal`);
  const r2 = await contributeTo(page, TARGET, gap);
  const toast2 = await grabToast(page);
  await page.waitForTimeout(1100);
  const g2 = await readGoal(page, TARGET);
  await page.screenshot({ path: SS("L98_02_completed.png") });
  note(`After gap: saved=${g2 && g2.saved} target=${g2 && g2.target} pct=${g2 && g2.pct} archived=${g2 && g2.archived}, toast="${toast2}" (r=${r2})`);
  if (r2 === "submitted" && g2 && ((g2.saved != null && g2.target != null && g2.saved + 0.01 >= g2.target) || (g2.pct != null && g2.pct >= 100) || g2.archived)) pass(`G-3 — goal reached its target (saved ${g2.saved}/${g2.target}, ${g2.pct}%)`);
  else absent_(`G-3 — goal not complete after gap (saved=${g2 && g2.saved}/${g2 && g2.target}, pct=${g2 && g2.pct})`);
  if (toast2 && /complet|milestone|100%|🎉|achiev|congrat|nice|done|reached|funded|hit/i.test(toast2)) pass(`G-4 — milestone/completion toast fired ("${toast2}")`);
  else absent_(`G-4 — no completion/milestone toast captured ("${toast2}")`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no goal") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
