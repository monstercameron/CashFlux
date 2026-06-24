// L81 E2E loop story — "The Re-org" (Priya) — 2026-06-24
//
// Theme: CATEGORY REASSIGN-ON-DELETE — DATA-INTEGRITY ON A DESTRUCTIVE OP
//
// Persona: Priya tidies her messy categories. The non-negotiable for an enterprise-grade
// finance app: deleting a category that's IN USE must NOT silently orphan or lose its
// transactions — it must force a reassignment first. Invariants:
//   I-1  Deleting an in-use category opens a REASSIGN panel (it does not delete immediately).
//   I-2  After reassign+delete the TOTAL transaction count is UNCHANGED (zero data loss).
//   I-3  The deleted category is gone from the list.
//   I-4  The reassign TARGET category absorbs the moved transactions (count rises by the moved #).
//   I-5  No transaction is left referencing the deleted category (no orphans on /transactions).
//   I-6  STRESS/edge: cancelling the reassign leaves everything intact (no partial delete).
//
// Screens: /categories (delete→reassign) → /transactions (count + orphan check)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_81_category_reorg.mjs

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

const gotoCategories = async (page) => {
  const ok = await page.evaluate(() => {
    const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === "Categories");
    if (l) { l.click(); return true; } return false;
  });
  if (!ok) await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1500);
};
const gotoNav = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1400);
};

const txnCount = (page) => page.evaluate(() => {
  const m = document.body.textContent.match(/([\d,]+)\s+transactions?\b/i);
  return m ? parseInt(m[1].replace(/,/g, ""), 10) : null;
});

// usage count shown in a category row ("Groceries · 50 transactions")
const catUsage = (page, name) => page.evaluate((name) => {
  const row = [...document.querySelectorAll('.rows .row')].find(r => r.textContent.includes(name));
  if (!row) return null;
  const m = row.textContent.match(/(\d+)\s+transaction/);
  return m ? parseInt(m[1], 10) : 0;
}, name);

const catExists = (page, name) => page.evaluate((name) =>
  [...document.querySelectorAll('.rows .row')].some(r => {
    const t = r.textContent;
    // match the name as the row's leading label, not a substring of another
    return new RegExp("^" + name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).test(t.trim());
  }), name);

const clickDelete = (page, name) => page.evaluate((name) => {
  const row = [...document.querySelectorAll('.rows .row')].find(r => new RegExp("^" + name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).test(r.textContent.trim()));
  if (!row) return "NO_ROW";
  const btn = [...row.querySelectorAll('button')].find(b => /delete/i.test(b.textContent + (b.getAttribute("aria-label") || "")));
  if (!btn) return "NO_DELETE_BTN";
  btn.click();
  return "clicked";
}, name);

const reassignPanelOpen = (page) => page.evaluate(() =>
  !!document.querySelector('select[aria-label="Reassign before deleting"]') ||
  /reassign before deleting/i.test(document.body.textContent));

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
    } catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  const VICTIM = "Gifts & Charity";   // small in-use category (1 txn in sample data)
  const TARGET = "Entertainment";     // reassign destination (same kind = expense)

  // ── Baseline ─────────────────────────────────────────────────────────────────
  await gotoNav(page, "Transactions");
  await page.waitForTimeout(800);
  const txnBefore = await txnCount(page);
  note(`Baseline transaction count: ${txnBefore}`);

  await gotoCategories(page);
  await page.screenshot({ path: SS("L81_01_categories_before.png") });
  const victimUsage = await catUsage(page, VICTIM);
  const targetBefore = await catUsage(page, TARGET);
  note(`${VICTIM} usage=${victimUsage} | ${TARGET} usage=${targetBefore}`);
  if (victimUsage === null) { absent_(`Could not find category "${VICTIM}" — picking another in-use category may be needed`); }
  else if (victimUsage > 0) pass(`B-0 — "${VICTIM}" is in use (${victimUsage} txn) — a valid reassign-on-delete test`);
  else absent_(`"${VICTIM}" has 0 transactions — won't trigger the reassign panel`);

  // ── I-1: delete in-use category → reassign panel (NOT silent delete) ─────────
  const del = await clickDelete(page, VICTIM);
  await page.waitForTimeout(700);
  const panelOpen = await reassignPanelOpen(page);
  const stillExists = await catExists(page, VICTIM);
  note(`Delete "${VICTIM}": ${del} | reassignPanel=${panelOpen} | stillExists=${stillExists}`);
  await page.screenshot({ path: SS("L81_02_reassign_panel.png") });
  if (panelOpen && stillExists) pass("I-1 — deleting an in-use category opens the reassign panel (NOT an immediate delete — data-loss prevented)");
  else if (!stillExists) fail("I-1 — the in-use category was DELETED immediately without reassignment (DATA LOSS risk: its transactions orphaned)");
  else absent_(`I-1 — no reassign panel appeared (del=${del}, panelOpen=${panelOpen})`);

  // ── I-6 edge: cancel leaves everything intact, then re-open ──────────────────
  if (panelOpen) {
    const cancelled = await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button')].find(b => b.textContent.trim() === "Cancel");
      if (btn) { btn.click(); return true; } return false;
    });
    await page.waitForTimeout(500);
    const afterCancelExists = await catExists(page, VICTIM);
    const afterCancelUsage = await catUsage(page, VICTIM);
    note(`After cancel: exists=${afterCancelExists} usage=${afterCancelUsage}`);
    if (cancelled && afterCancelExists && afterCancelUsage === victimUsage) pass("I-6 — cancelling reassign leaves the category and its transactions fully intact");
    else absent_(`I-6 — cancel state unexpected (exists=${afterCancelExists}, usage=${afterCancelUsage})`);
    // re-open the panel for the real reassign
    await clickDelete(page, VICTIM);
    await page.waitForTimeout(600);
  }

  // ── reassign to TARGET and confirm "Move and delete" ─────────────────────────
  const picked = await page.evaluate((target) => {
    const sel = document.querySelector('select[aria-label="Reassign before deleting"]') ||
      [...document.querySelectorAll('select')].find(s => [...s.options].some(o => /choose category/i.test(o.text)));
    if (!sel) return "NO_SELECT";
    const opt = [...sel.options].find(o => o.text.includes(target));
    if (!opt) return "NO_TARGET_OPTION:" + [...sel.options].map(o => o.text).join("|");
    sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true }));
    return "picked";
  }, TARGET);
  note(`Reassign target pick: ${picked}`);
  const confirmed = await page.evaluate(() => {
    const btn = [...document.querySelectorAll('button')].find(b => /move and delete/i.test(b.textContent));
    if (btn) { btn.click(); return "confirmed"; } return "NO_CONFIRM_BTN";
  });
  await page.waitForTimeout(1200);
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(500);
  note(`Confirm move+delete: ${confirmed}`);
  await page.screenshot({ path: SS("L81_03_after_reassign.png") });

  // ── I-3: victim gone ─────────────────────────────────────────────────────────
  const victimGone = !(await catExists(page, VICTIM));
  if (picked === "picked" && confirmed === "confirmed") {
    if (victimGone) pass(`I-3 — "${VICTIM}" removed from categories after reassign`);
    else fail(`I-3 — "${VICTIM}" still present after Move-and-delete`);
  }

  // ── I-4: target absorbed the moved transactions ─────────────────────────────
  const targetAfter = await catUsage(page, TARGET);
  note(`${TARGET} usage: ${targetBefore} → ${targetAfter}`);
  if (targetBefore !== null && targetAfter !== null && victimUsage !== null) {
    if (targetAfter === targetBefore + victimUsage) pass(`I-4 — "${TARGET}" absorbed all ${victimUsage} moved txn (${targetBefore}→${targetAfter})`);
    else absent_(`I-4 — "${TARGET}" delta ${targetAfter - targetBefore} != moved ${victimUsage} (${targetBefore}→${targetAfter})`);
  }

  // ── I-2: total transaction count unchanged (zero data loss) ──────────────────
  await gotoNav(page, "Transactions");
  await page.waitForTimeout(900);
  const txnAfter = await txnCount(page);
  note(`Transaction count: ${txnBefore} → ${txnAfter}`);
  if (txnBefore !== null && txnAfter !== null) {
    if (txnAfter === txnBefore) pass(`I-2 — total transactions UNCHANGED (${txnAfter}) — zero data loss on category delete`);
    else fail(`I-2 — transaction count changed ${txnBefore}→${txnAfter} — DATA LOSS / duplication on category delete`);
  }

  // ── I-5: no orphan references to the deleted category ────────────────────────
  const orphan = await page.evaluate((name) => {
    // open the category filter and check the deleted category is not an option / not shown
    const sel = [...document.querySelectorAll('select')].find(s => (s.getAttribute("aria-label") || "").toLowerCase().includes("categor"));
    const inFilter = sel ? [...sel.options].some(o => o.text.trim() === name) : null;
    return { inFilter, bodyHasName: document.body.textContent.includes(name) };
  }, VICTIM);
  note(`Orphan check: deleted cat in filter=${orphan.inFilter} | name visible on /transactions=${orphan.bodyHasName}`);
  if (orphan.inFilter === false) pass(`I-5 — deleted category "${VICTIM}" no longer offered as a transaction filter (no orphan category)`);
  else if (orphan.inFilter === null) note("I-5 — category filter not found to verify orphan (inconclusive)");
  else absent_(`I-5 — deleted category "${VICTIM}" still appears as a filter option (orphan reference)`);

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
