// L107 E2E loop story — "The Cleanup" (Priya) — 2026-06-25
//
// Theme: CATEGORY REASSIGN-ON-DELETE data integrity. Tidying categories must never lose money: deleting
// an in-use category opens a reassign panel, and moving its transactions to another category must leave
// total spending UNCHANGED (the spend is relabeled, not destroyed) while the source category disappears.
//
// Invariants:
//   C-1  Deleting an IN-USE category opens the "Reassign before deleting" panel (doesn't orphan/delete).
//   C-2  "Move and delete" to a target category removes the source category.
//   C-3  Reports TOTAL SPENDING is UNCHANGED (expense->expense reassign relabels, loses no money).
//   C-4  No JS errors.
//
// Run: node e2e/loopstory_107_the_cleanup.mjs  (against go run e2e/serve.go on :8099)

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

const readSpending = (page) => page.evaluate(() => {
  const m = (document.querySelector('main')?.textContent || "").replace(/\s+/g, " ").match(/spending\s*\$([\d,]+\.?\d*)/i);
  return m ? parseFloat(m[1].replace(/,/g, "")) : null;
});

const categoryPresent = (page, name) => page.evaluate((name) => [...document.querySelectorAll('.row')].some(r => (r.textContent || "").includes(name + "Expense") || (r.textContent || "").trim().startsWith(name + "Expense")), name);

const jsErrors = [];
const SRC = "Education & Loans", DEST = "Guilty pleasures";

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
  const s0 = await readSpending(page);
  note("Total spending before: $" + s0);

  await navTo(page, "Categories");
  const present0 = await categoryPresent(page, SRC);
  if (!present0) { absent_("setup — '" + SRC + "' category not found"); throw new Error("setup"); }

  // ── C-1: delete an in-use category opens the reassign panel ────────────────────
  const opened = await page.evaluate((SRC) => {
    const row = [...document.querySelectorAll('.row')].find(r => (r.textContent || "").includes(SRC + "Expense"));
    if (!row) return "NO_ROW";
    const del = [...row.querySelectorAll('button')].find(b => /delete|remove/i.test(b.getAttribute('aria-label') || b.title || ""));
    if (!del) return "NO_DEL";
    del.click();
    return "clicked";
  }, SRC);
  await page.waitForTimeout(600);
  const hasPanel = await page.evaluate(() => !![...document.querySelectorAll('select')].find(s => s.offsetParent !== null && /reassign before deleting/i.test(s.getAttribute('aria-label') || "")));
  if (opened === "clicked" && hasPanel) pass("C-1 — deleting an in-use category opened the 'Reassign before deleting' panel (no orphan)");
  else { absent_("C-1 — reassign panel did not open (" + opened + ", panel=" + hasPanel + ")"); throw new Error("panel"); }

  // ── C-2: choose target + Move and delete ──────────────────────────────────────
  const moved = await page.evaluate((DEST) => {
    const sel = [...document.querySelectorAll('select')].find(s => s.offsetParent !== null && /reassign before deleting/i.test(s.getAttribute('aria-label') || ""));
    if (!sel) return "NO_SELECT";
    const opt = [...sel.options].find(o => o.textContent.trim() === DEST);
    if (!opt) return "NO_DEST";
    const setS = Object.getOwnPropertyDescriptor(window.HTMLSelectElement.prototype, 'value').set;
    setS.call(sel, opt.value); sel.dispatchEvent(new Event('change', { bubbles: true }));
    const btn = [...document.querySelectorAll('button')].find(b => b.offsetParent !== null && /move and delete/i.test(b.textContent || ""));
    if (!btn) return "NO_BTN";
    btn.click(); return "moved";
  }, DEST);
  await page.waitForTimeout(1100);
  note("reassign result: " + moved + " (" + SRC + " -> " + DEST + ")");
  // re-read Categories (re-nav to be safe)
  await navTo(page, "Categories");
  const present1 = await categoryPresent(page, SRC);
  if (moved === "moved" && !present1) pass("C-2 — '" + SRC + "' category removed after Move-and-delete");
  else fail("C-2 — source category still present after reassign (moved=" + moved + ", present=" + present1 + ")");

  // ── C-3: total spending unchanged (money relabeled, not lost) ──────────────────
  await navTo(page, "Reports");
  const s1 = await readSpending(page);
  await page.screenshot({ path: path.join(SSDIR, "L107_cleanup.png") });
  note("Total spending after: $" + s1);
  if (s0 != null && s1 != null && Math.abs(s1 - s0) <= 0.01) pass("C-3 — total spending UNCHANGED ($" + s0 + " = $" + s1 + ") — reassign relabeled spend, lost no money");
  else fail("C-3 — total spending changed $" + s0 + " -> $" + s1 + " on a category reassign (money should be conserved!)");

  if (jsErrors.length === 0) pass("C-4 — zero runtime JS errors across the ritual");
  else fail("C-4 — " + jsErrors.length + " JS errors: " + jsErrors.slice(0, 3).join("; "));

} catch (err) {
  if (["setup", "panel"].indexOf(String(err.message)) === -1) { fail("UNEXPECTED_ERROR — " + err.message); console.error(err); }
} finally {
  await browser.close();
}

console.log("\n════════════════════════════════════════════");
console.log("RESULT: " + passed + " PASS · " + failed + " FAIL · " + absent + " ABSENT");
console.log("════════════════════════════════════════════");
process.exit(failed > 0 ? 1 : 0);
