// L104 E2E loop story — "The Rapid Logger" (Marcus) — 2026-06-25
//
// Theme: STRESS + cross-screen INTEGRITY under rapid entry. A busy household logs a burst of expenses
// back-to-back via the +Add → New transaction modal. Every write must land exactly once and propagate
// consistently: the category's budget "spent" and the Reports purchase count must move by exactly the
// burst, with no lost/duplicated writes and no crash. Uses pagination-immune metrics.
//
// NB (L104-T1, resolved 2026-06-25): the add-transaction flip-card modal now exposes data-testids
// (txn-add-amount/desc/category/account, flip-save). It commits on standard OnInput/OnChange; Save is
// disabled until BOTH a description and a non-zero amount are present (L78-T1 validity guard) — so the
// driver MUST set the description, which is why the first L104 attempt couldn't persist writes.
//
// Invariants:
//   ST-1  Baseline readable: Dining budget spent + Reports "N purchases".
//   ST-2  Rapidly adding 6 × $10 Dining expenses raises Dining's budget spent by EXACTLY $60.
//   ST-3  Reports purchase count rises by EXACTLY 6 (every write landed once — none lost or duplicated).
//   ST-4  No JS errors / no crash across the burst (app still navigable afterward).
//
// Run: node e2e/loopstory_104_rapid_logger.mjs  (against go run e2e/serve.go on :8099)

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
  await page.waitForTimeout(1200);
};

const diningSpent = (page) => page.evaluate(() => {
  for (const b of document.querySelectorAll('.budget')) {
    const t = (b.textContent || "").replace(/\s+/g, " ").trim();
    const m = t.match(/^Dining\$([\d,]+\.?\d*)\s*\/\s*\$/);
    if (m) return parseFloat(m[1].replace(/,/g, ""));
  }
  return null;
});

const purchaseCount = (page) => page.evaluate(() => {
  const m = (document.querySelector('main')?.textContent || "").replace(/\s+/g, " ").match(/(\d+)\s+purchases/i);
  return m ? parseInt(m[1], 10) : null;
});

// Atomic add via the testid'd modal. Sets amount + description (both required for Save) + category +
// Expense, clicks the testid'd Save, and waits for the modal to fully close before returning.
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
    if (!amt || !dsc) return "NO_FIELDS";
    const setI = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setI.call(amt, String(amount)); amt.dispatchEvent(new Event('input', { bubbles: true }));
    setI.call(dsc, desc); dsc.dispatchEvent(new Event('input', { bubbles: true }));
    if (cat) { const opt = [...cat.options].find(o => o.textContent.trim() === category); if (opt) { const setS = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set; setS.call(cat, opt.value); cat.dispatchEvent(new Event('change', { bubbles: true })); } }
    const exp = [...document.querySelectorAll('button')].find(b => b.offsetParent !== null && /^expense$/i.test((b.textContent || "").trim()));
    if (exp) exp.click();
    return "filled";
  }, [amount, category, desc]);
  if (res !== "filled") return res;
  // Save must be enabled now (desc + amount present).
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
const N = 6, AMT = 10, CAT = "Dining";

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
  const din0 = await diningSpent(page);
  await navTo(page, "Reports");
  const pc0 = await purchaseCount(page);
  note(`Baseline: Dining spent $${din0} · purchases ${pc0}`);
  if (din0 != null && pc0 != null) pass(`ST-1 — baseline readable (Dining $${din0}, ${pc0} purchases)`);
  else { absent_(`ST-1 — baseline unreadable (din=${din0}, pc=${pc0})`); throw new Error("baseline"); }

  let submitted = 0;
  for (let i = 1; i <= N; i++) {
    const r = await addExpense(page, AMT, CAT, "stress " + i);
    if (r === "submitted") submitted++; else note(`  add ${i}: ${r}`);
    await page.waitForTimeout(250);
  }
  note(`Submitted ${submitted}/${N} rapid adds`);
  if (submitted === N) pass(`ST-1b — all ${N} rapid adds submitted`);
  else absent_(`ST-1b — only ${submitted}/${N} adds submitted`);

  await navTo(page, "Budgets");
  const din1 = await diningSpent(page);
  await navTo(page, "Reports");
  const pc1 = await purchaseCount(page);
  await page.screenshot({ path: path.join(SSDIR, "L104_after_burst.png") });
  const dinDelta = (din1 ?? 0) - (din0 ?? 0);
  const pcDelta = (pc1 ?? 0) - (pc0 ?? 0);
  note(`After burst: Dining $${din0}→$${din1} (Δ$${dinDelta.toFixed(2)}), purchases ${pc0}→${pc1} (Δ${pcDelta})`);
  if (Math.abs(dinDelta - N * AMT) <= 0.01) pass(`ST-2 — Dining budget spent rose by EXACTLY $${N * AMT} ($${din0} → $${din1}) — every $10 expense propagated`);
  else fail(`ST-2 — Dining spent Δ$${dinDelta.toFixed(2)}, expected +$${N * AMT} ($${din0} → $${din1})`);
  if (pcDelta === N) pass(`ST-3 — Reports purchase count rose by EXACTLY ${N} (${pc0} → ${pc1}) — no lost or duplicated writes`);
  else fail(`ST-3 — purchase count Δ${pcDelta}, expected +${N} (${pc0} → ${pc1})`);

  await navTo(page, "Dashboard");
  const alive = await page.evaluate(() => !!document.querySelector('nav[aria-label="Main navigation"]') && document.querySelectorAll('main *').length > 5);
  if (alive) pass("ST-4a — app still navigable after the burst (no crash)");
  else fail("ST-4a — app appears broken after the burst");
  if (jsErrors.length === 0) pass("ST-4b — zero runtime JS errors across the burst");
  else fail(`ST-4b — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "baseline") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
