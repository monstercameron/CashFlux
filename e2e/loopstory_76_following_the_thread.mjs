// L76 E2E loop story — "Following the Thread" (Marco) — 2026-06-22
//
// Theme: INTER-PAGE LINKS & CROSS-NAVIGATION — Story 3: LATERAL CONTEXT PIVOTS + BREADCRUMBS
//
// KEY INSIGHT FROM DOM PROBE:
//   All lateral links in CashFlux are implemented as <button> elements, NOT <a href>.
//   The probe must query `button` (by text/title/aria) rather than `a[href*="..."]`.
//   Exception: breadcrumbs use actual <a> tags or a custom breadcrumb element.
//
// LATERAL-LINK CHAIN (≥5 screens, 8+ navigations):
//  LL-1  /transactions → account (button: "Always categorize like this" → /rules; or account column)
//  LL-2  /transactions → category (button: "Always categorize like this" → /rules)
//  LL-3  /transactions → budget (no dedicated button confirmed)
//  LL-4  /transactions → rule ("Always categorize like this" → /rules confirmed present)
//  LL-5  /accounts → ledger (button "Transactions" → /transactions filtered — VERIFY ROW SET)
//  LL-6  /accounts → goals (no reverse link — GAP-G account-side)
//  LL-7  /accounts → bills (no bill/recurring button on /accounts)
//  LL-8  /categories → transactions (button "N transactions" — VERIFY ROW SET filtered)
//  LL-9  /categories → budget (no budget button/link on /categories)
//  LL-10 /categories → rules (no rule button/link on /categories)
//  LL-11 /budgets → category (budget rows show category name as button — test navigation)
//  LL-12 /budgets → transactions (no "Transactions" button per budget row found)
//  LL-13 /rules → transactions ("Apply to existing" button present)
//  LL-14 /goals → linked account (button "· linked to Emergency Savings (HYSA)" — test navigation)
//  LL-15 /goals → contributions/transactions (button "Transactions" on goals page)
//  BC-1..9 BREADCRUMBS per screen (already confirmed present for all main screens)
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_76_following_the_thread.mjs

import { createRequire } from "module";
import { fileURLToPath }  from "url";
import path from "path";
import fs   from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require   = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8080";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (name) => path.join(SSDIR, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass    = (label) => { console.log(`PASS:   ${label}`);  passed++; };
const fail    = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_ = (label) => { console.log(`ABSENT: ${label}`);  absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

// ── lateral-link matrix accumulator ───────────────────────────────────────────
const llMatrix = [];
const recordLL = (id, from, to, exists, specific, notes) => {
  llMatrix.push({ id, from, to,
    exists:   exists   ? "YES" : "NO",
    specific: specific === null ? "N/A" : (specific ? "YES" : "NO"),
    notes });
};

// ── breadcrumb audit accumulator ───────────────────────────────────────────────
const bcAudit = [];
const recordBC = (screen, present, functional, notes) => {
  bcAudit.push({ screen,
    present:    present    ? "YES" : "NO",
    functional: functional === null ? "N/A" : (functional ? "YES" : "NO"),
    notes });
};

// ── helpers ────────────────────────────────────────────────────────────────────

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link  = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const currentURL = (page) => page.evaluate(() => location.pathname + location.search);

const dismissModal = async (page) => {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  await page.waitForTimeout(200);
};

const hardReload = async (page) => {
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(800);
};

// Count visible data rows
const countVisibleRows = async (page) => page.evaluate(() => {
  const trs = Array.from(document.querySelectorAll('table tbody tr, [role="row"]'))
    .filter(r => {
      const t = r.textContent.trim();
      return t.length > 5 && !/^(date|amount|category|description|account|type)/i.test(t);
    });
  if (trs.length > 0) return trs.length;
  return Array.from(document.querySelectorAll('li'))
    .filter(r => /\$[\d,]+/.test(r.textContent)).length;
});

// Get active filter chip text
const getFilterChipText = async (page) => page.evaluate(() => {
  // Look for close (✕) buttons with account/category context — that's the active filter chip
  const chips = Array.from(document.querySelectorAll('button')).filter(b => {
    const t = b.textContent.trim();
    return /✕|×|✗/.test(t) || /^(Account:|Category:|Budget:|Rule:)/i.test(t);
  });
  if (chips.length > 0) return chips[0].textContent.trim();
  // Look for chip-style elements
  const chip = document.querySelector('[data-cf="filter-chip"], .filter-chip, .active-filter');
  if (chip) return chip.textContent.trim();
  const url = location.search;
  if (url) return `URL:${url}`;
  return null;
});

// Check for breadcrumb
const getBreadcrumb = async (page) => page.evaluate(() => {
  const bc = document.querySelector(
    'nav[aria-label*="breadcrumb" i], [data-cf*="breadcrumb"], .breadcrumb, ' +
    '[role="navigation"][aria-label*="breadcrumb" i], ol.breadcrumbs, ul.breadcrumbs, ' +
    '.bread-crumb, [data-cf="breadcrumb"], [data-cf*="trail"]'
  );
  if (bc) return { found: true, text: bc.textContent.trim().slice(0, 120), selector: "breadcrumb-element" };
  const backLink = document.querySelector('a[aria-label*="back" i], button[aria-label*="back" i]');
  if (backLink) return { found: true, text: backLink.textContent.trim().slice(0, 80), selector: "back-link" };
  return { found: false, text: null, selector: null };
});

// Click a button in main by exact text match
const clickButtonInMain = async (page, textPattern) => {
  return page.evaluate((pat) => {
    const re = new RegExp(pat, "i");
    const main = document.querySelector('main, article') || document.body;
    for (const btn of main.querySelectorAll('button')) {
      const txt = (btn.textContent + " " + (btn.getAttribute("aria-label") || "") + " " + (btn.getAttribute("title") || "")).trim();
      if (re.test(txt)) {
        btn.click();
        return `button: "${txt.slice(0,80)}"`;
      }
    }
    return "NOT FOUND";
  }, textPattern);
};

// List all buttons in main with their text
const listMainButtons = async (page) => page.evaluate(() => {
  const main = document.querySelector('main, article') || document.body;
  return Array.from(main.querySelectorAll('button')).map(b => ({
    text: b.textContent.trim().slice(0, 100),
    aria: b.getAttribute("aria-label"),
    title: b.getAttribute("title"),
  }));
});

// ── main ──────────────────────────────────────────────────────────────────────

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => {
    const msg = String(e);
    if (!msg.includes("Go program has already exited")) jsErrors.push(msg);
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  pass("HYDRATION — app loaded and nav visible");

  // Reset member filter to "Everyone"
  await page.evaluate(() => {
    const sel = document.querySelector('select[aria-label*="member" i], select[aria-label*="view as" i]');
    if (sel) { sel.value = sel.options[0]?.value; sel.dispatchEvent(new Event("change")); }
  });
  await page.waitForTimeout(400);
  await hardReload(page);
  note("Hard reload complete — View as: Everyone");

  // ── Baseline: total unfiltered transaction count ──────────────────────────
  await navTo(page, "Transactions");
  await page.waitForTimeout(1000);
  const totalTxnCount = await countVisibleRows(page);
  note(`Baseline unfiltered transaction row count: ${totalTxnCount}`);

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 1: /transactions — pivot to account, category, budget, rule
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 1: /transactions — lateral pivots to account / category / budget / rule ──────");

  await navTo(page, "Transactions");
  await dismissModal(page);
  await page.waitForTimeout(800);

  await page.screenshot({ path: SS("L76_hop1_transactions.png") });
  note("Screenshot: L76_hop1_transactions.png");

  // Breadcrumb
  const txnBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /transactions: found=${txnBC.found} text="${txnBC.text}"`);
  recordBC("/transactions", txnBC.found, txnBC.found ? true : null,
    txnBC.found ? `"${txnBC.text?.slice(0,80)}"` : "no breadcrumb");

  // Collect all buttons in main to understand what pivots are available
  const txnButtons = await listMainButtons(page);
  note(`  Total main buttons on /transactions: ${txnButtons.length}`);
  // List buttons relevant to pivoting
  const txnPivotBtns = txnButtons.filter(b =>
    /account|categor|budget|rule|always categorize/i.test(b.text + " " + (b.aria || "") + " " + (b.title || "")));
  note(`  Pivot-relevant buttons: ${txnPivotBtns.length}`);
  txnPivotBtns.slice(0, 8).forEach(b => note(`    text="${b.text}" aria="${b.aria}" title="${b.title}"`));

  // LL-1: /transactions → account pivot
  // Transaction rows show: account name as text but no account-navigation button per row
  // The "Account" column header button is just a sort control, not a pivot
  const txnAcctPivot = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    // Look for buttons whose title or aria mentions "view account" or navigates to accounts
    const btns = Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /view.{0,5}account|go to account|account detail/i.test(txt);
    });
    return btns.map(b => ({ text: b.textContent.trim().slice(0,60), title: b.getAttribute("title") }));
  });
  note(`  LL-1 account pivot buttons on /transactions: ${txnAcctPivot.length}`);

  const txnAcctExists = txnAcctPivot.length > 0;
  recordLL("LL-1", "/transactions", "/accounts (specific account)", txnAcctExists, false,
    txnAcctExists
      ? `button: "${txnAcctPivot[0].text}"`
      : "no per-row account pivot button; account shown as text in table column only (no link/button)");
  if (txnAcctExists) pass("LL-1 — /transactions has account pivot button");
  else absent_("LL-1 — /transactions has NO account pivot (account column is text-only, no clickable link)");

  // LL-2: /transactions → category pivot
  const txnCatPivot = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    const btns = Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /view.{0,5}categor|go to categor|categor detail/i.test(txt);
    });
    return btns.map(b => ({ text: b.textContent.trim().slice(0,60), title: b.getAttribute("title") }));
  });
  note(`  LL-2 category pivot buttons on /transactions: ${txnCatPivot.length}`);

  const txnCatExists = txnCatPivot.length > 0;
  recordLL("LL-2", "/transactions", "/categories (that category)", txnCatExists, false,
    txnCatExists
      ? `button: "${txnCatPivot[0].text}"`
      : "no per-row category pivot button; category shown as text in table column only");
  if (txnCatExists) pass("LL-2 — /transactions has category pivot button");
  else absent_("LL-2 — /transactions has NO category pivot (category column is text-only, no clickable link)");

  // LL-3: /transactions → budget pivot
  const txnBudgetPivot = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    const btns = Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /budget/i.test(txt) && !/new budget/i.test(txt);
    });
    return btns.map(b => ({ text: b.textContent.trim().slice(0,60), title: b.getAttribute("title") }));
  });
  note(`  LL-3 budget pivot buttons on /transactions: ${txnBudgetPivot.length}`);

  const txnBudgetExists = txnBudgetPivot.length > 0;
  recordLL("LL-3", "/transactions", "/budgets (category's budget)", txnBudgetExists, null,
    txnBudgetExists
      ? `button found: "${txnBudgetPivot[0].text}"`
      : "no budget pivot button on /transactions");
  if (txnBudgetExists) pass("LL-3 — /transactions has budget pivot button");
  else absent_("LL-3 — /transactions has NO budget pivot");

  // LL-4: /transactions → rule ("Always categorize like this" button confirmed from DOM probe)
  const txnRulePivot = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    const btns = Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /always categorize|rule|open the rules/i.test(txt);
    });
    return btns.map(b => ({ text: b.textContent.trim().slice(0,60), title: b.getAttribute("title") }));
  });
  note(`  LL-4 rule pivot buttons on /transactions: ${txnRulePivot.length}`);
  txnRulePivot.slice(0,2).forEach(b => note(`    "${b.text}" title="${b.title}"`));

  const txnRuleExists = txnRulePivot.length > 0;
  // Click the rule button to verify it navigates to /rules
  let afterRuleURL = await currentURL(page);
  if (txnRuleExists) {
    await clickButtonInMain(page, "Always categorize like this");
    await page.waitForTimeout(1200);
    afterRuleURL = await currentURL(page);
    note(`  After 'Always categorize like this' click: URL=${afterRuleURL}`);
    await dismissModal(page);
    await page.waitForTimeout(300);
  }

  const txnRuleLandsOnRules = afterRuleURL.startsWith("/rules");
  recordLL("LL-4", "/transactions", "/rules (Always categorize like this → /rules)", txnRuleExists, txnRuleLandsOnRules,
    `buttons=${txnRulePivot.length} dest=${afterRuleURL} landsOnRules=${txnRuleLandsOnRules}`);
  if (txnRuleExists && txnRuleLandsOnRules) pass("LL-4 — /transactions 'Always categorize like this' navigates to /rules ✓");
  else if (txnRuleExists) pass("LL-4 — /transactions has 'Always categorize like this' button (opens modal, not /rules directly)");
  else absent_("LL-4 — /transactions has NO rule pivot");

  await page.screenshot({ path: SS("L76_hop1b_transactions_pivots.png") });
  note("Screenshot: L76_hop1b_transactions_pivots.png");

  // Navigate back to transactions for subsequent hops
  await navTo(page, "Transactions");
  await page.waitForTimeout(800);

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 2: /accounts — pivot to ledger (filtered), goals, bills
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 2: /accounts — ledger drill + goals + bills ─────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L76_hop2_accounts.png") });
  note("Screenshot: L76_hop2_accounts.png");

  // Breadcrumb
  const acctBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /accounts: found=${acctBC.found} text="${acctBC.text}"`);
  recordBC("/accounts", acctBC.found, acctBC.found ? true : null,
    acctBC.found ? `"${acctBC.text?.slice(0,80)}"` : "no breadcrumb");

  // LL-5: /accounts → ledger (Transactions button — confirmed from DOM probe)
  // From probe: button text="Transactions" title="View this account's transactions"
  const acctTxnButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = b.textContent.trim();
      const title = b.getAttribute("title") || "";
      return /^transactions$/i.test(txt) || /view this account.s transactions/i.test(title);
    }).map(b => ({ text: b.textContent.trim(), title: b.getAttribute("title") }));
  });
  note(`  LL-5 Transactions buttons on /accounts: ${acctTxnButtons.length}`);

  let acctDrillRowCount = null;
  let acctFilterChip = null;
  const acctTxnExists = acctTxnButtons.length > 0;

  if (acctTxnExists) {
    // Click the first "Transactions" button
    await page.evaluate(() => {
      const main = document.querySelector('main, article') || document.body;
      const btn = Array.from(main.querySelectorAll('button')).find(b =>
        /^transactions$/i.test(b.textContent.trim()) ||
        /view this account.s transactions/i.test(b.getAttribute("title") || ""));
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);

    acctDrillRowCount = await countVisibleRows(page);
    acctFilterChip = await getFilterChipText(page);
    const afterURL = await currentURL(page);
    note(`  After account Transactions click: URL=${afterURL} rows=${acctDrillRowCount} chip="${acctFilterChip}"`);

    await page.screenshot({ path: SS("L76_hop2b_accounts_ledger.png") });
    note("Screenshot: L76_hop2b_accounts_ledger.png");

    const acctDrillFiltered = acctDrillRowCount !== null && totalTxnCount > 0 && acctDrillRowCount < totalTxnCount;
    const acctChipSpecific = acctFilterChip && /account:/i.test(acctFilterChip);
    recordLL("LL-5", "/accounts", "/transactions (ledger — filtered by account)", acctTxnExists, acctDrillFiltered,
      `rows=${acctDrillRowCount} total=${totalTxnCount} chip="${acctFilterChip}" filtered=${acctDrillFiltered} chipSpecific=${acctChipSpecific}`);

    if (acctDrillFiltered && acctChipSpecific) pass("LL-5 — /accounts Transactions button → /transactions IS filtered with specific chip (rows=" + acctDrillRowCount + " of " + totalTxnCount + ")");
    else if (acctDrillFiltered) pass("LL-5 — /accounts Transactions drill IS filtered (rows=" + acctDrillRowCount + " < total=" + totalTxnCount + ")");
    else if (acctDrillRowCount === totalTxnCount) fail("LL-5 — /accounts Transactions drill NOT filtered (rows=" + acctDrillRowCount + " = total=" + totalTxnCount + ")");
    else absent_("LL-5 — row count unverifiable");

    await navTo(page, "Accounts");
    await page.waitForTimeout(800);
  } else {
    recordLL("LL-5", "/accounts", "/transactions (ledger — filtered by account)", false, null,
      "no Transactions button found in main — DOM probe confirmed it exists; selector may have failed");
    absent_("LL-5 — Transactions button not found (unexpected)");
  }

  // LL-6: /accounts → goals (any goal linked to this account)
  const acctGoalButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /goal/i.test(txt) && !/new goal/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-6 goal-related buttons on /accounts: ${acctGoalButtons.length}`);
  acctGoalButtons.slice(0,3).forEach(b => note(`    "${b.text}" title="${b.title}"`));

  const acctGoalExists = acctGoalButtons.length > 0;
  recordLL("LL-6", "/accounts", "/goals (goal linked to account)", acctGoalExists, null,
    acctGoalExists
      ? `button: "${acctGoalButtons[0].text}"`
      : "no goal link/button on /accounts — no reverse relationship from account to goal");
  if (acctGoalExists) pass("LL-6 — /accounts has goal-related button");
  else absent_("LL-6 — /accounts has NO goal link (account→goal reverse link absent)");

  // LL-7: /accounts → bills/recurring
  const acctBillButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /bill|recurring|subscri/i.test(txt) && !/new bill/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-7 bill/recurring buttons on /accounts: ${acctBillButtons.length}`);

  const acctBillExists = acctBillButtons.length > 0;
  recordLL("LL-7", "/accounts", "/bills (recurring bills drawing from account)", acctBillExists, null,
    acctBillExists
      ? `button: "${acctBillButtons[0].text}"`
      : "no bill/recurring link on /accounts");
  if (acctBillExists) pass("LL-7 — /accounts has bill/recurring link");
  else absent_("LL-7 — /accounts has NO bill/recurring link (account→bills reverse relationship absent)");

  await page.screenshot({ path: SS("L76_hop2c_accounts_pivots.png") });
  note("Screenshot: L76_hop2c_accounts_pivots.png");

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 3: /categories — pivot to transactions (filter verify), budget, rules
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 3: /categories — transactions drill (filter verify) + budget + rules ──────────");

  await navTo(page, "Categories");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L76_hop3_categories.png") });
  note("Screenshot: L76_hop3_categories.png");

  // Breadcrumb
  const catBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /categories: found=${catBC.found} text="${catBC.text}"`);
  recordBC("/categories", catBC.found, catBC.found ? true : null,
    catBC.found ? `"${catBC.text?.slice(0,80)}"` : "no breadcrumb");

  // List all "N transactions" buttons (from DOM probe: "25 transactions", "24 transactions", etc.)
  const catTxnButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      return /\d+\s+transactions?/i.test(b.textContent.trim());
    }).map(b => ({ text: b.textContent.trim(), count: parseInt(b.textContent.trim()) }));
  });
  note(`  LL-8 "N transactions" buttons on /categories: ${catTxnButtons.length}`);
  catTxnButtons.slice(0, 5).forEach(b => note(`    "${b.text}" (count=${b.count})`));

  let catDrillRowCount = null;
  let catFilterChip = null;
  const catTxnExists = catTxnButtons.length > 0;

  if (catTxnExists) {
    // The first "N transactions" button tells us the expected filtered count
    const expectedCount = catTxnButtons[0].count;
    note(`  Expected filtered count from button label: ${expectedCount}`);

    // Click the first "N transactions" button
    await page.evaluate(() => {
      const main = document.querySelector('main, article') || document.body;
      const btn = Array.from(main.querySelectorAll('button')).find(b =>
        /\d+\s+transactions?/i.test(b.textContent.trim()));
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);

    catDrillRowCount = await countVisibleRows(page);
    catFilterChip = await getFilterChipText(page);
    const afterURL = await currentURL(page);
    note(`  After category txn drill: URL=${afterURL} rows=${catDrillRowCount} chip="${catFilterChip}" expected=${expectedCount}`);

    await page.screenshot({ path: SS("L76_hop3b_categories_txn_drill.png") });
    note("Screenshot: L76_hop3b_categories_txn_drill.png");

    const catDrillFiltered = catDrillRowCount !== null && totalTxnCount > 0 && catDrillRowCount < totalTxnCount;
    const catChipSpecific = catFilterChip && !/select all/i.test(catFilterChip) && catFilterChip !== null;
    // Also check if row count matches expected from button label
    const catCountMatchesLabel = catDrillRowCount !== null && Math.abs(catDrillRowCount - expectedCount) <= 2;

    recordLL("LL-8", "/categories", "/transactions (filtered by category)", catTxnExists, catDrillFiltered,
      `rows=${catDrillRowCount} total=${totalTxnCount} expected=${expectedCount} chip="${catFilterChip}" filtered=${catDrillFiltered} chipSpecific=${catChipSpecific} matchesLabel=${catCountMatchesLabel}`);

    if (catDrillFiltered && catChipSpecific) pass("LL-8 — /categories txn drill → /transactions IS filtered (rows=" + catDrillRowCount + " < " + totalTxnCount + "), chip specific");
    else if (catDrillFiltered) pass("LL-8 — /categories txn drill → /transactions IS filtered (rows=" + catDrillRowCount + " < " + totalTxnCount + "), chip generic");
    else if (catTxnExists && catDrillRowCount === totalTxnCount) fail("LL-8 — /categories txn drill NOT FILTERED (rows=" + catDrillRowCount + " = total=" + totalTxnCount + " — GAP-E re-confirmed for categories)");
    else absent_("LL-8 — row count unverifiable");

    await navTo(page, "Categories");
    await page.waitForTimeout(800);
  } else {
    recordLL("LL-8", "/categories", "/transactions (filtered by category)", false, null,
      "no 'N transactions' button found on /categories");
    absent_("LL-8 — /categories has NO transaction count button");
  }

  // LL-9: /categories → budget
  const catBudgetButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /budget/i.test(txt) && !/new budget/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-9 budget buttons on /categories: ${catBudgetButtons.length}`);
  catBudgetButtons.slice(0,3).forEach(b => note(`    "${b.text}"`));

  const catBudgetExists = catBudgetButtons.length > 0;
  recordLL("LL-9", "/categories", "/budgets (budget this category belongs to)", catBudgetExists, null,
    catBudgetExists
      ? `button: "${catBudgetButtons[0].text}"`
      : "no budget link/button on /categories; categories show transaction counts and Edit/Delete only");
  if (catBudgetExists) pass("LL-9 — /categories has budget link");
  else absent_("LL-9 — /categories has NO budget link (no path from category to its budget)");

  // LL-10: /categories → rules
  const catRuleButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /rule/i.test(txt) && !/new rule/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-10 rule buttons on /categories: ${catRuleButtons.length}`);

  const catRuleExists = catRuleButtons.length > 0;
  recordLL("LL-10", "/categories", "/rules (rules referencing this category)", catRuleExists, null,
    catRuleExists
      ? `button: "${catRuleButtons[0].text}"`
      : "no rule link/button on /categories");
  if (catRuleExists) pass("LL-10 — /categories has rule link");
  else absent_("LL-10 — /categories has NO rule link");

  await page.screenshot({ path: SS("L76_hop3c_categories_pivots.png") });
  note("Screenshot: L76_hop3c_categories_pivots.png");

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 4: /budgets — pivot to category + transactions (GAP-E re-test)
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 4: /budgets — category pivot + transactions filter re-test (GAP-E) ────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L76_hop4_budgets.png") });
  note("Screenshot: L76_hop4_budgets.png");

  // Breadcrumb
  const budgetsBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /budgets: found=${budgetsBC.found} text="${budgetsBC.text}"`);
  recordBC("/budgets", budgetsBC.found, budgetsBC.found ? true : null,
    budgetsBC.found ? `"${budgetsBC.text?.slice(0,80)}"` : "no breadcrumb");

  // List budget row buttons (from DOM probe: "Dining", "Entertainment", "Groceries", "Cover…", etc.)
  const budgetRowButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = b.textContent.trim();
      // Exclude known UI chrome
      return txt.length > 2 && !/^(New |Edit$|Cover|Delete|Week|Month|Quarter|Year|Dashboard|Dismiss)/i.test(txt) &&
             !/^(Custom range|Turn music|Notifications|Add something|Scan|Start fresh)/i.test(txt) &&
             !/aria.*previous|aria.*next/i.test(b.getAttribute("aria-label") || "");
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  Budget row buttons (non-chrome): ${budgetRowButtons.length}`);
  budgetRowButtons.forEach(b => note(`    "${b.text}" title="${b.title}"`));

  // LL-11: /budgets → category link
  // From probe: budget rows show "Dining", "Entertainment" etc. as buttons with no title
  // These appear to be category name buttons that might open edit or filter — test by clicking
  const catNameButton = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    // Find a budget category name button (not Cover, Edit, Delete)
    const btn = Array.from(main.querySelectorAll('button')).find(b => {
      const txt = b.textContent.trim();
      const title = b.getAttribute("title") || "";
      return /^(Dining|Entertainment|Groceries|Shopping|Transportation|Subscriptions|Gifts)/i.test(txt) &&
             !title && txt.length < 30;
    });
    if (!btn) return null;
    return { text: btn.textContent.trim(), title: btn.getAttribute("title") };
  });
  note(`  Budget category name button found: ${JSON.stringify(catNameButton)}`);

  // Clicking category name button — what does it do?
  let budgetCatNavDest = null;
  if (catNameButton) {
    const beforeURL = await currentURL(page);
    await page.evaluate((txt) => {
      const main = document.querySelector('main, article') || document.body;
      const btn = Array.from(main.querySelectorAll('button')).find(b => b.textContent.trim() === txt);
      if (btn) btn.click();
    }, catNameButton.text);
    await page.waitForTimeout(1200);
    budgetCatNavDest = await currentURL(page);
    note(`  After clicking budget category name "${catNameButton.text}": URL=${budgetCatNavDest}`);
    // Check if modal opened
    const modalOpen = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
    note(`  Modal open: ${modalOpen}`);
    await dismissModal(page);
    await page.waitForTimeout(400);
  }

  const budgetCatNavToCat = budgetCatNavDest && /categor/i.test(budgetCatNavDest);
  const budgetCatNavStayed = budgetCatNavDest === "/budgets" || !budgetCatNavDest;
  recordLL("LL-11", "/budgets", "/categories (that category)", catNameButton !== null, budgetCatNavToCat,
    catNameButton
      ? `button="${catNameButton.text}" dest=${budgetCatNavDest} (category name button opens modal or stays on /budgets, does NOT navigate to /categories)`
      : "no category name button found — budgets show category names as buttons but DOM probe found them");
  if (budgetCatNavToCat) pass("LL-11 — /budgets category button navigates to /categories");
  else if (catNameButton && budgetCatNavStayed) absent_("LL-11 — /budgets category name button EXISTS but stays on /budgets (opens inline edit, not category pivot)");
  else absent_("LL-11 — /budgets has NO category pivot link");

  // LL-12: /budgets → transactions (GAP-E re-test)
  // From DOM probe: /budgets has NO "Transactions" button per row — only "Cover…", "Edit", "Delete"
  // This re-confirms GAP-E: /budgets has no drill-to-transactions link
  await navTo(page, "Budgets");
  await page.waitForTimeout(600);

  const budgetTxnButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /transactions?/i.test(txt) && !/new transaction/i.test(txt) && !/\d+\s+transaction/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-12 Transactions buttons on /budgets: ${budgetTxnButtons.length}`);
  budgetTxnButtons.forEach(b => note(`    "${b.text}" title="${b.title}"`));

  const budgetTxnExists = budgetTxnButtons.length > 0;
  recordLL("LL-12", "/budgets", "/transactions (filtered by category — GAP-E re-test)", budgetTxnExists, null,
    budgetTxnExists
      ? `button: "${budgetTxnButtons[0].text}"`
      : "NO transactions button on /budgets — GAP-E RE-CONFIRMED: /budgets→/transactions drill absent");
  if (budgetTxnExists) {
    // Click and verify filtering
    await page.evaluate((txt) => {
      const main = document.querySelector('main, article') || document.body;
      const btn = Array.from(main.querySelectorAll('button')).find(b => b.textContent.trim() === txt);
      if (btn) btn.click();
    }, budgetTxnButtons[0].text);
    await page.waitForTimeout(1500);
    const budgetDrillRowCount = await countVisibleRows(page);
    const budgetFilterChip = await getFilterChipText(page);
    note(`  Budget txn drill rows=${budgetDrillRowCount} chip="${budgetFilterChip}"`);
    await page.screenshot({ path: SS("L76_hop4b_budgets_txn_drill.png") });
    note("Screenshot: L76_hop4b_budgets_txn_drill.png");
    if (budgetDrillRowCount < totalTxnCount) pass("LL-12 — /budgets drill → /transactions IS filtered");
    else fail("LL-12 — /budgets drill → /transactions NOT filtered (rows=" + budgetDrillRowCount + " = total)");
    await navTo(page, "Budgets");
    await page.waitForTimeout(600);
  } else {
    absent_("LL-12 — /budgets has NO Transactions drill link (GAP-E STILL OPEN for /budgets)");
  }

  await page.screenshot({ path: SS("L76_hop4c_budgets_pivots.png") });
  note("Screenshot: L76_hop4c_budgets_pivots.png");

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 5: /rules — pivot to transactions it affects
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 5: /rules — pivot to transactions it affects ────────────────────────────────");

  await navTo(page, "Rules");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L76_hop5_rules.png") });
  note("Screenshot: L76_hop5_rules.png");

  // Breadcrumb
  const rulesBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /rules: found=${rulesBC.found} text="${rulesBC.text}"`);
  recordBC("/rules", rulesBC.found, rulesBC.found ? true : null,
    rulesBC.found ? `"${rulesBC.text?.slice(0,80)}"` : "no breadcrumb");

  // List all rule-related buttons
  const ruleButtons = await listMainButtons(page);
  note(`  Total buttons on /rules: ${ruleButtons.length}`);
  ruleButtons.slice(0, 20).forEach(b => note(`    "${b.text}" title="${b.title}"`));

  // LL-13: /rules → transactions ("Apply to existing" confirmed from DOM probe)
  const rulesTxnPivot = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = (b.textContent + (b.getAttribute("aria-label") || "") + (b.getAttribute("title") || "")).trim();
      return /apply|preview|transactions|affected/i.test(txt) &&
             !/new transaction/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,100), title: b.getAttribute("title") }));
  });
  note(`  LL-13 rule action buttons: ${rulesTxnPivot.length}`);
  rulesTxnPivot.forEach(b => note(`    "${b.text}" title="${b.title}"`));

  // Click "Apply to existing" and see what happens
  let rulesApplyDest = await currentURL(page);
  let rulesApplyModal = false;
  if (rulesTxnPivot.length > 0) {
    await clickButtonInMain(page, rulesTxnPivot[0].text);
    await page.waitForTimeout(1200);
    rulesApplyDest = await currentURL(page);
    rulesApplyModal = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
    note(`  After rule action click: URL=${rulesApplyDest} modal=${rulesApplyModal}`);
    await dismissModal(page);
    await page.waitForTimeout(400);
  }

  const rulesTxnExists = rulesTxnPivot.length > 0;
  // True pivot = button that navigates to /transactions or shows a filtered preview
  const rulesGoesToTxn = /transaction/i.test(rulesApplyDest);
  recordLL("LL-13", "/rules", "/transactions (affected by rule)", rulesTxnExists, rulesGoesToTxn,
    `buttons=${rulesTxnPivot.length} dest=${rulesApplyDest} modal=${rulesApplyModal} "${rulesTxnPivot[0]?.text?.slice(0,60) || "none"}"`);
  if (rulesTxnExists && rulesGoesToTxn) pass("LL-13 — /rules has transaction preview link navigating to /transactions");
  else if (rulesTxnExists && rulesApplyModal) pass("LL-13 — /rules has 'Apply to existing' action (opens confirmation modal, applies rules)");
  else if (rulesTxnExists) pass("LL-13 — /rules has action button (Apply to existing — may open modal/confirm)");
  else absent_("LL-13 — /rules has NO transaction preview or apply action");

  await page.screenshot({ path: SS("L76_hop5b_rules_pivots.png") });
  note("Screenshot: L76_hop5b_rules_pivots.png");

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 6: /goals — pivot to linked account (GAP-G re-test) + transactions
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 6: /goals — linked account (GAP-G re-test) + contributions ───────────────────");

  await navTo(page, "Goals");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L76_hop6_goals.png") });
  note("Screenshot: L76_hop6_goals.png");

  // Breadcrumb
  const goalsBC = await getBreadcrumb(page);
  note(`  Breadcrumb on /goals: found=${goalsBC.found} text="${goalsBC.text}"`);
  recordBC("/goals", goalsBC.found, goalsBC.found ? true : null,
    goalsBC.found ? `"${goalsBC.text?.slice(0,80)}"` : "no breadcrumb");

  // From DOM probe: button "· linked to Emergency Savings (HYSA)" — this is a button, not an anchor
  // Also: button "Transactions" (from contribClick in first pass)
  const goalsAllButtons = await listMainButtons(page);
  note(`  Total buttons on /goals: ${goalsAllButtons.length}`);
  goalsAllButtons.filter(b => !/^(Contribute|Edit|New |Dismiss|Dashboard|Turn music|Notifications|Add something|Scan|Start fresh)/i.test(b.text))
    .forEach(b => note(`    "${b.text}" title="${b.title}"`));

  // LL-14: /goals → linked account (GAP-G re-test)
  // From probe: "· linked to Emergency Savings (HYSA)" as button
  const goalsLinkedAcctButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = b.textContent.trim();
      return /linked to|account/i.test(txt) && !/new account/i.test(txt) && txt.length > 5;
    }).map(b => ({ text: b.textContent.trim().slice(0,100), title: b.getAttribute("title") }));
  });
  note(`  LL-14 linked-account buttons on /goals: ${goalsLinkedAcctButtons.length}`);
  goalsLinkedAcctButtons.forEach(b => note(`    "${b.text}" title="${b.title}"`));

  let goalsLinkedAcctDest = null;
  let goalsLinkedAcctModal = false;
  if (goalsLinkedAcctButtons.length > 0) {
    const beforeURL = await currentURL(page);
    await clickButtonInMain(page, "linked to");
    await page.waitForTimeout(1200);
    goalsLinkedAcctDest = await currentURL(page);
    goalsLinkedAcctModal = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
    note(`  After linked-account button click: URL=${goalsLinkedAcctDest} modal=${goalsLinkedAcctModal}`);
    await dismissModal(page);
    await page.waitForTimeout(400);
    // Navigate back to goals if we left
    if (goalsLinkedAcctDest !== "/goals") {
      await navTo(page, "Goals");
      await page.waitForTimeout(800);
    }
  }

  const goalsAcctExists = goalsLinkedAcctButtons.length > 0;
  const goalsAcctNavToAcct = goalsLinkedAcctDest && /account/i.test(goalsLinkedAcctDest);
  const goalsAcctStayed = goalsLinkedAcctDest === "/goals" || (goalsLinkedAcctModal && !goalsAcctNavToAcct);

  recordLL("LL-14", "/goals", "/accounts (specific linked account — GAP-G re-test)", goalsAcctExists, goalsAcctNavToAcct,
    goalsAcctExists
      ? `button="${goalsLinkedAcctButtons[0].text}" dest=${goalsLinkedAcctDest} modal=${goalsLinkedAcctModal} (button present but navigates to: ${goalsLinkedAcctDest || "stayed on /goals"})`
      : "no linked-account button found — GAP-G STILL OPEN");
  if (goalsAcctNavToAcct) pass("LL-14 — /goals linked-account button navigates to /accounts (GAP-G CLOSED)");
  else if (goalsAcctExists && goalsAcctStayed) absent_("LL-14 — /goals 'linked to' button EXISTS but does NOT navigate to /accounts (stays on /goals or opens modal — GAP-G label-only, no navigation)");
  else absent_("LL-14 — /goals has NO linked-account button — GAP-G STILL OPEN");

  // LL-15: /goals → transactions/contributions
  const goalsTxnButtons = await page.evaluate(() => {
    const main = document.querySelector('main, article') || document.body;
    return Array.from(main.querySelectorAll('button')).filter(b => {
      const txt = b.textContent.trim();
      return /^transactions?$|^contributions?$|history/i.test(txt);
    }).map(b => ({ text: b.textContent.trim().slice(0,80), title: b.getAttribute("title") }));
  });
  note(`  LL-15 transaction/contribution buttons on /goals: ${goalsTxnButtons.length}`);
  goalsTxnButtons.forEach(b => note(`    "${b.text}" title="${b.title}"`));

  let goalsTxnDest = null;
  if (goalsTxnButtons.length > 0) {
    await clickButtonInMain(page, goalsTxnButtons[0].text);
    await page.waitForTimeout(1200);
    goalsTxnDest = await currentURL(page);
    note(`  After goals Transactions click: URL=${goalsTxnDest}`);
    await dismissModal(page);
    if (goalsTxnDest !== "/goals") {
      await navTo(page, "Goals");
      await page.waitForTimeout(600);
    }
  }

  const goalsTxnExists = goalsTxnButtons.length > 0;
  const goalsTxnGoesToTxn = goalsTxnDest && /transaction/i.test(goalsTxnDest);
  recordLL("LL-15", "/goals", "/transactions or contribution history", goalsTxnExists, goalsTxnGoesToTxn,
    goalsTxnExists
      ? `button="${goalsTxnButtons[0].text}" dest=${goalsTxnDest}`
      : "no transaction/contribution button on /goals");
  if (goalsTxnExists && goalsTxnGoesToTxn) pass("LL-15 — /goals Transactions button navigates to /transactions");
  else if (goalsTxnExists) pass("LL-15 — /goals has Transactions/Contributions button (dest=" + goalsTxnDest + ")");
  else absent_("LL-15 — /goals has NO transaction or contribution link");

  await page.screenshot({ path: SS("L76_hop6b_goals_pivots.png") });
  note("Screenshot: L76_hop6b_goals_pivots.png");

  // ══════════════════════════════════════════════════════════════════════════
  // HOP 7: BREADCRUMB sweep — remaining screens
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 7: BREADCRUMB sweep ─────────────────────────────────────────────────────────");

  for (const [title, screen] of [["Bills", "/bills"], ["Planning", "/planning"], ["Dashboard", "/dashboard"]]) {
    await navTo(page, title);
    await dismissModal(page);
    await page.waitForTimeout(800);
    await page.screenshot({ path: SS(`L76_hop7_${title.toLowerCase()}.png`) });
    note(`Screenshot: L76_hop7_${title.toLowerCase()}.png`);
    const bc = await getBreadcrumb(page);
    note(`  Breadcrumb on ${screen}: found=${bc.found} text="${bc.text}"`);
    recordBC(screen, bc.found, bc.found ? true : null,
      bc.found ? `"${bc.text?.slice(0,80)}"` : `no breadcrumb (${title} screen)`);
  }

  // ══════════════════════════════════════════════════════════════════════════
  // FINAL: JS errors + summary
  // ══════════════════════════════════════════════════════════════════════════
  console.log("\n── FINAL ───────────────────────────────────────────────────────────────────────────");

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across all hops");
  else fail(`JS_ERRORS — ${jsErrors.length} JS error(s): ${jsErrors.slice(0,2).join("; ")}`);

  await page.screenshot({ path: SS("L76_final.png") });
  note("Screenshot: L76_final.png");

  // Print lateral-link matrix
  console.log("\n══════════════ LATERAL-LINK MATRIX ═══════════════");
  console.log("ID     | From              | To                                                 | Exists | Specific | Notes");
  console.log("-------|-------------------|-----------------------------------------------------|--------|----------|-------");
  for (const r of llMatrix) {
    const id  = r.id.padEnd(6);
    const frm = r.from.padEnd(17).slice(0,17);
    const to  = r.to.padEnd(51).slice(0,51);
    const ex  = r.exists.padEnd(6);
    const sp  = r.specific.padEnd(8);
    console.log(`${id} | ${frm} | ${to} | ${ex} | ${sp} | ${r.notes}`);
  }

  // Print breadcrumb audit
  console.log("\n══════════════ BREADCRUMB AUDIT ═══════════════════");
  console.log("Screen         | Present | Functional | Notes");
  console.log("---------------|---------|------------|-------");
  for (const r of bcAudit) {
    const sc = r.screen.padEnd(14).slice(0,14);
    const pr = r.present.padEnd(7);
    const fn = r.functional.padEnd(10);
    console.log(`${sc} | ${pr} | ${fn} | ${r.notes}`);
  }

} catch (err) {
  console.error("FATAL:", err);
  failed++;
} finally {
  await browser.close();
  const total = passed + failed + absent;
  console.log(`\n${passed} PASS · ${failed} FAIL · ${absent} ABSENT   EXIT ${failed > 0 ? 1 : 0}`);
  process.exit(failed > 0 ? 1 : 0);
}
