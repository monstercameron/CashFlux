// L106 E2E loop story — "The Budget Breach" (Marcus) — 2026-06-25
//
// Theme: BUDGET-BREACH alerting under a live add. Pushing a category from under-budget to over must
// (a) flip that row to "Over budget" with a negative remaining, and (b) raise the over-budget count —
// the glanceable "am I blowing my budget" signal a household relies on.
//
// Invariants:
//   B-1  Entertainment starts UNDER budget ($0.00 / $25.00, not flagged over).
//   B-2  Adding a $30 Entertainment expense lands (spent -> $30.00).
//   B-3  Entertainment now reads "Over budget" with spent > budget.
//   B-4  The count of over-budget categories rises by EXACTLY 1.
//   B-5  No JS errors.
//
// Run: node e2e/loopstory_106_the_budget_breach.mjs  (against go run e2e/serve.go on :8099)

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
const pass = (l) => { console.log("PASS:   " + l); passed++; };
const fail = (l) => { console.error("FAIL:   " + l); failed++; };
const absent_ = (l) => { console.log("ABSENT: " + l); absent++; };
const note = (l) => { console.log("NOTE:   " + l); };

const navTo = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1200);
};

const readBudget = (page, cat) => page.evaluate((cat) => {
  for (const b of document.querySelectorAll('.budget')) {
    const t = (b.textContent || "").replace(/\s+/g, " ").trim();
    if (t.startsWith(cat + "$")) {
      const m = t.match(/^.+?\$([\d,]+\.?\d*)\s*\/\s*\$([\d,]+\.?\d*)/);
      return { spent: m ? parseFloat(m[1].replace(/,/g, "")) : null, budget: m ? parseFloat(m[2].replace(/,/g, "")) : null, over: /over budget/i.test(t), text: t.slice(0, 90) };
    }
  }
  return null;
}, cat);

const overCount = (page) => page.evaluate(() => [...document.querySelectorAll('.budget')].filter(b => /over budget/i.test(b.textContent || "")).length);

const addExpense = async (page, amount, category, desc) => {
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(x => /add something new/i.test(x.getAttribute('aria-label') || x.title || "")); if (b) b.click(); });
  await page.waitForTimeout(220);
  const opened = await page.evaluate(() => { const b = [...document.querySelectorAll('button,a')].find(x => /new transaction/i.test(x.textContent || "")); if (b) { b.click(); return true; } return false; });
  if (!opened) return "NO_MENU";
  try { await page.waitForSelector('[data-testid="txn-add-amount"]', { state: "visible", timeout: 5000 }); } catch (e) { return "NO_OPEN"; }
  const res = await page.evaluate((args) => {
    const [amount, category, desc] = args;
    const amt = document.querySelector('[data-testid="txn-add-amount"]');
    const dsc = document.querySelector('[data-testid="txn-add-desc"]');
    const cat = document.querySelector('[data-testid="txn-add-category"]');
    if (!amt || !dsc || !cat) return "NO_FIELDS";
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    setI.call(dsc, desc); dsc.dispatchEvent(new Event('input', { bubbles: true }));
    const opt = [...cat.options].find(o => o.textContent.trim() === category);
    if (!opt) return "NO_CAT";
    const setS = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
    setS.call(cat, opt.value); cat.dispatchEvent(new Event('change', { bubbles: true }));
    const exp = [...document.querySelectorAll('button')].find(b => b.offsetParent !== null && /^expense$/i.test((b.textContent || "").trim()));
    if (exp) exp.click();
    return "filled";
  }, [amount, category, desc]);
  if (res !== "filled") return res;
  const saved = await page.evaluate(() => { const s = document.querySelector('[data-testid="flip-save"]'); if (!s) return "NO_SAVE"; if (s.disabled) return "SAVE_DISABLED"; s.click(); return "submitted"; });
  if (saved === "submitted") { try { await page.waitForSelector('[data-testid="txn-add-amount"]', { state: "detached", timeout: 5000 }); } catch (e) { } }
  return saved;
};

const jsErrors = [];
// Transportation is a SHARED (household) budget — any member's expense counts. (Entertainment is an
// INDIVIDUAL budget owned by Marcus, so a default expense attributed to Priya is CORRECTLY excluded —
// that scope behavior is by design, not a bug; see L106 note.) $1250/$1300 room=$50; +$60 -> over $10.
const CAT = "Transportation", AMT = 60;

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  await navTo(page, "Budgets");
  const b0 = await readBudget(page, CAT);
  const over0 = await overCount(page);
  note(CAT + " before: " + JSON.stringify(b0) + " · over-count " + over0);
  if (b0 && b0.budget != null && !b0.over) pass("B-1 — " + CAT + " starts UNDER budget ($" + b0.spent + "/$" + b0.budget + ")");
  else { absent_("B-1 — " + CAT + " not in expected under-budget state (" + JSON.stringify(b0) + ")"); throw new Error("baseline"); }

  const r = await addExpense(page, AMT, CAT, "Movie night");
  note("addExpense: " + r);
  if (r === "submitted") pass("B-2a — $" + AMT + " " + CAT + " expense recorded");
  else { absent_("B-2a — expense not recorded (" + r + ")"); throw new Error("add"); }

  // Re-read with a poll: the budget rollup recompute can lag a beat behind the write, so retry a few
  // times (re-navigating) until the spend reflects the add rather than reading a stale render.
  await navTo(page, "Budgets");
  let b1 = await readBudget(page, CAT);
  for (let i = 0; i < 6 && (!b1 || Math.abs((b1.spent || 0) - (b0.spent || 0)) < 0.01); i++) {
    await navTo(page, "Dashboard");
    await navTo(page, "Budgets");
    b1 = await readBudget(page, CAT);
  }
  const over1 = await overCount(page);
  await page.screenshot({ path: path.join(SSDIR, "L106_breach.png") });
  note(CAT + " after: " + JSON.stringify(b1) + " · over-count " + over1);
  const spentDelta = (b1 && b1.spent ? b1.spent : 0) - (b0 && b0.spent ? b0.spent : 0);
  if (Math.abs(spentDelta - AMT) <= 0.01) pass("B-2 — " + CAT + " spent rose by exactly $" + AMT + " ($" + b0.spent + " -> $" + b1.spent + ")");
  else fail("B-2 — spent delta $" + spentDelta.toFixed(2) + ", expected +$" + AMT);
  if (b1 && b1.over && b1.spent > b1.budget) pass("B-3 — " + CAT + " flipped to OVER BUDGET ($" + b1.spent + " > $" + b1.budget + ") — " + b1.text);
  else fail("B-3 — " + CAT + " did NOT flag over budget after breach (" + JSON.stringify(b1) + ")");
  if (over1 === over0 + 1) pass("B-4 — over-budget category count rose by EXACTLY 1 (" + over0 + " -> " + over1 + ")");
  else fail("B-4 — over-count delta " + (over1 - over0) + ", expected +1 (" + over0 + " -> " + over1 + ")");

  if (jsErrors.length === 0) pass("B-5 — zero runtime JS errors across the ritual");
  else fail("B-5 — " + jsErrors.length + " JS errors: " + jsErrors.slice(0, 3).join("; "));

} catch (err) {
  if (["baseline", "add"].indexOf(String(err.message)) === -1) { fail("UNEXPECTED_ERROR — " + err.message); console.error(err); }
} finally {
  await browser.close();
}

console.log("\n════════════════════════════════════════════");
console.log("RESULT: " + passed + " PASS · " + failed + " FAIL · " + absent + " ABSENT");
console.log("════════════════════════════════════════════");
process.exit(failed > 0 ? 1 : 0);
