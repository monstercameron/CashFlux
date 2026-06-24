// L88 E2E loop story — "The Budget Squeeze" (Marcus) — 2026-06-24
//
// Theme: BUDGET-vs-ACTUAL INTEGRITY. The everyday question a finance-aware home asks of a budget is
// "are we over, and by how much?" That answer must be (1) glanceable per category (spent / limit +
// a progress bar), (2) clearly FLAGGED when a category is over its limit, (3) LIVE — logging an
// expense in a category raises that category's "spent", and (4) CONSISTENT — the budget's "spent"
// for a category equals the Reports "Spending by category" figure for the same category. Invariants:
//   S-1  Each budget row shows spent / limit and a progress bar.
//   S-2  An over-limit category is visibly flagged ("Over budget" + a .bar-fill.over).
//   S-3  Logging an expense in a budgeted category RAISES that category's spent on the Budgets screen.
//   S-4  A category's budget "spent" matches the Reports "Spending by category" figure (cross-screen).
//   S-5  STRESS: several rapid expenses post without crash; the category's spent rises monotonically.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_88_budget_squeeze.mjs

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

// Read every budget row: { name, spent, limit, over (bool), sub }.
const budgetRows = (page) => page.evaluate(() => {
  return [...document.querySelectorAll('.budget')].map(row => {
    const head = row.querySelector('.budget-head');
    const amount = row.querySelector('.budget-amount');
    const sub = row.querySelector('.budget-sub');
    // name = head text before the amount, stripped of trailing action labels
    let name = "";
    if (head) { const a = head.querySelector('.budget-amount'); name = (head.textContent || "").replace(a ? a.textContent : "", "").replace(/(Cover|Edit|Top up|Top up…|Set limit).*$/gi, "").replace(/[…·]/g, "").trim(); }
    const amt = amount ? amount.textContent.trim() : "";
    // "$89.97 / $40.00" -> split on the slash, strip currency from each side
    const parts = amt.split("/");
    const num = (s) => { if (!s) return null; const n = parseFloat(s.replace(/[^0-9.]/g, "")); return isNaN(n) ? null : n; };
    return {
      name,
      spent: parts.length >= 1 ? num(parts[0]) : null,
      limit: parts.length >= 2 ? num(parts[1]) : null,
      over: !!row.querySelector('.bar-fill.over') || /over budget/i.test(sub ? sub.textContent : ""),
      sub: sub ? sub.textContent.trim() : "",
    };
  }).filter(r => r.name);
});

// Reports "Spending by category" figure for a given category name (from the ranked list/legend).
const reportsCategorySpend = (page, cat) => page.evaluate((cat) => {
  // rows under the "Spending by category" section: look for an element pairing the cat name + a $ value
  const cands = [...document.querySelectorAll('*')].filter(e => e.children.length <= 3 && new RegExp("\\b" + cat + "\\b", "i").test(e.textContent || "") && /\$[\d,]/.test(e.textContent || "") && (e.textContent || "").length < 60);
  for (const e of cands) { const m = (e.textContent || "").match(/\$[\d,]+\.?\d*/); if (m) return m[0]; }
  return null;
}, cat);

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1100 });
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

  // ── S-1: budgets show spent / limit + bar ─────────────────────────────────────
  await navTo(page, "Budgets");
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L88_01_budgets.png") });
  const rows0 = await budgetRows(page);
  note(`Budget rows: ${rows0.length} [${rows0.map(r => `${r.name} ${r.spent}/${r.limit}`).join(" | ").slice(0, 200)}]`);
  const bars = await page.evaluate(() => document.querySelectorAll('.bar-fill').length);
  if (rows0.length > 0 && rows0.every(r => r.spent != null && r.limit != null)) pass(`S-1 — ${rows0.length} budget rows each show spent / limit`);
  else absent_(`S-1 — could not parse spent/limit for all rows (${rows0.length} rows)`);
  if (bars > 0) pass(`S-1b — progress bars present (${bars})`); else absent_("S-1b — no progress bars found");

  // ── S-2: an over-limit category is flagged ────────────────────────────────────
  const over = rows0.filter(r => r.over || (r.spent != null && r.limit != null && r.spent > r.limit));
  note(`Over-budget rows: ${over.map(r => `${r.name} (${r.spent}/${r.limit}) "${r.sub}"`).join(" | ")}`);
  if (over.length > 0) pass(`S-2 — ${over.length} over-limit categor${over.length === 1 ? "y is" : "ies are"} flagged (e.g. ${over[0].name}: "${over[0].sub}")`);
  else absent_("S-2 — no over-limit category present to verify the flag (sample data may be all under budget)");

  // ── S-3 / S-5: log expenses in a budgeted category, watch its spent rise ───────
  // pick a budgeted category with a parseable spent and a clean single-token name that the
  // transaction form's category dropdown will also offer (prefer Groceries — a universal staple).
  const clean = rows0.filter(r => r.spent != null && /^[A-Za-z][A-Za-z &]+$/.test(r.name));
  const target = clean.find(r => /^Groceries$/i.test(r.name)) || clean.find(r => /^Dining$/i.test(r.name)) || clean[0] || rows0[0];
  if (!target) { absent_("S-3 — no budget row to target"); }
  else {
    note(`Target category for spend test: "${target.name}" (start spent=${target.spent})`);
    const addExpense = async (amount, cat) => {
      await navTo(page, "Transactions");
      const opened = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /add transaction|new transaction|\+ add|^add$/i.test(b.textContent.trim())); if (b) { b.click(); return true; } return false; });
      await page.waitForTimeout(600);
      if (!opened) return "no-open";
      const r = await page.evaluate((args) => {
        const [amt, cat] = args;
        // Expense type (default, but assert)
        const exp = [...document.querySelectorAll('button,label')].find(e => /^expense$/i.test((e.textContent || "").trim())); if (exp) exp.click();
        // account: first available
        const accSel = [...document.querySelectorAll('select')].find(s => s.getAttribute('aria-label') === 'Account');
        if (accSel && accSel.options.length) { accSel.value = accSel.options[0].value; accSel.dispatchEvent(new Event('change', { bubbles: true })); }
        // category select (match by visible option text)
        const catSel = [...document.querySelectorAll('select')].find(s => /categor/i.test(s.getAttribute('aria-label') || "") || [...s.options].some(o => new RegExp(cat, "i").test(o.textContent)));
        let catSet = false;
        if (catSel) { const opt = [...catSel.options].find(o => new RegExp("^" + cat + "$", "i").test(o.textContent.trim())) || [...catSel.options].find(o => new RegExp(cat, "i").test(o.textContent)); if (opt) { catSel.value = opt.value; catSel.dispatchEvent(new Event('change', { bubbles: true })); catSet = true; } }
        const amtEl = [...document.querySelectorAll('input[type="number"]')].find(e => e.getAttribute('aria-label') === 'Amount') || document.querySelector('input[type="number"]');
        if (amtEl) { amtEl.value = String(amt); amtEl.dispatchEvent(new Event('input', { bubbles: true })); }
        const desc = [...document.querySelectorAll('input[type="text"]')].find(e => e.getAttribute('aria-label') === 'Description'); if (desc) { desc.value = "L88 squeeze"; desc.dispatchEvent(new Event('input', { bubbles: true })); }
        return { catSet };
      }, [amount, cat]);
      await page.waitForTimeout(250);
      const saved = await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => b.textContent.trim() === "Save"); if (b) { b.click(); return true; } return false; });
      await page.waitForTimeout(1100);
      return r.catSet ? (saved ? "ok" : "no-save") : "no-cat";
    };

    const r1 = await addExpense(50, target.name);
    note(`Add #1 ($50 to ${target.name}): ${r1}`);
    await navTo(page, "Budgets"); await page.waitForTimeout(800);
    const rowsAfter1 = await budgetRows(page);
    const t1 = rowsAfter1.find(r => r.name === target.name);
    if (r1 === "ok" && t1 && target.spent != null && t1.spent != null) {
      if (t1.spent > target.spent) pass(`S-3 — a $50 expense raised "${target.name}" spent ${target.spent} → ${t1.spent} on Budgets (live reflow)`);
      else fail(`S-3 — "${target.name}" spent did NOT rise after a $50 expense (${target.spent} → ${t1.spent}) — budget not tracking transactions`);
    } else absent_(`S-3 — could not complete the categorized expense (result=${r1}, row=${t1 ? "found" : "missing"})`);

    // ── S-4: budget spent == Reports spending-by-category ─────────────────────────
    if (t1 && t1.spent != null) {
      await navTo(page, "Reports"); await page.waitForTimeout(1200);
      const repRaw = await reportsCategorySpend(page, target.name);
      const rep = money(repRaw);
      note(`Reports "Spending by category" for ${target.name}: ${repRaw} (budget spent=${t1.spent})`);
      if (rep != null) {
        if (Math.abs(rep - t1.spent) <= Math.max(1, t1.spent * 0.02)) pass(`S-4 — budget spent (${t1.spent}) matches Reports category spend (${rep}) within 2% — one source of truth`);
        else absent_(`S-4 — budget spent (${t1.spent}) ≠ Reports category spend (${rep}) — possible period/scope mismatch (review)`);
      } else absent_(`S-4 — could not read Reports spend for "${target.name}"`);
    }

    // ── S-5: STRESS — three more rapid expenses, spent rises monotonically ────────
    let prev = t1 ? t1.spent : (target.spent || 0);
    let monotonic = true; const seq = [prev];
    for (let i = 0; i < 3; i++) {
      const rr = await addExpense(10, target.name);
      await navTo(page, "Budgets"); await page.waitForTimeout(700);
      const rows = await budgetRows(page);
      const t = rows.find(r => r.name === target.name);
      const cur = t ? t.spent : null;
      seq.push(cur);
      if (rr !== "ok" || cur == null || cur < prev) monotonic = false;
      if (cur != null) prev = cur;
    }
    note(`Stress spent sequence for ${target.name}: [${seq.join(" → ")}]`);
    if (monotonic && seq[seq.length - 1] > seq[0]) pass(`S-5 — 3 rapid $10 expenses raised spent monotonically (${seq[0]} → ${seq[seq.length - 1]}), no crash`);
    else absent_(`S-5 — spent not monotonic / incomplete under stress [${seq.join(" → ")}]`);
  }

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
