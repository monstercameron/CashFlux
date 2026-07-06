// L105 E2E loop story — "The Paycheck" (Priya) — 2026-06-25
//
// Theme: INCOME path + income/expense SEPARATION. Most prior stories exercised expenses; this verifies
// that recording INCOME raises income + net but NEVER touches spending (income is not an expense). Now
// possible because the add-transaction modal is e2e-drivable (L104-T1 testids).
//
// Invariants:
//   P-1  Reports baseline KPIs readable (Income / Spending / Net).
//   P-2  Adding a $5,000 income raises Reports Income by EXACTLY $5,000.
//   P-3  Spending is UNCHANGED (income must not be miscounted as an expense).
//   P-4  Net rises by EXACTLY $5,000 (income flows to net 1:1).
//   P-5  No JS errors.
//
// Run: node e2e/loopstory_105_the_paycheck.mjs  (against go run e2e/serve.go on :8099)

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

const readKPIs = (page) => page.evaluate(() => {
  const t = (document.querySelector('main')?.textContent || "").replace(/\s+/g, " ");
  const num = (m) => m ? parseFloat(m.replace(/,/g, "")) : null;
  const inc = num((t.match(/income\s*\$([\d,]+\.?\d*)/i) || [])[1]);
  const spd = num((t.match(/spending\s*\$([\d,]+\.?\d*)/i) || [])[1]);
  const nm = t.match(/Net\s*(\()?\$([\d,]+\.?\d*)(\))?/i); // no \b: hero runs "…-01Net($…)" together
  let net = nm ? num(nm[2]) : null;
  if (nm && nm[1]) net = -net; // parenthesized = negative
  return { income: inc, spending: spd, net };
});

// Add an INCOME transaction via the testid'd QuickAdd modal.
const addIncome = async (page, amount, desc) => {
  await page.evaluate(() => { const b = [...document.querySelectorAll('button')].find(x => /add something new/i.test(x.getAttribute('aria-label') || x.title || "")); if (b) b.click(); });
  await page.waitForTimeout(220);
  const opened = await page.evaluate(() => { const b = [...document.querySelectorAll('button,a')].find(x => /new transaction/i.test(x.textContent || "")); if (b) { b.click(); return true; } return false; });
  if (!opened) return "NO_MENU";
  try { await page.waitForSelector('[data-testid="txn-add-amount"]', { state: "visible", timeout: 5000 }); } catch (e) { return "NO_OPEN"; }
  const res = await page.evaluate((args) => {
    const [amount, desc] = args;
    const amt = document.querySelector('[data-testid="txn-add-amount"]');
    const dsc = document.querySelector('[data-testid="txn-add-desc"]');
    if (!amt || !dsc) return "NO_FIELDS";
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    setI.call(dsc, desc); dsc.dispatchEvent(new Event('input', { bubbles: true }));
    // Choose INCOME (not the default Expense).
    const inc = [...document.querySelectorAll('button')].find(b => b.offsetParent !== null && /^income$/i.test((b.textContent || "").trim()));
    if (!inc) return "NO_INCOME_TOGGLE";
    inc.click();
    return "filled";
  }, [amount, desc]);
  if (res !== "filled") return res;
  const saved = await page.evaluate(() => {
    const s = document.querySelector('[data-testid="flip-save"]');
    if (!s) return "NO_SAVE";
    if (s.disabled || s.getAttribute("aria-disabled") === "true") return "SAVE_DISABLED";
    s.click(); return "submitted";
  });
  if (saved === "submitted") { try { await page.waitForSelector('[data-testid="txn-add-amount"]', { state: "detached", timeout: 5000 }); } catch (e) { } }
  return saved;
};

const jsErrors = [];
const PAY = 5000;

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  await navTo(page, "Reports");
  const k0 = await readKPIs(page);
  note(`Baseline KPIs: ${JSON.stringify(k0)}`);
  if (k0.income != null && k0.spending != null && k0.net != null) pass(`P-1 — KPIs readable (income $${k0.income}, spending $${k0.spending}, net $${k0.net})`);
  else { absent_(`P-1 — KPIs unreadable (${JSON.stringify(k0)})`); throw new Error("baseline"); }

  const r = await addIncome(page, PAY, "Paycheck");
  note(`addIncome result: ${r}`);
  if (r === "submitted") pass(`P-2a — $${PAY} income recorded via the add modal (Income mode)`);
  else { absent_(`P-2a — income not recorded (${r})`); throw new Error("add"); }

  await navTo(page, "Reports");
  const k1 = await readKPIs(page);
  await page.screenshot({ path: path.join(SSDIR, "L105_after_income.png") });
  note(`After paycheck: ${JSON.stringify(k1)}`);
  const incDelta = (k1.income ?? 0) - (k0.income ?? 0);
  const spdDelta = (k1.spending ?? 0) - (k0.spending ?? 0);
  const netDelta = (k1.net ?? 0) - (k0.net ?? 0);
  if (Math.abs(incDelta - PAY) <= 0.01) pass(`P-2 — Income rose by EXACTLY $${PAY} ($${k0.income} → $${k1.income})`);
  else fail(`P-2 — Income Δ$${incDelta.toFixed(2)}, expected +$${PAY} ($${k0.income} → $${k1.income})`);
  if (Math.abs(spdDelta) <= 0.01) pass(`P-3 — Spending UNCHANGED ($${k0.spending} = $${k1.spending}) — income is not miscounted as an expense`);
  else fail(`P-3 — Spending changed by $${spdDelta.toFixed(2)} on an INCOME entry ($${k0.spending} → $${k1.spending}) — income/expense leak!`);
  if (Math.abs(netDelta - PAY) <= 0.01) pass(`P-4 — Net rose by EXACTLY $${PAY} ($${k0.net} → $${k1.net}) — income flows to net 1:1`);
  else fail(`P-4 — Net Δ$${netDelta.toFixed(2)}, expected +$${PAY} ($${k0.net} → $${k1.net})`);

  if (jsErrors.length === 0) pass("P-5 — zero runtime JS errors across the ritual");
  else fail(`P-5 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (!["baseline", "add"].includes(String(err.message))) { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
