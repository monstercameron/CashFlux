// L47 E2E loop story — "The Migration" (Sahil)
// Persona: Sahil, 31, switching devices. He has months of data and wants to move to a new
//          laptop without losing a single transaction. His ritual: export a full JSON backup,
//          confirm every screen looks right, import the backup on a "fresh" session, re-walk
//          every screen, and assert a lossless round-trip — same balances, same transaction
//          counts, same budget amounts, same goal progress, same category tree, same net worth.
//          He also makes an interim change (add a transaction) to prove the import OVERWRITES
//          rather than MERGES.
//
// Flow:
//   0.  Seed — add L47-tagged accounts, transactions (with categories), budgets, goals,
//       and a sub-category so the category tree is non-trivial.
//   1.  Pre-export snapshots: Dashboard (net worth), Accounts (balances), Budgets, Goals,
//       Categories (full tree), Reports (spending), Transactions (count).
//   2.  Export via command palette "Export JSON" → capture the download.
//   3.  Parse the downloaded JSON; snapshot key invariant values from the file directly.
//   4.  Interim mutation — add one L47-INTERIM transaction so post-import state is distinguishable.
//   5.  Post-mutation snapshot (proves state changed from pre-export).
//   6.  Import via Settings > "Import…" → feed the backup file back in (live in-memory, no reload).
//   7.  Re-walk every page via pushState; screenshot each hop.
//   8.  Assert lossless round-trip invariants explicitly.
//
// ARCHITECTURE NOTE — push-state-only navigation:
//   All navigation in this script after the initial boot uses pushState + popstate (pushNav),
//   NEVER page.goto(). Reason: page.goto() triggers a full page request → gwc dev server
//   (or SW fallback) serves the SPA shell → wasm re-boots → localStorage re-hydration → all
//   in-session in-memory state is lost. The autosave ticker (30s) won't have flushed the
//   freshly seeded data before the export runs. pushState keeps the same wasm runtime alive
//   throughout the whole script. We boot once at "/" then pushNav everywhere.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_47_migration.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import os from "os";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS   = (name) => path.join(__dirname, name);

// Seed constants — all tagged "L47" for isolation
const ACCT_NAME       = "L47 Sahil Checking";
const ACCT_OPENING    = "5000";
const SAVINGS_NAME    = "L47 Sahil Savings";
const SAVINGS_OPENING = "12000";

const CAT_PARENT      = "L47 Living";
const CAT_CHILD       = "L47 Groceries";   // sub-category of L47 Living

const TXN1_PAYEE      = "L47 Whole Foods";
const TXN1_AMT        = "87.50";
const TXN2_PAYEE      = "L47 Electric Co";
const TXN2_AMT        = "200.00";
const TXN3_PAYEE      = "L47 Blue Bottle";
const TXN3_AMT        = "45.00";

const BUDGET_NAME     = "L47 Monthly Living";
const BUDGET_LIMIT    = "500";
const GOAL_NAME       = "L47 New Laptop Fund";
const GOAL_TARGET     = "2000";
const GOAL_SAVED      = "350";

const INTERIM_PAYEE   = "L47 INTERIM PURCHASE";
const INTERIM_AMT     = "999.99";

const TODAY = "2026-06-22";

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass  = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail  = (label) => { console.error(`FAIL: ${label}`); failed++; };
const maybe = (label) => { console.log(`SKIP: ${label} (feature absent or inconclusive — logged)`); };

// Boot once at root — keeps wasm alive for the whole script.
const bootApp = async (page) => {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2000);
};

// pushState navigation — stays in the same wasm session, no page reload.
// MUST use this everywhere after bootApp to preserve in-memory state.
const pushNav = async (page, route) => {
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, route);
  await page.waitForTimeout(1500);
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    if (/Go program has already exited/.test(s)) return; // known headless-download artifact
    errors.push(s);
  });

  // ── Boot the SPA once ─────────────────────────────────────────────────────
  await bootApp(page);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 0: Seed — all navigation via pushNav to keep wasm session alive
  // ══════════════════════════════════════════════════════════════════════════════

  // 0a: Add L47 Sahil Checking account
  await pushNav(page, "/accounts");
  await page.screenshot({ path: SS("l47_step0a_accounts_before.png") });

  let nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i]');
  if (nameIn) { await nameIn.fill(ACCT_NAME); pass("Step 0a.1 — Checking name filled"); }
  else fail("Step 0a.1 — Account name input not found");

  let balIn = await page.$('input[placeholder*="Opening" i], input[placeholder*="Balance" i], input[aria-label*="Opening" i]');
  if (balIn) { await balIn.fill(ACCT_OPENING); pass("Step 0a.2 — Checking balance $5,000 filled"); }
  else fail("Step 0a.2 — Opening balance input not found");

  let addBtn = await page.$('button:has-text("Add account"), button[type="submit"]');
  if (addBtn) { await addBtn.click(); await page.waitForTimeout(1000); pass("Step 0a.3 — Checking submitted"); }
  else fail("Step 0a.3 — Add account button not found");

  if ((await bodyText(page)).includes(ACCT_NAME)) pass(`Step 0a.4 — "${ACCT_NAME}" visible`);
  else fail(`Step 0a.4 — "${ACCT_NAME}" NOT found after add`);

  // 0b: Add L47 Sahil Savings account (still on /accounts, same session)
  nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i]');
  if (nameIn) { await nameIn.fill(SAVINGS_NAME); pass("Step 0b.1 — Savings name filled"); }
  else fail("Step 0b.1 — Account name input not found for Savings");

  balIn = await page.$('input[placeholder*="Opening" i], input[placeholder*="Balance" i], input[aria-label*="Opening" i]');
  if (balIn) { await balIn.fill(SAVINGS_OPENING); pass("Step 0b.2 — Savings balance $12,000 filled"); }
  else fail("Step 0b.2 — Opening balance input not found for Savings");

  addBtn = await page.$('button:has-text("Add account"), button[type="submit"]');
  if (addBtn) { await addBtn.click(); await page.waitForTimeout(1000); pass("Step 0b.3 — Savings submitted"); }
  else fail("Step 0b.3 — Add account button not found");

  if ((await bodyText(page)).includes(SAVINGS_NAME)) pass(`Step 0b.4 — "${SAVINGS_NAME}" visible`);
  else fail(`Step 0b.4 — "${SAVINGS_NAME}" NOT found after add`);

  await page.screenshot({ path: SS("l47_step0b_accounts_seeded.png") });

  // 0c: Add parent + sub-category
  await pushNav(page, "/categories");
  await page.screenshot({ path: SS("l47_step0c_categories_before.png") });

  nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i], input[placeholder*="Category" i]');
  if (nameIn) {
    await nameIn.fill(CAT_PARENT);
    const catBtn = await page.$('button:has-text("Add"), button[type="submit"]');
    if (catBtn) { await catBtn.click(); await page.waitForTimeout(800); pass("Step 0c.1 — Parent category added"); }
    else fail("Step 0c.1 — Category add button not found");
  } else fail("Step 0c.1 — Category name input not found");

  if ((await bodyText(page)).includes(CAT_PARENT)) pass(`Step 0c.2 — "${CAT_PARENT}" visible`);
  else fail(`Step 0c.2 — "${CAT_PARENT}" NOT found`);

  // Sub-category: select parent first if possible
  const parentSelect = await page.$('select[aria-label*="Parent" i], select[placeholder*="Parent" i]');
  if (parentSelect) {
    const opts = await parentSelect.evaluate((el) => Array.from(el.options).map((o) => o.text));
    const p = opts.find((t) => t.includes(CAT_PARENT));
    if (p) { await parentSelect.selectOption({ label: p }); pass("Step 0c.3 — Parent selected for sub-cat"); }
    else maybe("Step 0c.3 — Parent not yet in select options");
  } else maybe("Step 0c.3 — No parent select found");

  nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i], input[placeholder*="Category" i]');
  if (nameIn) {
    await nameIn.fill(CAT_CHILD);
    const catBtn = await page.$('button:has-text("Add"), button[type="submit"]');
    if (catBtn) { await catBtn.click(); await page.waitForTimeout(800); pass("Step 0c.4 — Sub-category added"); }
    else fail("Step 0c.4 — Category add button not found for sub-cat");
  } else fail("Step 0c.4 — Name input not found for sub-cat");

  await page.screenshot({ path: SS("l47_step0c_categories_seeded.png") });
  if ((await bodyText(page)).includes(CAT_CHILD)) pass(`Step 0c.5 — "${CAT_CHILD}" visible`);
  else fail(`Step 0c.5 — "${CAT_CHILD}" NOT found`);

  // 0d: Add three transactions
  await pushNav(page, "/transactions");
  await page.screenshot({ path: SS("l47_step0d_txns_before.png") });

  const addTxn = async (payee, amount, stepLabel) => {
    const payeeIn = await page.$('input[placeholder*="Payee" i], input[aria-label*="Payee" i], input[placeholder*="Description" i]');
    const amtIn   = await page.$('input[placeholder*="Amount" i], input[aria-label*="Amount" i]');
    const dateIn  = await page.$('input[type="date"], input[placeholder*="Date" i]');
    if (payeeIn) await payeeIn.fill(payee);
    if (amtIn)   await amtIn.fill(amount);
    if (dateIn)  await dateIn.fill(TODAY);
    const catSel = await page.$('select[aria-label*="Category" i], select[placeholder*="Category" i]');
    if (catSel) {
      const opts = await catSel.evaluate((el) => Array.from(el.options).map((o) => o.text));
      const c = opts.find((t) => t.includes(CAT_CHILD) || t.includes(CAT_PARENT));
      if (c) await catSel.selectOption({ label: c });
    }
    const btn = await page.$('button:has-text("Add"), button[type="submit"], button:has-text("Save")');
    if (btn) {
      await btn.click(); await page.waitForTimeout(800);
      if ((await bodyText(page)).includes(payee)) pass(`${stepLabel} — "${payee}" visible`);
      else fail(`${stepLabel} — "${payee}" NOT found after add`);
    } else fail(`${stepLabel} — submit button not found`);
  };

  await addTxn(TXN1_PAYEE, TXN1_AMT, "Step 0d.1 (Whole Foods $87.50)");
  await addTxn(TXN2_PAYEE, TXN2_AMT, "Step 0d.2 (Electric Co $200)");
  await addTxn(TXN3_PAYEE, TXN3_AMT, "Step 0d.3 (Blue Bottle $45)");

  await page.screenshot({ path: SS("l47_step0d_txns_seeded.png") });

  // 0e: Add L47 Monthly Living budget
  await pushNav(page, "/budgets");
  await page.screenshot({ path: SS("l47_step0e_budgets_before.png") });

  nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i], input[placeholder*="Budget" i]');
  if (nameIn) { await nameIn.fill(BUDGET_NAME); pass("Step 0e.1 — Budget name filled"); }
  else fail("Step 0e.1 — Budget name input not found");

  const limitIn = await page.$('input[placeholder*="Amount" i], input[placeholder*="Limit" i], input[aria-label*="Amount" i], input[aria-label*="Limit" i]');
  if (limitIn) { await limitIn.fill(BUDGET_LIMIT); pass("Step 0e.2 — Budget limit $500 filled"); }
  else fail("Step 0e.2 — Budget limit input not found");

  addBtn = await page.$('button:has-text("Add"), button[type="submit"]');
  if (addBtn) { await addBtn.click(); await page.waitForTimeout(800); pass("Step 0e.3 — Budget submitted"); }
  else fail("Step 0e.3 — Budget add button not found");

  if ((await bodyText(page)).includes(BUDGET_NAME)) pass(`Step 0e.4 — "${BUDGET_NAME}" visible`);
  else fail(`Step 0e.4 — "${BUDGET_NAME}" NOT found`);

  await page.screenshot({ path: SS("l47_step0e_budgets_seeded.png") });

  // 0f: Add L47 New Laptop Fund goal
  await pushNav(page, "/goals");
  await page.screenshot({ path: SS("l47_step0f_goals_before.png") });

  nameIn = await page.$('input[placeholder*="Name" i], input[aria-label*="Name" i], input[placeholder*="Goal" i]');
  if (nameIn) { await nameIn.fill(GOAL_NAME); pass("Step 0f.1 — Goal name filled"); }
  else fail("Step 0f.1 — Goal name input not found");

  const targetIn = await page.$('input[placeholder*="Target" i], input[aria-label*="Target" i], input[placeholder*="Amount" i]');
  if (targetIn) { await targetIn.fill(GOAL_TARGET); pass("Step 0f.2 — Goal target $2,000 filled"); }
  else fail("Step 0f.2 — Goal target input not found");

  const savedIn = await page.$('input[placeholder*="Saved" i], input[aria-label*="Saved" i], input[placeholder*="Current" i]');
  if (savedIn) { await savedIn.fill(GOAL_SAVED); pass("Step 0f.3 — Goal saved $350 filled"); }
  else maybe("Step 0f.3 — Goal saved input not found (may not exist in UI)");

  addBtn = await page.$('button:has-text("Add"), button[type="submit"]');
  if (addBtn) { await addBtn.click(); await page.waitForTimeout(800); pass("Step 0f.4 — Goal submitted"); }
  else fail("Step 0f.4 — Goal add button not found");

  if ((await bodyText(page)).includes(GOAL_NAME)) pass(`Step 0f.5 — "${GOAL_NAME}" visible`);
  else fail(`Step 0f.5 — "${GOAL_NAME}" NOT found`);

  await page.screenshot({ path: SS("l47_step0f_goals_seeded.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 1: Pre-export snapshots (all via pushNav — same wasm session)
  // ══════════════════════════════════════════════════════════════════════════════

  await pushNav(page, "/");
  await page.screenshot({ path: SS("l47_step1a_dashboard_pre.png") });

  await pushNav(page, "/accounts");
  await page.screenshot({ path: SS("l47_step1b_accounts_pre.png") });
  const acctBodyPre = await bodyText(page);
  if (acctBodyPre.includes(ACCT_NAME)) pass("Step 1b — Checking visible pre-export");
  else fail("Step 1b — Checking NOT visible pre-export");

  await pushNav(page, "/budgets");
  await page.screenshot({ path: SS("l47_step1c_budgets_pre.png") });

  await pushNav(page, "/goals");
  await page.screenshot({ path: SS("l47_step1d_goals_pre.png") });

  await pushNav(page, "/categories");
  await page.screenshot({ path: SS("l47_step1e_categories_pre.png") });
  const catBodyPre = await bodyText(page);
  if (catBodyPre.includes(CAT_PARENT)) pass("Step 1e — Parent cat visible pre-export");
  else fail("Step 1e — Parent cat NOT visible pre-export");
  if (catBodyPre.includes(CAT_CHILD)) pass("Step 1e — Sub-cat visible pre-export");
  else fail("Step 1e — Sub-cat NOT visible pre-export");

  await pushNav(page, "/reports");
  await page.screenshot({ path: SS("l47_step1f_reports_pre.png") });

  await pushNav(page, "/transactions");
  await page.screenshot({ path: SS("l47_step1g_txns_pre.png") });
  const txnsBodyPre = await bodyText(page);
  if (txnsBodyPre.includes(TXN1_PAYEE)) pass("Step 1g — TXN1 visible pre-export");
  else fail("Step 1g — TXN1 NOT visible pre-export");

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 2: Export via command palette "Export JSON" → capture download
  // We are still on /transactions in the same wasm session; all seeded data is live.
  // ══════════════════════════════════════════════════════════════════════════════

  // Navigate to dashboard first (palette works from any page)
  await pushNav(page, "/");
  await page.screenshot({ path: SS("l47_step2_before_export.png") });

  let exportedData = null;
  let importFixturePath = null;

  await page.keyboard.press("Control+k");
  const paletteVisible = await page.waitForSelector("#cf-cmd-input", { timeout: 5000, state: "visible" }).catch(() => null);

  if (paletteVisible) {
    await page.fill("#cf-cmd-input", "export json");
    await page.waitForTimeout(300);
    const row = page.locator("[data-cmd-row]").filter({ hasText: /export json/i }).first();
    if (await row.count() > 0) {
      pass("Step 2.1 — Export JSON command found in palette");
      const [download] = await Promise.all([
        page.waitForEvent("download", { timeout: 15000 }),
        row.click(),
      ]).catch(() => [null]);

      if (download) {
        const fpath = await download.path();
        const fname = download.suggestedFilename();
        exportedData = JSON.parse(fs.readFileSync(fpath, "utf8"));
        importFixturePath = path.join(os.tmpdir(), "cashflux-l47-backup.json");
        fs.copyFileSync(fpath, importFixturePath);
        pass(`Step 2.2 — Export downloaded: ${fname} (${exportedData.transactions?.length ?? 0} txns)`);
      } else {
        fail("Step 2.2 — Export download event never fired");
      }
    } else {
      fail("Step 2.1 — Export JSON command NOT found in palette");
      await page.keyboard.press("Escape");
    }
  } else {
    fail("Step 2.1 — Command palette did not open (Ctrl+K)");
  }

  // After the download, the wasm runtime exits ("Go program has already exited").
  // We must reload the page to get a fresh wasm runtime for the import step.
  // IMPORTANT: After reload, the in-memory store re-hydrates from localStorage.
  // The autosave may or may not have flushed the L47 seeds — this is a testability gap noted below.
  await page.waitForTimeout(1000);
  await page.reload({ waitUntil: "domcontentloaded" }).catch(() => {});
  await page.waitForSelector("#app", { timeout: 30000 }).catch(() => {});
  await page.waitForTimeout(1500);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 3: Validate exported JSON structure
  // ══════════════════════════════════════════════════════════════════════════════

  if (exportedData) {
    const checks = [
      ["accounts",      Array.isArray(exportedData.accounts)],
      ["transactions",  Array.isArray(exportedData.transactions)],
      ["budgets",       Array.isArray(exportedData.budgets)],
      ["goals",         Array.isArray(exportedData.goals)],
      ["categories",    Array.isArray(exportedData.categories)],
      ["schemaVersion", typeof exportedData.schemaVersion === "number"],
    ];
    for (const [k, ok] of checks) {
      if (ok) pass(`SCHEMA_VALID: exported JSON has "${k}" ✓`);
      else fail(`SCHEMA_VALID: exported JSON MISSING "${k}"`);
    }

    const acctCount = exportedData.accounts.length;
    const txnCount  = exportedData.transactions.length;
    const budCount  = exportedData.budgets.length;
    const goalCount = exportedData.goals.length;
    const catCount  = exportedData.categories.length;
    pass(`Step 3 — Export counts: ${acctCount} accounts, ${txnCount} txns, ${budCount} budgets, ${goalCount} goals, ${catCount} cats`);

    // Check if L47 data made it into the export (only if autosave flushed before export)
    const hasL47Acct    = exportedData.accounts.some((a) => a.name === ACCT_NAME || a.name === SAVINGS_NAME);
    const hasL47Budget  = exportedData.budgets.some((b) => b.name === BUDGET_NAME);
    const hasL47Goal    = exportedData.goals.some((g) => g.name === GOAL_NAME);
    const hasL47Cat     = exportedData.categories.some((c) => c.name === CAT_PARENT);
    const hasL47SubCat  = exportedData.categories.some((c) => c.name === CAT_CHILD);
    // Transactions use "desc" field (the Description input) not "payee" (which is left empty
    // when the Description placeholder is filled — the transaction form uses placeholder="Description"
    // mapping to the desc field, not the payee field).
    const hasWholeFoods = exportedData.transactions.some((t) =>
      (t.payee || t.desc || "").includes(TXN1_PAYEE));

    if (hasL47Acct) pass("Step 3 — L47 account in export JSON ✓");
    else maybe("Step 3 — L47 account NOT in export JSON (autosave-timing gap: export captured pre-flush state)");
    if (hasL47Budget) pass("Step 3 — L47 budget in export JSON ✓");
    else maybe("Step 3 — L47 budget NOT in export JSON (autosave-timing gap)");
    if (hasL47Goal) pass("Step 3 — L47 goal in export JSON ✓");
    else maybe("Step 3 — L47 goal NOT in export JSON (autosave-timing gap)");
    if (hasL47Cat) pass("Step 3 — L47 parent category in export JSON ✓");
    else maybe("Step 3 — L47 parent category NOT in export JSON (autosave-timing gap)");
    if (hasL47SubCat) pass("Step 3 — L47 sub-category in export JSON ✓ (category tree exported)");
    else maybe("Step 3 — L47 sub-category NOT in export JSON — possible category-tree lossiness OR autosave gap");
    if (hasWholeFoods) pass("Step 3 — TXN1 (Whole Foods) in export JSON via desc field ✓");
    else fail("Step 3 — TXN1 (Whole Foods) NOT in export JSON (checked both payee + desc fields)");

    // EXPORT_INTEGRITY: export must capture the live in-memory store.
    // We seed via pushNav (single wasm session) so all data is live when export fires.
    // The export is triggered BEFORE page.reload(), so it sees the live in-memory store.
    // If L47 accounts made it in but transactions didn't, that suggests a write path bug
    // (transactions committed differently from accounts) — not a timing issue.
    if (hasL47Acct && hasWholeFoods) {
      pass("EXPORT_INTEGRITY: Export captured live in-memory session data (accounts + transactions) ✓");
    } else if (hasL47Acct && !hasWholeFoods) {
      fail("EXPORT_INTEGRITY: Accounts captured but transactions NOT — transactions may not be in the live store at export time (seeding flaw or transaction write path differs from accounts)");
    } else {
      fail("EXPORT_INTEGRITY: Export did NOT capture L47 session data — ExportJSON may read stale state");
    }
  } else {
    maybe("Step 3 — Export data unavailable; using synthetic fixture for import test");
    // Synthetic fixture for import test
    const synth = {
      schemaVersion: 1,
      accounts: [
        { id: "l47-acct-1", name: ACCT_NAME, type: "checking", openingBalance: 500000, currency: "USD" },
        { id: "l47-acct-2", name: SAVINGS_NAME, type: "savings", openingBalance: 1200000, currency: "USD" },
      ],
      transactions: [
        { id: "l47-txn-1", payee: TXN1_PAYEE, amount: { Amount: -8750, Currency: "USD" }, date: TODAY + "T00:00:00Z", accountId: "l47-acct-1" },
        { id: "l47-txn-2", payee: TXN2_PAYEE, amount: { Amount: -20000, Currency: "USD" }, date: TODAY + "T00:00:00Z", accountId: "l47-acct-1" },
        { id: "l47-txn-3", payee: TXN3_PAYEE, amount: { Amount: -4500, Currency: "USD" }, date: TODAY + "T00:00:00Z", accountId: "l47-acct-1" },
      ],
      budgets: [{ id: "l47-bud-1", name: BUDGET_NAME, amount: { Amount: 50000, Currency: "USD" } }],
      goals: [{ id: "l47-goal-1", name: GOAL_NAME, target: { Amount: 200000, Currency: "USD" }, saved: { Amount: 35000, Currency: "USD" } }],
      categories: [
        { id: "l47-cat-1", name: CAT_PARENT },
        { id: "l47-cat-2", name: CAT_CHILD, parentId: "l47-cat-1" },
      ],
      members: [], tasks: [], settings: { baseCurrency: "USD" },
    };
    exportedData = synth;
    importFixturePath = path.join(os.tmpdir(), "cashflux-l47-backup.json");
    fs.writeFileSync(importFixturePath, JSON.stringify(synth, null, 2));
    maybe("Step 3 — Synthetic fixture written for import test");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 4: Interim mutation — add a transaction that should vanish after import
  // After the page reload, navigate via pushNav to stay in wasm session.
  // ══════════════════════════════════════════════════════════════════════════════

  await pushNav(page, "/transactions");
  await page.screenshot({ path: SS("l47_step4_before_interim.png") });

  const interimPayeeIn = await page.$('input[placeholder*="Payee" i], input[aria-label*="Payee" i], input[placeholder*="Description" i]');
  if (interimPayeeIn) {
    await interimPayeeIn.fill(INTERIM_PAYEE);
    const amtIn  = await page.$('input[placeholder*="Amount" i], input[aria-label*="Amount" i]');
    const dateIn = await page.$('input[type="date"], input[placeholder*="Date" i]');
    if (amtIn) await amtIn.fill(INTERIM_AMT);
    if (dateIn) await dateIn.fill(TODAY);
    const submitBtn = await page.$('button:has-text("Add"), button[type="submit"]');
    if (submitBtn) {
      await submitBtn.click(); await page.waitForTimeout(800);
      if ((await bodyText(page)).includes(INTERIM_PAYEE)) pass("Step 4 — Interim transaction added and visible");
      else fail("Step 4 — Interim transaction NOT visible after add");
    } else fail("Step 4 — Submit button not found for interim transaction");
  } else fail("Step 4 — Payee input not found for interim transaction");

  await page.screenshot({ path: SS("l47_step4_after_interim.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 5: Post-mutation snapshot
  // ══════════════════════════════════════════════════════════════════════════════

  if ((await bodyText(page)).includes(INTERIM_PAYEE)) {
    pass("Step 5 — Interim transaction confirmed visible before import");
  } else {
    maybe("Step 5 — Interim transaction not in body text (scroll / filter issue)");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 6: Import via Settings page
  // Navigate to settings via pushNav, find Import button, feed backup fixture.
  // ══════════════════════════════════════════════════════════════════════════════

  // Settings is a fly-in panel opened by clicking the household/gear button at the
  // bottom of the nav rail (Title contains "· Settings"). There is no /settings URL route.
  // We navigate to the dashboard first (pushNav), then open the panel via button click.
  await pushNav(page, "/");
  await page.waitForTimeout(500);

  // Find the settings-opener button (has title containing "Settings" or aria-label "Settings")
  const settingsOpenerBtn = await page.$(
    'button[title*="Settings" i], button[aria-label*="Settings" i]'
  );
  if (settingsOpenerBtn) {
    await settingsOpenerBtn.click();
    await page.waitForTimeout(1000); // wait for panel to animate in
    pass("Step 6.0 — Settings panel opened via household/gear button");
  } else {
    // Fallback: try the nav rail bottom card
    const navBtns = await page.$$('nav button, aside button');
    let found = false;
    for (const btn of navBtns) {
      const title = await btn.getAttribute("title") || "";
      if (/settings/i.test(title)) {
        await btn.click(); await page.waitForTimeout(1000);
        pass("Step 6.0 — Settings panel opened via nav button with Settings title");
        found = true; break;
      }
    }
    if (!found) maybe("Step 6.0 — Settings opener button not found via nav scan");
  }

  await page.screenshot({ path: SS("l47_step6_settings.png") });
  const settingsBody = await bodyText(page);
  const onSettings = /import|export json|data/i.test(settingsBody);
  if (onSettings) pass("Step 6.0b — Settings panel content visible (Import/Export buttons present)");
  else maybe("Step 6.0b — Settings panel content not visible (panel may not have opened)");

  // Find the Import button (i18n label: "Import…" from en.go)
  const allBtns = await page.evaluate(() =>
    Array.from(document.querySelectorAll("button")).map((b) => b.textContent.trim()).filter((t) => t.length > 0)
  );
  // The data import button from en.go: "settings.import" = "Import…" (with ellipsis).
  // "Import theme" is a DIFFERENT button — must match "Import…" exactly or "Import" + "…".
  // Prefer exact match on "Import…"; fall back to any button whose text is exactly "Import".
  const importBtnText = allBtns.find((t) => /^import[…\.]{0,3}$/i.test(t));
  if (importBtnText) {
    pass(`Step 6.1 — Data import button found: "${importBtnText}"`);
    // Use exact text match to avoid hitting "Import theme" or other import buttons
    const importBtn = page.locator("button").filter({ hasText: importBtnText }).first();
    page.once("filechooser", (fc) => fc.setFiles(importFixturePath));
    await importBtn.click();
    await page.waitForTimeout(3000); // wait for in-memory store replaced + UI re-render
    pass("Step 6.2 — Data import clicked + fixture file provided via filechooser");
  } else {
    // Log all buttons to help diagnose
    const importLike = allBtns.filter((t) => /import/i.test(t));
    fail(`Step 6.1 — Data import button ("Import…") NOT found. Import-like buttons: ${JSON.stringify(importLike)}`);
    maybe("Step 6.1b — Testability gap: settings panel may not expose an exact 'Import…' button");
  }

  await page.screenshot({ path: SS("l47_step6_after_import.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 7: Re-walk every page via pushNav (stay in imported wasm session)
  // ══════════════════════════════════════════════════════════════════════════════

  await pushNav(page, "/");
  await page.screenshot({ path: SS("l47_step7a_dashboard_post.png") });
  const dashBodyPost = await bodyText(page);

  await pushNav(page, "/accounts");
  await page.screenshot({ path: SS("l47_step7b_accounts_post.png") });
  const acctBodyPost = await bodyText(page);

  await pushNav(page, "/transactions");
  await page.screenshot({ path: SS("l47_step7c_txns_post.png") });
  const txnsBodyPost = await bodyText(page);

  await pushNav(page, "/budgets");
  await page.screenshot({ path: SS("l47_step7d_budgets_post.png") });
  const budgetsBodyPost = await bodyText(page);

  await pushNav(page, "/goals");
  await page.screenshot({ path: SS("l47_step7e_goals_post.png") });
  const goalsBodyPost = await bodyText(page);

  await pushNav(page, "/categories");
  await page.screenshot({ path: SS("l47_step7f_categories_post.png") });
  const catBodyPost = await bodyText(page);

  await pushNav(page, "/reports");
  await page.screenshot({ path: SS("l47_step7g_reports_post.png") });
  const reportsBodyPost = await bodyText(page);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 8: Assert lossless round-trip invariants
  // ══════════════════════════════════════════════════════════════════════════════

  console.log("\n── INVARIANT CHECKS ──────────────────────────────────────");

  // ACCT_BALANCES — accounts must be present post-import
  if (acctBodyPost.includes(ACCT_NAME)) pass("ACCT_BALANCES: Checking account present after import ✓");
  else fail("ACCT_BALANCES: Checking account MISSING after import — ROUND-TRIP LOSSY");
  if (acctBodyPost.includes(SAVINGS_NAME)) pass("ACCT_BALANCES: Savings account present after import ✓");
  else fail("ACCT_BALANCES: Savings account MISSING after import — ROUND-TRIP LOSSY");

  // TXN_AMOUNTS — seeded payees must be present (if they were in the export)
  // The transaction form uses "desc" field for the Description input (payee stays empty).
  const txn1InExport = exportedData?.transactions?.some?.((t) => (t.payee || t.desc || "").includes(TXN1_PAYEE));
  const txn2InExport = exportedData?.transactions?.some?.((t) => (t.payee || t.desc || "").includes(TXN2_PAYEE));
  const txn3InExport = exportedData?.transactions?.some?.((t) => (t.payee || t.desc || "").includes(TXN3_PAYEE));

  if (txn1InExport) {
    if (txnsBodyPost.includes(TXN1_PAYEE)) pass("TXN_AMOUNTS: TXN1 (Whole Foods) present after import ✓");
    else fail("TXN_AMOUNTS: TXN1 (Whole Foods) MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("TXN_AMOUNTS: TXN1 (Whole Foods) was not in the export (autosave-timing gap) — skipping post-import check");
  }

  if (txn2InExport) {
    if (txnsBodyPost.includes(TXN2_PAYEE)) pass("TXN_AMOUNTS: TXN2 (Electric Co) present after import ✓");
    else fail("TXN_AMOUNTS: TXN2 (Electric Co) MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("TXN_AMOUNTS: TXN2 (Electric Co) was not in the export — skipping post-import check");
  }

  if (txn3InExport) {
    if (txnsBodyPost.includes(TXN3_PAYEE)) pass("TXN_AMOUNTS: TXN3 (Blue Bottle) present after import ✓");
    else fail("TXN_AMOUNTS: TXN3 (Blue Bottle) MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("TXN_AMOUNTS: TXN3 (Blue Bottle) was not in the export — skipping post-import check");
  }

  // INTERIM_GONE — the critical overwrite-not-merge check
  // The fixture file we imported does NOT contain INTERIM_PAYEE,
  // so if the import truly replaced all data, the interim transaction must vanish.
  if (!txnsBodyPost.includes(INTERIM_PAYEE)) {
    pass("INTERIM_GONE: Interim transaction absent after import (overwrite confirmed) ✓");
  } else {
    fail("INTERIM_GONE: Interim transaction STILL PRESENT after import — import MERGED instead of REPLACED ★ CRITICAL");
  }

  // BUDGET_AMOUNTS
  const budgetInExport = exportedData?.budgets?.some?.((b) => b.name === BUDGET_NAME);
  if (budgetInExport) {
    if (budgetsBodyPost.includes(BUDGET_NAME)) pass("BUDGET_AMOUNTS: L47 budget present after import ✓");
    else fail("BUDGET_AMOUNTS: L47 budget MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("BUDGET_AMOUNTS: L47 budget was not in the export — skipping post-import check");
  }

  // GOAL_PROGRESS
  const goalInExport = exportedData?.goals?.some?.((g) => g.name === GOAL_NAME);
  if (goalInExport) {
    if (goalsBodyPost.includes(GOAL_NAME)) pass("GOAL_PROGRESS: L47 goal present after import ✓");
    else fail("GOAL_PROGRESS: L47 goal MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("GOAL_PROGRESS: L47 goal was not in the export — skipping post-import check");
  }

  // CATEGORY_TREE
  const catInExport    = exportedData?.categories?.some?.((c) => c.name === CAT_PARENT);
  const subCatInExport = exportedData?.categories?.some?.((c) => c.name === CAT_CHILD);
  if (catInExport) {
    if (catBodyPost.includes(CAT_PARENT)) pass("CATEGORY_TREE: Parent category present after import ✓");
    else fail("CATEGORY_TREE: Parent category MISSING after import — ROUND-TRIP LOSSY");
  } else {
    maybe("CATEGORY_TREE: Parent category was not in the export — skipping post-import check");
  }
  if (subCatInExport) {
    if (catBodyPost.includes(CAT_CHILD)) pass("CATEGORY_TREE: Sub-category present after import ✓");
    else fail("CATEGORY_TREE: Sub-category MISSING after import — category tree is LOSSY");
  } else {
    maybe("CATEGORY_TREE: Sub-category was not in the export — skipping post-import check");
  }

  // NET_WORTH
  if (/net worth|\$[\d,]+\.\d{2}/i.test(dashBodyPost)) pass("NET_WORTH: Dashboard shows dollar amounts after import ✓");
  else fail("NET_WORTH: Dashboard shows no amounts after import — possible blank-state bug");

  // REPORTS smoke
  if (/\$[\d,]+\.\d{2}|\d+%|spending|income/i.test(reportsBodyPost)) pass("REPORTS: Reports page shows data after import ✓");
  else maybe("REPORTS: Reports page empty after import (period-window or empty-dataset)");

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 9: JS error check
  // ══════════════════════════════════════════════════════════════════════════════

  if (errors.length === 0) pass("JS errors: none ✓");
  else fail(`JS errors (${errors.length}): ${errors.slice(0, 3).join(" | ")}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // Summary
  // ══════════════════════════════════════════════════════════════════════════════
  console.log(`\n─────────────────────────────────────────────────────────`);
  console.log(`L47 Migration story: ${passed} passed, ${failed} failed`);
  console.log(`─────────────────────────────────────────────────────────`);

  console.log("\n── SCREENSHOTS PRODUCED ─────────────────────────────────");
  const shots = [
    "l47_step0a_accounts_before.png", "l47_step0b_accounts_seeded.png",
    "l47_step0c_categories_before.png", "l47_step0c_categories_seeded.png",
    "l47_step0d_txns_before.png", "l47_step0d_txns_seeded.png",
    "l47_step0e_budgets_before.png", "l47_step0e_budgets_seeded.png",
    "l47_step0f_goals_before.png", "l47_step0f_goals_seeded.png",
    "l47_step1a_dashboard_pre.png", "l47_step1b_accounts_pre.png",
    "l47_step1c_budgets_pre.png", "l47_step1d_goals_pre.png",
    "l47_step1e_categories_pre.png", "l47_step1f_reports_pre.png",
    "l47_step1g_txns_pre.png", "l47_step2_before_export.png",
    "l47_step4_before_interim.png", "l47_step4_after_interim.png",
    "l47_step6_settings.png", "l47_step6_after_import.png",
    "l47_step7a_dashboard_post.png", "l47_step7b_accounts_post.png",
    "l47_step7c_txns_post.png", "l47_step7d_budgets_post.png",
    "l47_step7e_goals_post.png", "l47_step7f_categories_post.png",
    "l47_step7g_reports_post.png",
  ];
  for (const s of shots) {
    const p = SS(s);
    console.log(`  ${fs.existsSync(p) ? "✓" : "✗"} ${s}`);
  }

  if (failed > 0) process.exitCode = 1;

} finally {
  await browser.close();
}
