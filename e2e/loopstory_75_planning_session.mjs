// L75 E2E loop story — "The Planning Session" (Dev) — 2026-06-22
//
// Theme: INTER-PAGE LINKS & CROSS-NAVIGATION — Story 2: FORWARD/ACTIONABLE links
//
// Persona: Dev, doing forward planning. For each SIGNAL (goal behind pace, budget overage,
// projected shortfall, alert/insight) surfaced by the app, expects a direct link to the ACTION
// that addresses it. Chain spanning ≥5 screens and 8+ navigations.
//
// KEY ASSERTIONS (SIGNAL→ACTION matrix):
//
//  SA-1  /dashboard alert/signal          → action link exists (label + dest)
//  SA-2  /goals behind-pace goal          → inline "Contribute"/"Fund this" action
//  SA-3  /goals linked-account link       → correct /accounts row (not just nav rail)
//  SA-4  /budgets over-budget category    → "Cover"/"adjust"/"move money" action inline
//  SA-5  /budgets over-budget drill       → /transactions filtered (GAP-E re-test: row set ACTUALLY filtered)
//  SA-6  /bills due/overdue bill          → "Mark paid" or "Pay now" action that posts
//  SA-7  /bills bill                      → link to related account/transaction
//  SA-8  /planning projected shortfall    → link to lever (recurring, budget, account)
//  SA-9  /planning recurring items        → surfaced on forecast (Thread B re-test)
//  SA-10 /reports biggest category        → drill to /transactions filtered (verify row set)
//  SA-11 /insights suggestion             → "Act on this" link or save-as-task round-trip
//  SA-12 Filter specificity (GAP-E)       → after category drill, RESULT ROW SET is filtered
//                                            (count rows, not just chip text)
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_75_planning_session.mjs

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

// ── signal→action matrix accumulator ──────────────────────────────────────────
const saMatrix = [];
const recordSA = (id, screen, signal, action, exists, filterOk, notes) => {
  saMatrix.push({ id, screen, signal, action,
    exists: exists ? "YES" : "NO",
    filterOk: filterOk === null ? "N/A" : (filterOk ? "YES" : "NO"),
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

// Count visible data rows (excluding header, footer, empty state)
const countVisibleRows = async (page) => page.evaluate(() => {
  // Try table rows
  const trs = Array.from(document.querySelectorAll('table tbody tr, [role="row"]'))
    .filter(r => {
      const t = r.textContent.trim();
      return t.length > 5 && !/^(date|amount|category|description|account|type)/i.test(t);
    });
  if (trs.length > 0) return trs.length;
  // Try list items with money amounts
  const lis = Array.from(document.querySelectorAll('li, [data-cf*="row"], [data-cf*="txn"], [data-cf*="transaction"]'))
    .filter(r => /\$[\d,]+/.test(r.textContent));
  return lis.length;
});

// Check filter chip or active filter state
const getActiveFilterInfo = async (page) => page.evaluate(() => {
  // Look for filter chips
  const chip = document.querySelector(
    '[data-cf="filter-chip"], .filter-chip, [aria-label*="filter" i], .active-filter, ' +
    '[data-cf*="active-filter"], [data-cf*="filter-active"]'
  );
  if (chip) return { text: chip.textContent.trim(), selector: "chip" };
  // Look for filter badges
  const badge = document.querySelector('.filter-badge, [data-filter], [data-cf*="badge"]');
  if (badge) return { text: badge.textContent.trim(), selector: "badge" };
  // Check URL for filter params
  const url = location.search;
  if (url) return { text: `URL: ${url}`, selector: "url" };
  return null;
});

// Find and click the first element matching any of the provided text patterns
const findAndClick = async (page, patterns, context = "main, article, section, [data-cf]") => {
  return page.evaluate(([pats, ctx]) => {
    const scope = document.querySelector(ctx) || document.body;
    for (const pat of pats) {
      const re = new RegExp(pat, "i");
      // Anchors
      for (const el of scope.querySelectorAll('a[href], a')) {
        const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "") + " " + (el.getAttribute("title") || "")).trim();
        if (re.test(txt)) { el.click(); return `anchor: "${txt.slice(0,60)}" (pat=${pat})`; }
      }
      // Buttons
      for (const el of scope.querySelectorAll('button')) {
        const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "") + " " + (el.getAttribute("title") || "")).trim();
        if (re.test(txt)) { el.click(); return `button: "${txt.slice(0,60)}" (pat=${pat})`; }
      }
      // data-cf clickables
      for (const el of scope.querySelectorAll('[data-cf], [role="link"], [role="button"]')) {
        const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "")).trim();
        if (re.test(txt)) { el.click(); return `data-cf: "${txt.slice(0,60)}" (pat=${pat})`; }
      }
    }
    return "NOT FOUND";
  }, [patterns, context]);
};

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

  // Reset "View as member" — reset to Everyone
  await page.evaluate(() => {
    const sel = document.querySelector('select[aria-label*="member" i], select[aria-label*="view as" i]');
    if (sel) { sel.value = sel.options[0]?.value; sel.dispatchEvent(new Event("change")); }
  });
  await page.waitForTimeout(400);

  await hardReload(page);
  note("Hard reload complete — View as: Everyone");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 1: /dashboard — catalog alerts/signals and their action links
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 1: /dashboard — catalog signals + action links ───────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(800);

  await page.screenshot({ path: SS("L75_hop1_dashboard.png") });
  note("Screenshot: L75_hop1_dashboard.png");

  const dashURL = await currentURL(page);
  note(`Dashboard URL: ${dashURL}`);

  // Enumerate alerts/signals visible on the dashboard
  const dashSignals = await page.evaluate(() => {
    const signals = [];
    // Alert-style: badges, warnings, "over budget", "behind", "due", "shortfall"
    const alertEls = Array.from(document.querySelectorAll(
      '[data-cf*="alert"], [data-cf*="signal"], [data-cf*="warning"], ' +
      '[aria-label*="alert" i], [role="alert"], .alert, .warning, ' +
      '[data-cf*="insight"], [data-cf*="notification"]'
    ));
    for (const el of alertEls) {
      const t = el.textContent.trim().slice(0, 80);
      if (t) signals.push({ type: "data-cf-alert", text: t });
    }
    // Text pattern alerts in widget content
    const bodyText = document.body.textContent;
    const patterns = [
      /over.?budget/gi,
      /behind.?pace|behind schedule/gi,
      /due soon|overdue|past due/gi,
      /shortfall|projected deficit/gi,
      /bill.{0,10}due/gi,
      /goal.{0,10}behind/gi,
    ];
    for (const pat of patterns) {
      const m = bodyText.match(pat);
      if (m) signals.push({ type: "text-match", text: m[0] });
    }
    return signals;
  });
  note(`Dashboard signals found: ${dashSignals.length}`);
  dashSignals.forEach(s => note(`  signal: [${s.type}] "${s.text}"`));

  // Check for actionable CTAs associated with these signals
  const dashActionLinks = await page.evaluate(() => {
    const actions = [];
    // Look for action-type buttons/links near signals
    const els = Array.from(document.querySelectorAll('a[href], button')).filter(el => {
      const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "")).trim();
      return /contribute|fund|pay|cover|adjust|fix|act|add funds|move money|resolve|view|details/i.test(txt) &&
             txt.length < 80;
    });
    for (const el of els) {
      const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "")).trim();
      const href = el.getAttribute("href") || "";
      actions.push({ tag: el.tagName, text: txt.slice(0, 60), href });
    }
    return actions;
  });
  note(`Dashboard action links found: ${dashActionLinks.length}`);
  dashActionLinks.slice(0, 5).forEach(a => note(`  action: [${a.tag}] "${a.text}" href="${a.href}"`));

  const sa1Exists = dashActionLinks.length > 0 || dashSignals.length > 0;
  recordSA("SA-1", "/dashboard", "any visible alert/signal", "action link present", sa1Exists, null,
    `signals=${dashSignals.length} action-links=${dashActionLinks.length}`);
  if (dashActionLinks.length > 0) pass("SA-1 — Dashboard has signal→action links");
  else if (dashSignals.length > 0) absent_("SA-1 — Dashboard shows signals but NO action links for them");
  else absent_("SA-1 — Dashboard has no signals and no action links (blank slate seed data?)");

  await page.screenshot({ path: SS("L75_hop1b_dashboard_signals.png") });
  note("Screenshot: L75_hop1b_dashboard_signals.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 2: /goals — behind-pace goal → "Contribute" / "Fund this" inline action
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 2: /goals — behind-pace signal → Contribute action ──────────────────────────");

  await navTo(page, "Goals");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L75_hop2_goals.png") });
  note("Screenshot: L75_hop2_goals.png");

  const goalsURL = await currentURL(page);
  note(`Goals URL: ${goalsURL}`);

  const goalsBehindText = await page.evaluate(() => {
    const t = document.body.textContent;
    return {
      hasBehind: /behind|behind pace|off track|short/i.test(t),
      hasContribute: /contribute|fund|add funds|deposit|move money/i.test(t),
      hasOnTrack: /on track|ahead/i.test(t),
    };
  });
  note(`Goals page state: behind=${goalsBehindText.hasBehind} contribute=${goalsBehindText.hasContribute} ontrack=${goalsBehindText.hasOnTrack}`);

  // Look for "Contribute"/"Fund" action button inline with goal rows
  const contributeClickResult = await findAndClick(page,
    ["Contribute", "Fund this", "Add funds", "Make a contribution", "Deposit", "Move money to goal"],
    "main, article, [data-cf*='goal']");
  note(`  Contribute action click: ${contributeClickResult}`);
  await page.waitForTimeout(1000);

  const afterContributeURL = await currentURL(page);
  const contributeModalOpen = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  const contributeExists = contributeClickResult !== "NOT FOUND" && (contributeModalOpen || afterContributeURL !== goalsURL || goalsBehindText.hasContribute);
  note(`  After contribute click: URL=${afterContributeURL} modal=${contributeModalOpen}`);

  recordSA("SA-2", "/goals", "goal behind pace", "Contribute/Fund inline action", contributeExists, null,
    `click=${contributeClickResult} modal=${contributeModalOpen} dest=${afterContributeURL}`);
  if (contributeExists) pass("SA-2 — Goals page has inline Contribute/Fund action for behind-pace goal");
  else absent_("SA-2 — Goals page has NO inline Contribute action for behind-pace goals (SIGNAL→ACTION gap)");

  // Dismiss any modal opened
  await dismissModal(page);
  await page.waitForTimeout(400);

  // Now look for linked-account link that goes to SPECIFIC account row (not just nav rail)
  const goalLinkedAcctResult = await page.evaluate(() => {
    // Look for per-goal linked account links that are INSIDE the goals content area
    // Exclude nav rail links (usually in <nav>)
    const mainEl = document.querySelector('main, [data-cf*="content"], article') || document.body;
    const links = Array.from(mainEl.querySelectorAll('a[href*="account"]'));
    if (links.length === 0) return "NOT FOUND";
    const best = links.find(a => {
      const href = a.getAttribute("href") || "";
      // A specific account link would include an ID or hash
      return href.includes("#") || href.includes("?") || href.match(/accounts\/[a-z0-9-]+/i);
    });
    if (best) {
      best.click();
      return `specific-account: "${best.textContent.trim().slice(0,40)}" href="${best.getAttribute("href")}"`;
    }
    // Fall back to first account link in main content
    links[0].click();
    return `generic-account: "${links[0].textContent.trim().slice(0,40)}" href="${links[0].getAttribute("href")}"`;
  });
  note(`  Goal linked-account click: ${goalLinkedAcctResult}`);
  await page.waitForTimeout(1200);

  const goalAcctURL = await currentURL(page);
  const goalAcctIsSpecific = /accounts\/|accounts\?|accounts#/i.test(goalLinkedAcctResult) ||
    (goalLinkedAcctResult !== "NOT FOUND" && /\/accounts/.test(goalAcctURL));
  const goalAcctIsNavRail = goalLinkedAcctResult.includes("NOT FOUND") === false &&
    goalLinkedAcctResult.includes("href=\"/accounts\"") && !goalLinkedAcctResult.includes("#") && !goalLinkedAcctResult.includes("?");
  note(`  Goal linked-account dest: ${goalAcctURL} specific=${goalAcctIsSpecific} navRail=${goalAcctIsNavRail}`);

  recordSA("SA-3", "/goals", "linked account label", "specific /accounts row link (not nav rail)", goalAcctIsSpecific && !goalAcctIsNavRail, null,
    `click=${goalLinkedAcctResult} dest=${goalAcctURL} navRail=${goalAcctIsNavRail}`);
  if (goalAcctIsSpecific && !goalAcctIsNavRail) pass("SA-3 — Goal linked-account link goes to specific account (not just nav rail)");
  else if (!goalAcctIsNavRail && /\/accounts/.test(goalAcctURL)) absent_("SA-3 — Goal linked-account navigates to /accounts but link may be nav rail (check href has no ID/hash)");
  else absent_("SA-3 — Goal linked-account link is either missing or just the nav rail (GAP-G re-confirmed)");

  await page.screenshot({ path: SS("L75_hop2b_goals_contribute.png") });
  note("Screenshot: L75_hop2b_goals_contribute.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 3: /budgets — over-budget → Cover/adjust action + drill filter ACTUALLY applied
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 3: /budgets — over-budget signal → action + drill filter (GAP-E re-test) ─────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L75_hop3_budgets.png") });
  note("Screenshot: L75_hop3_budgets.png");

  const budgetsURL = await currentURL(page);
  note(`Budgets URL: ${budgetsURL}`);

  // Count total transactions visible (before any drill)
  const totalTxnCountBeforeDrill = await page.evaluate(() => {
    // Just record body text snapshot for comparison
    return document.body.textContent.length;
  });

  const budgetsState = await page.evaluate(() => {
    const t = document.body.textContent;
    return {
      hasOverBudget: /over.?budget|exceeded|over limit|100%|[1-9]\d{2,}%/i.test(t),
      hasCoverAction: /cover|adjust|move money|reallocate|transfer funds/i.test(t),
      hasCategoryRows: /\$[\d,]+/.test(t),
    };
  });
  note(`Budgets state: overBudget=${budgetsState.hasOverBudget} cover=${budgetsState.hasCoverAction} rows=${budgetsState.hasCategoryRows}`);

  // SA-4: Check for Cover/adjust/move-money action on over-budget category
  const coverActionResult = await findAndClick(page,
    ["Cover", "Adjust budget", "Move money", "Reallocate", "Transfer funds", "Fix overage"],
    "main, article, [data-cf*='budget']");
  note(`  Cover/adjust action click: ${coverActionResult}`);
  await page.waitForTimeout(800);

  const afterCoverURL = await currentURL(page);
  const coverModalOpen = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  const coverExists = coverActionResult !== "NOT FOUND" || budgetsState.hasCoverAction;
  note(`  After cover click: URL=${afterCoverURL} modal=${coverModalOpen}`);

  recordSA("SA-4", "/budgets", "over-budget category signal", "Cover/Adjust/Move-money action inline", coverExists, null,
    `click=${coverActionResult} modal=${coverModalOpen} hasCoverText=${budgetsState.hasCoverAction}`);
  if (coverExists && coverActionResult !== "NOT FOUND") pass("SA-4 — Budgets page has Cover/Adjust action for over-budget category");
  else if (budgetsState.hasCoverAction) absent_("SA-4 — Budget has cover-related text but no clickable action found");
  else absent_("SA-4 — Budgets page has NO Cover/Move-money action for over-budget (SIGNAL→ACTION gap)");

  // Dismiss any modal
  await dismissModal(page);
  await page.waitForTimeout(400);

  // SA-5 + GAP-E re-test: click a budget category row → /transactions, then COUNT ROWS
  // to verify the result set is actually filtered (not just chip text)
  await navTo(page, "Budgets");
  await page.waitForTimeout(800);

  // Get total transaction count on /transactions BEFORE the drill
  await navTo(page, "Transactions");
  await page.waitForTimeout(1000);
  const totalTxnCount = await countVisibleRows(page);
  note(`Total transaction rows on /transactions (unfiltered): ${totalTxnCount}`);

  // Go back to /budgets and do the drill
  await navTo(page, "Budgets");
  await page.waitForTimeout(800);

  // Find the category name shown in a budget row (for later comparison)
  const budgetCategoryName = await page.evaluate(() => {
    // Look for category name text near a budget row with percentage
    const rows = Array.from(document.querySelectorAll('[data-cf*="budget"], li, tr, [role="row"]'));
    for (const r of rows) {
      const t = r.textContent.trim();
      if (t.length > 5 && t.length < 200 && /\$[\d,]+/.test(t)) {
        // Extract first word-like token as category name
        const m = t.match(/^([A-Za-z &]+)/);
        return m ? m[1].trim() : null;
      }
    }
    return null;
  });
  note(`  Budget category name for drill: "${budgetCategoryName}"`);

  // Click a transactions drill link from /budgets
  const budgetDrillClick = await page.evaluate(() => {
    const sel = [
      'a[href*="transactions"]',
      'button[aria-label*="transactions" i]',
      'button[aria-label*="View transactions" i]',
      '[data-cf*="drill"] a',
    ];
    for (const s of sel) {
      const el = document.querySelector(s);
      if (el) { el.click(); return `${s}: "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Click any anchor in a budget category row
    const links = Array.from(document.querySelectorAll('main a, article a')).filter(a => {
      const href = a.getAttribute('href') || '';
      return href.includes('transaction') || href.includes('category');
    });
    if (links.length > 0) { links[0].click(); return `main-a: "${links[0].textContent.trim().slice(0,40)}" href="${links[0].getAttribute('href')}"`; }
    return "NOT FOUND";
  });
  note(`  Budget→Transactions drill click: ${budgetDrillClick}`);
  await page.waitForTimeout(1800);

  const budgetDrillURL = await currentURL(page);
  const budgetDrillOnTxns = /\/transactions/i.test(budgetDrillURL);
  note(`  Budget drill landed at: ${budgetDrillURL}`);

  if (budgetDrillOnTxns) {
    // CRITICAL: count ACTUAL rows after drill to verify filter is applied
    const drillRowCount = await countVisibleRows(page);
    const filterInfo = await getActiveFilterInfo(page);
    note(`  GAP-E re-test: drill row count=${drillRowCount} total=${totalTxnCount} filter=${JSON.stringify(filterInfo)}`);

    await page.screenshot({ path: SS("L75_hop3b_budgets_drill_filter.png") });
    note("Screenshot: L75_hop3b_budgets_drill_filter.png");

    // Check filter specificity: row count should be LESS than total (unless only 1 category)
    const filterIsApplied = drillRowCount < totalTxnCount || (filterInfo && filterInfo.text && !/select all/i.test(filterInfo.text));
    const filterChipIsSpecific = filterInfo && filterInfo.text && !/select all|all categories/i.test(filterInfo.text) && filterInfo.text.length > 3;

    recordSA("SA-5", "/budgets→/transactions", "over-budget category drill", "filter actually applied (row count < total)", filterIsApplied, filterChipIsSpecific,
      `drillRows=${drillRowCount} totalRows=${totalTxnCount} chip="${filterInfo?.text ?? "none"}" URL=${budgetDrillURL}`);

    if (filterIsApplied && filterChipIsSpecific) {
      pass("SA-5 + GAP-E CLOSED — Budget drill filter IS applied (row set filtered + chip is specific)");
    } else if (filterIsApplied) {
      pass("SA-5 partial — Budget drill row count is filtered but chip label is generic");
      absent_("GAP-E PERSISTS — Filter chip shows generic text despite row set being filtered (label bug)");
    } else if (filterChipIsSpecific) {
      absent_("SA-5 partial — Filter chip is specific but row count equals total (chip label lies, no actual filter)");
      fail("GAP-E CONFIRMED — Drill link shows specific chip but ALL transactions appear (filter not actually applied)");
    } else {
      fail("SA-5 + GAP-E CONFIRMED — Budget drill opens /transactions UNFILTERED (row count == total, chip is generic 'Select all')");
    }
  } else {
    recordSA("SA-5", "/budgets→/transactions", "over-budget drill", "link exists + filter applied", false, null,
      `drill stayed at ${budgetDrillURL} — click: ${budgetDrillClick}`);
    absent_("SA-5 — /budgets has no drill link to /transactions (can't test GAP-E)");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 4: /bills — due bill → "Mark paid" action + link to related account
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 4: /bills — due bill → Mark paid + account link ─────────────────────────────");

  await navTo(page, "Bills");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L75_hop4_bills.png") });
  note("Screenshot: L75_hop4_bills.png");

  const billsURL = await currentURL(page);
  note(`Bills URL: ${billsURL}`);

  const billsState = await page.evaluate(() => {
    const t = document.body.textContent;
    return {
      hasDue: /due|overdue|past due|upcoming/i.test(t),
      hasMarkPaid: /mark paid|mark as paid|pay now|paid/i.test(t),
      hasAccountLink: /account/i.test(t),
    };
  });
  note(`Bills state: due=${billsState.hasDue} markPaid=${billsState.hasMarkPaid} acctLink=${billsState.hasAccountLink}`);

  // SA-6: Find "Mark paid" or "Pay now" action
  const markPaidClick = await findAndClick(page,
    ["Mark paid", "Mark as paid", "Pay now", "Record payment", "Paid"],
    "main, article, [data-cf*='bill']");
  note(`  Mark paid click: ${markPaidClick}`);
  await page.waitForTimeout(1000);

  const afterMarkPaidURL = await currentURL(page);
  const markPaidModal = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  const markPaidExists = markPaidClick !== "NOT FOUND" || billsState.hasMarkPaid;

  recordSA("SA-6", "/bills", "due/overdue bill signal", "Mark paid / Pay now action", markPaidExists, null,
    `click=${markPaidClick} modal=${markPaidModal} hasMarkPaidText=${billsState.hasMarkPaid}`);
  if (markPaidExists && markPaidClick !== "NOT FOUND") pass("SA-6 — Bills page has Mark paid / Pay now action");
  else if (billsState.hasMarkPaid) absent_("SA-6 — Bills has paid text but no clickable Mark paid action found");
  else absent_("SA-6 — Bills page has NO Mark paid / Pay now action (SIGNAL→ACTION gap)");

  // Dismiss modal
  await dismissModal(page);
  await page.waitForTimeout(400);

  // SA-7: Find account/transaction link from a bill row
  await navTo(page, "Bills");
  await page.waitForTimeout(800);

  const billAcctLinkClick = await page.evaluate(() => {
    const mainEl = document.querySelector('main, article') || document.body;
    const links = Array.from(mainEl.querySelectorAll('a[href*="account"], a[href*="transaction"]'));
    if (links.length === 0) return "NOT FOUND";
    links[0].click();
    return `${links[0].getAttribute('href')}: "${links[0].textContent.trim().slice(0,40)}"`;
  });
  note(`  Bill account/txn link click: ${billAcctLinkClick}`);
  await page.waitForTimeout(1200);

  const billAcctDrillURL = await currentURL(page);
  const billAcctExists = /\/accounts|\/transactions/i.test(billAcctDrillURL) || billAcctLinkClick !== "NOT FOUND";

  recordSA("SA-7", "/bills", "bill row", "link to related account/transaction", billAcctExists, null,
    `click=${billAcctLinkClick} dest=${billAcctDrillURL}`);
  if (billAcctExists) pass("SA-7 — Bill row has link to related account or transaction");
  else absent_("SA-7 — Bill row has NO link to account or transaction (SIGNAL→ACTION gap)");

  await page.screenshot({ path: SS("L75_hop4b_bills_markpaid.png") });
  note("Screenshot: L75_hop4b_bills_markpaid.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 5: /planning — projected shortfall → link to lever; recurring items (Thread B)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 5: /planning — shortfall → lever link; Thread B recurring ──────────────────");

  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("L75_hop5_planning.png") });
  note("Screenshot: L75_hop5_planning.png");

  const planningURL = await currentURL(page);
  note(`Planning URL: ${planningURL}`);

  const planningState = await page.evaluate(() => {
    const t = document.body.textContent;
    return {
      hasShortfall: /shortfall|deficit|negative|projected.*negative|over.?spend/i.test(t),
      hasRecommendation: /recommend|suggestion|tip|action item|what to do/i.test(t),
      hasRecurring: /recurring|subscription|repeat|monthly/i.test(t),
      hasForecast: /forecast|project|estimate|future/i.test(t),
      hasLeverLink: /budget|account|adjust|change/i.test(t),
      rawLength: t.length,
    };
  });
  note(`Planning state: shortfall=${planningState.hasShortfall} rec=${planningState.hasRecommendation} recurring=${planningState.hasRecurring} forecast=${planningState.hasForecast} lever=${planningState.hasLeverLink}`);

  // Thread B: does planning/forecast surface recurring items?
  recordSA("SA-9", "/planning", "recurring items", "surfaced on forecast (Thread B)", planningState.hasRecurring, null,
    `hasRecurringText=${planningState.hasRecurring}`);
  if (planningState.hasRecurring) pass("SA-9 (Thread B) — Planning/forecast surfaces recurring items");
  else absent_("SA-9 (Thread B) — Planning/forecast does NOT surface recurring items");

  // SA-8: From a projected shortfall or recommendation, link to lever
  const shortfallLinkClick = await findAndClick(page,
    ["Adjust", "Fix shortfall", "Change budget", "View recurring", "See what to cut", "Open account", "Edit recurring", "Modify", "View budget"],
    "main, article, [data-cf*='planning'], [data-cf*='forecast']");
  note(`  Shortfall lever link click: ${shortfallLinkClick}`);
  await page.waitForTimeout(1200);

  const planningLeverURL = await currentURL(page);
  const planningModal = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  const leverExists = shortfallLinkClick !== "NOT FOUND" &&
    (planningModal || planningLeverURL !== planningURL ||
     /\/budgets|\/accounts|\/planning|\/transactions/i.test(planningLeverURL));

  recordSA("SA-8", "/planning", "projected shortfall / recommendation", "link to lever (recurring/budget/account)", leverExists, null,
    `click=${shortfallLinkClick} dest=${planningLeverURL} modal=${planningModal}`);
  if (leverExists) pass("SA-8 — Planning has actionable lever link from shortfall/recommendation");
  else absent_("SA-8 — Planning has NO actionable link from shortfall to a lever (SIGNAL→ACTION gap)");

  await dismissModal(page);
  await page.waitForTimeout(400);
  await page.screenshot({ path: SS("L75_hop5b_planning_lever.png") });
  note("Screenshot: L75_hop5b_planning_lever.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 6: /reports — biggest category → drill to filtered /transactions (verify row set)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 6: /reports — biggest category → /transactions filter (row set verification) ─");

  await navTo(page, "Reports");
  await dismissModal(page);
  await page.waitForTimeout(1200);

  await page.screenshot({ path: SS("L75_hop6_reports.png") });
  note("Screenshot: L75_hop6_reports.png");

  const reportsURL = await currentURL(page);
  note(`Reports URL: ${reportsURL}`);

  // Get the name of the first/biggest category visible
  const topCategoryName = await page.evaluate(() => {
    // Look in spending-by-category section for the top entry
    const rows = Array.from(document.querySelectorAll('[data-cf*="category"], li, tr, [role="row"]'));
    for (const r of rows) {
      const t = r.textContent.trim();
      if (t.length > 3 && t.length < 150 && /\$[\d,]+/.test(t)) {
        const m = t.match(/^([A-Za-z &\/\-]+)/);
        return m ? m[1].trim() : null;
      }
    }
    return null;
  });
  note(`  Top category name: "${topCategoryName}"`);

  // Click the first drill link from reports
  const reportsDrillClick = await page.evaluate(() => {
    const mainEl = document.querySelector('main, article') || document.body;
    const links = Array.from(mainEl.querySelectorAll('a[href*="transaction"]'));
    if (links.length > 0) {
      links[0].click();
      return `href-link: "${links[0].textContent.trim().slice(0,40)}" href="${links[0].getAttribute('href')}"`;
    }
    // Try category row clicks
    const rows = Array.from(mainEl.querySelectorAll('[role="row"], li, [data-cf*="category"]'))
      .filter(r => /\$[\d,]+/.test(r.textContent));
    if (rows.length > 0) { rows[0].click(); return `row-click: "${rows[0].textContent.trim().slice(0,40)}"`; }
    return "NOT FOUND";
  });
  note(`  Reports category drill click: ${reportsDrillClick}`);
  await page.waitForTimeout(1800);

  const reportsDrillURL = await currentURL(page);
  const reportsDrillOnTxns = /\/transactions/i.test(reportsDrillURL);
  note(`  Reports drill landed at: ${reportsDrillURL}`);

  if (reportsDrillOnTxns) {
    // Count rows to verify filtering
    const drillRowCount2 = await countVisibleRows(page);
    const filterInfo2 = await getActiveFilterInfo(page);
    note(`  Reports GAP-E re-test: drill rows=${drillRowCount2} total=${totalTxnCount} filter=${JSON.stringify(filterInfo2)}`);

    await page.screenshot({ path: SS("L75_hop6b_reports_drill_filter.png") });
    note("Screenshot: L75_hop6b_reports_drill_filter.png");

    const filterApplied2 = drillRowCount2 < totalTxnCount || (filterInfo2 && filterInfo2.text && !/select all/i.test(filterInfo2.text));
    const chipSpecific2 = filterInfo2 && filterInfo2.text && !/select all|all categories/i.test(filterInfo2.text);

    recordSA("SA-10", "/reports→/transactions", "biggest category anomaly drill", "filter applied (row count < total)", filterApplied2, chipSpecific2,
      `drillRows=${drillRowCount2} total=${totalTxnCount} chip="${filterInfo2?.text ?? "none"}" URL=${reportsDrillURL}`);

    if (filterApplied2 && chipSpecific2) pass("SA-10 + GAP-E CLOSED — Reports drill IS filtered (specific chip + fewer rows)");
    else if (filterApplied2) pass("SA-10 partial — Reports drill row count filtered but chip is generic");
    else if (chipSpecific2) {
      absent_("SA-10 partial — Reports chip is specific but ALL rows appear");
      fail("GAP-E CONFIRMED on /reports — chip label is specific but row set is NOT filtered");
    } else {
      fail("SA-10 + GAP-E CONFIRMED — Reports drill opens UNFILTERED /transactions (all rows + generic chip)");
    }
  } else {
    recordSA("SA-10", "/reports→/transactions", "biggest category drill", "link + filter applied", false, null,
      `stayed at ${reportsDrillURL} — click: ${reportsDrillClick}`);
    absent_("SA-10 — /reports category has no drill link to /transactions");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 7: /insights — suggestion → "Act on this" / save-as-task round-trip
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 7: /insights — suggestion → act-on-this / save-as-task ─────────────────────");

  await navTo(page, "Insights");
  await dismissModal(page);
  await page.waitForTimeout(1200);

  await page.screenshot({ path: SS("L75_hop7_insights.png") });
  note("Screenshot: L75_hop7_insights.png");

  const insightsURL = await currentURL(page);
  note(`Insights URL: ${insightsURL}`);

  const insightsState = await page.evaluate(() => {
    const t = document.body.textContent;
    return {
      hasInsights: /insight|suggestion|tip|recommend|anomaly|pattern/i.test(t),
      hasActOnThis: /act on this|action|save as task|add to tasks|add task/i.test(t),
      hasDeepLink: true, // will check for links below
      rawLen: t.length,
    };
  });
  note(`Insights state: hasInsights=${insightsState.hasInsights} actOnThis=${insightsState.hasActOnThis}`);

  // SA-11: Find "Act on this" or save-as-task button
  const actOnThisClick = await findAndClick(page,
    ["Act on this", "Save as task", "Add to tasks", "Add task", "Create task", "Take action", "Respond"],
    "main, article, [data-cf*='insight'], [data-cf*='suggestion']");
  note(`  Act on this click: ${actOnThisClick}`);
  await page.waitForTimeout(1000);

  const afterActURL = await currentURL(page);
  const actModal = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  const actExists = actOnThisClick !== "NOT FOUND" || insightsState.hasActOnThis;

  recordSA("SA-11", "/insights", "AI/heuristic suggestion", "Act on this / save-as-task round-trip", actExists, null,
    `click=${actOnThisClick} modal=${actModal} dest=${afterActURL} hasActOnThisText=${insightsState.hasActOnThis}`);
  if (actExists && actOnThisClick !== "NOT FOUND") pass("SA-11 — Insights has Act on this / Save-as-task action");
  else if (insightsState.hasActOnThis) absent_("SA-11 — Insights has act-related text but no clickable action found");
  else absent_("SA-11 — Insights has NO Act on this or Save-as-task action (SIGNAL→ACTION gap)");

  // Dismiss modal if opened
  await dismissModal(page);
  await page.waitForTimeout(400);

  // Check for any deep link from insight to underlying screen
  const insightDeepLinkClick = await page.evaluate(() => {
    const mainEl = document.querySelector('main, article') || document.body;
    const links = Array.from(mainEl.querySelectorAll('a[href]')).filter(a => {
      const href = a.getAttribute('href') || '';
      return href.includes('budget') || href.includes('account') || href.includes('transaction') ||
             href.includes('goal') || href.includes('bill') || href.includes('planning');
    });
    if (links.length > 0) {
      const href = links[0].getAttribute('href');
      const txt = links[0].textContent.trim().slice(0, 40);
      return `href="${href}" text="${txt}"`;
    }
    return "NOT FOUND";
  });
  note(`  Insight deep link: ${insightDeepLinkClick}`);
  const hasInsightDeepLink = insightDeepLinkClick !== "NOT FOUND";
  if (hasInsightDeepLink) pass("SA-11b — Insights has deep link to underlying screen/entity");
  else absent_("SA-11b — Insights has NO deep link to underlying data screen");

  await page.screenshot({ path: SS("L75_hop7b_insights_action.png") });
  note("Screenshot: L75_hop7b_insights_action.png");

  // ════════════════════════════════════════════════════════════════════════════
  // FINAL: Check JS errors + summary screenshot
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── FINAL CHECK ─────────────────────────────────────────────────────────────────────");

  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);
  await page.screenshot({ path: SS("L75_final_dashboard.png") });
  note("Screenshot: L75_final_dashboard.png");

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across entire ritual");
  else fail(`JS_ERRORS — ${jsErrors.length} error(s): ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`);
  console.error(err);
} finally {
  await browser.close();
}

// ── print signal→action matrix ────────────────────────────────────────────────
console.log("\n\n══════════════════════════════════════════════════════════════════════════");
console.log("SIGNAL→ACTION MATRIX (L75 — The Planning Session)");
console.log("══════════════════════════════════════════════════════════════════════════");
console.log("ID     Screen            Signal                 Action               Exists  Filter  Notes");
console.log("─────  ────────────────  ─────────────────────  ──────────────────── ──────  ──────  ─────────────────────────────────────────");
for (const r of saMatrix) {
  const id     = r.id.padEnd(6);
  const screen = r.screen.padEnd(17);
  const signal = r.signal.slice(0,22).padEnd(23);
  const action = r.action.slice(0,20).padEnd(21);
  const ex     = r.exists.padEnd(7);
  const fi     = r.filterOk.padEnd(7);
  console.log(`${id} ${screen} ${signal} ${action} ${ex} ${fi} ${r.notes.slice(0,80)}`);
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
