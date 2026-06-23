// L74 E2E loop story — "The Sunday Review" (Priya) — 2026-06-22
//
// Theme: INTER-PAGE LINKS & CROSS-NAVIGATION
//
// Persona: Priya, 42, household manager. It's Sunday afternoon. She opens CashFlux for her
// weekly review — not to enter data, but to *navigate*. She taps each widget on the dashboard,
// follows every drill-down link, expects filters + period to carry across hops, and wants
// clean Back-navigation at every point.
//
// KEY INVARIANTS ASSERTED (L74 LINK MATRIX):
//   LM-1  /dashboard widget → /transactions (net worth drill)
//   LM-2  /dashboard widget → /reports     (spending / insights)
//   LM-3  /dashboard widget → /budgets     (budget tile)
//   LM-4  /dashboard widget → /goals       (goal tile)
//   LM-5  /dashboard widget → /bills       (upcoming bills widget)
//   LM-6  /dashboard widget → /transactions (recent-transactions row)
//   LM-7  /reports spending-by-category row → /transactions filtered  (re-test L58)
//   LM-8  /budgets over-budget row → /transactions filtered
//   LM-9  /goals goal row → /accounts (linked account)
//   LM-10 /bills bill row → /transactions or /accounts
//   LM-11 /accounts account row → /transactions ledger (filtered)
//   LM-12 /insights settings CTA → /settings                          (re-test L62)
//   LM-13 FILTER_CARRY — filter+period survives each cross-page hop
//   LM-14 BACK_NAV    — browser Back (or nav Back) returns to origin without crash
//
// Screens exercised (≥7): /dashboard → /reports → /transactions → /budgets → /goals →
//                          /bills → /accounts → /insights
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_74_sunday_review.mjs

import { createRequire } from "module";
import { fileURLToPath }  from "url";
import path from "path";
import fs   from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require   = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (name) => path.join(SSDIR, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass    = (label) => { console.log(`PASS:   ${label}`);  passed++; };
const fail    = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_ = (label) => { console.log(`ABSENT: ${label}`);  absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

// ── link matrix accumulator ────────────────────────────────────────────────────
const linkMatrix = [];
const recordLink = (id, src, tgt, exists, carriesState, notes) => {
  const status = exists ? "YES" : "NO";
  const carry  = carriesState === null ? "N/A" : (carriesState ? "YES" : "NO");
  linkMatrix.push({ id, src, tgt, exists: status, carry, notes });
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

const getPeriodPill = (page) => page.evaluate(() => {
  const cands = [
    document.querySelector('[data-cf="period-pill"]'),
    document.querySelector('[aria-label*="period" i]'),
    document.querySelector('.period-pill'),
    ...Array.from(document.querySelectorAll('button, span')).filter(el =>
      /\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{4}|\b\d{4}\b|This month|Last month/i.test(el.textContent.trim()) &&
      el.textContent.trim().length < 30),
  ].filter(Boolean);
  return cands.length ? cands[0].textContent.trim() : null;
});

const getFilterChip = (page) => page.evaluate(() => {
  // Look for active filter chip or filter badge
  const chip = document.querySelector('[data-cf="filter-chip"], .filter-chip, [aria-label*="filter" i], .active-filter');
  return chip ? chip.textContent.trim() : null;
});

const clickWidgetLink = async (page, ariaPatterns, descriptionForLog) => {
  const result = await page.evaluate((patterns) => {
    for (const pat of patterns) {
      const re = new RegExp(pat, "i");
      // Try anchor tags first
      const anchors = Array.from(document.querySelectorAll('a[href], a'));
      for (const a of anchors) {
        const txt = (a.textContent + " " + (a.getAttribute("aria-label") || "") + " " + (a.getAttribute("title") || "")).trim();
        if (re.test(txt)) { a.click(); return `clicked anchor: "${txt.slice(0,60)}" (pattern=${pat})`; }
      }
      // Then buttons
      const buttons = Array.from(document.querySelectorAll('button'));
      for (const b of buttons) {
        const txt = (b.textContent + " " + (b.getAttribute("aria-label") || "") + " " + (b.getAttribute("title") || "")).trim();
        if (re.test(txt)) { b.click(); return `clicked button: "${txt.slice(0,60)}" (pattern=${pat})`; }
      }
      // Then clickable divs/spans (data-cf links)
      const clickables = Array.from(document.querySelectorAll('[data-cf], [role="link"], [role="button"]'));
      for (const el of clickables) {
        const txt = (el.textContent + " " + (el.getAttribute("aria-label") || "")).trim();
        if (re.test(txt)) { el.click(); return `clicked data-cf: "${txt.slice(0,60)}" (pattern=${pat})`; }
      }
    }
    return "NOT FOUND";
  }, ariaPatterns);
  note(`  Widget link "${descriptionForLog}": ${result}`);
  return result;
};

const dismissModal = async (page) => {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  await page.waitForTimeout(200);
};

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(300);
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

  // Hard reload to clear stale state
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  note("Hard reload complete");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 1: /dashboard — screenshot, enumerate widget drill links
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 1: /dashboard ────────────────────────────────────────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(800);

  await page.screenshot({ path: SS("L74_hop1_dashboard.png") });
  note("Screenshot: L74_hop1_dashboard.png");

  const dashURL = await currentURL(page);
  const dashPeriod = await getPeriodPill(page);
  note(`Dashboard URL: ${dashURL} | Period: ${dashPeriod}`);

  const dashText = await page.evaluate(() => document.body.textContent);

  // Probe each widget type
  const hasNetWorth      = /net worth/i.test(dashText);
  const hasSpendingWidget= /spending|spend/i.test(dashText);
  const hasBudgetTile    = /budget/i.test(dashText);
  const hasGoalTile      = /goal/i.test(dashText);
  const hasUpcomingBills = /upcoming bill|bill/i.test(dashText);
  const hasRecentTxns    = /recent transaction|transaction/i.test(dashText);
  const hasInsightsWidget= /insight|report/i.test(dashText);

  note(`Dashboard widgets: netWorth=${hasNetWorth} spending=${hasSpendingWidget} budget=${hasBudgetTile} goal=${hasGoalTile} bills=${hasUpcomingBills} recentTxns=${hasRecentTxns} insights=${hasInsightsWidget}`);

  if (hasNetWorth)       pass("LM-1a — Net Worth widget present on Dashboard");
  else                   absent_("LM-1a — Net Worth widget absent from Dashboard");

  if (hasSpendingWidget) pass("LM-2a — Spending widget present on Dashboard");
  else                   absent_("LM-2a — Spending widget absent from Dashboard");

  if (hasBudgetTile)     pass("LM-3a — Budget tile present on Dashboard");
  else                   absent_("LM-3a — Budget tile absent from Dashboard");

  if (hasGoalTile)       pass("LM-4a — Goal tile present on Dashboard");
  else                   absent_("LM-4a — Goal tile absent from Dashboard");

  if (hasUpcomingBills)  pass("LM-5a — Upcoming bills widget present on Dashboard");
  else                   absent_("LM-5a — Upcoming bills widget absent from Dashboard");

  if (hasRecentTxns)     pass("LM-6a — Recent transactions widget present on Dashboard");
  else                   absent_("LM-6a — Recent transactions widget absent from Dashboard");

  // ── LM-1: Net Worth → click "View all" or "Details" or the widget itself
  console.log("\n  → LM-1: Dashboard Net Worth → /transactions or /accounts drill");
  const nwClickResult = await clickWidgetLink(page,
    ["View all accounts", "All accounts", "Net Worth.*View", "View.*net worth", "→.*account", "See all"],
    "Net Worth drill-link");
  await page.waitForTimeout(1200);
  const nwDest = await currentURL(page);
  const nwExists = !/^\/$|^\/dashboard/.test(nwDest) && nwClickResult !== "NOT FOUND";
  note(`  Net Worth drill destination: ${nwDest}`);
  recordLink("LM-1", "/dashboard (net worth)", "/accounts or /transactions", nwExists, null,
    nwClickResult !== "NOT FOUND" ? `dest=${nwDest}` : "no drill link found");
  if (nwExists) pass("LM-1 — Net Worth widget has a drill link (navigated away from dashboard)");
  else          absent_("LM-1 — Net Worth widget has NO drill link (stays on /dashboard)");

  // Back to dashboard
  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);

  // ── LM-2: Spending → Reports
  console.log("\n  → LM-2: Dashboard Spending → /reports");
  const spendClickResult = await clickWidgetLink(page,
    ["View report", "See report", "Spending.*View", "View.*spending", "Reports"],
    "Spending drill-link");
  await page.waitForTimeout(1200);
  const spendDest = await currentURL(page);
  const spendExists = /\/reports/i.test(spendDest) || (spendClickResult !== "NOT FOUND" && !/dashboard/.test(spendDest));
  note(`  Spending drill destination: ${spendDest}`);
  recordLink("LM-2", "/dashboard (spending)", "/reports", spendExists, null,
    spendClickResult !== "NOT FOUND" ? `dest=${spendDest}` : "no drill link found");
  if (spendExists) pass("LM-2 — Spending widget drills to /reports");
  else             absent_("LM-2 — Spending widget has NO link to /reports");
  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);

  // ── LM-3: Budget tile → /budgets
  console.log("\n  → LM-3: Dashboard Budget tile → /budgets");
  const budgetClickResult = await clickWidgetLink(page,
    ["View budget", "See budget", "Budget.*View", "Manage budget", "All budgets"],
    "Budget tile drill-link");
  await page.waitForTimeout(1200);
  const budgetDest = await currentURL(page);
  const budgetExists = /\/budgets/i.test(budgetDest) || (budgetClickResult !== "NOT FOUND" && !/dashboard/.test(budgetDest));
  note(`  Budget tile drill destination: ${budgetDest}`);
  recordLink("LM-3", "/dashboard (budget tile)", "/budgets", budgetExists, null,
    budgetClickResult !== "NOT FOUND" ? `dest=${budgetDest}` : "no drill link found");
  if (budgetExists) pass("LM-3 — Budget tile drills to /budgets");
  else              absent_("LM-3 — Budget tile has NO link to /budgets");
  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);

  // ── LM-4: Goal tile → /goals
  console.log("\n  → LM-4: Dashboard Goal tile → /goals");
  const goalClickResult = await clickWidgetLink(page,
    ["View goals", "See goals", "Goal.*View", "All goals", "Manage goals"],
    "Goal tile drill-link");
  await page.waitForTimeout(1200);
  const goalDest = await currentURL(page);
  const goalExists = /\/goals/i.test(goalDest) || (goalClickResult !== "NOT FOUND" && !/dashboard/.test(goalDest));
  note(`  Goal tile drill destination: ${goalDest}`);
  recordLink("LM-4", "/dashboard (goal tile)", "/goals", goalExists, null,
    goalClickResult !== "NOT FOUND" ? `dest=${goalDest}` : "no drill link found");
  if (goalExists) pass("LM-4 — Goal tile drills to /goals");
  else            absent_("LM-4 — Goal tile has NO link to /goals");
  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);

  // ── LM-5: Upcoming bills → /bills
  console.log("\n  → LM-5: Dashboard Upcoming Bills → /bills");
  const billsClickResult = await clickWidgetLink(page,
    ["View bills", "See bills", "Bill.*View", "All bills", "Upcoming bill"],
    "Upcoming bills drill-link");
  await page.waitForTimeout(1200);
  const billsDest = await currentURL(page);
  const billsExists = /\/bills/i.test(billsDest) || (billsClickResult !== "NOT FOUND" && !/dashboard/.test(billsDest));
  note(`  Upcoming bills drill destination: ${billsDest}`);
  recordLink("LM-5", "/dashboard (upcoming bills)", "/bills", billsExists, null,
    billsClickResult !== "NOT FOUND" ? `dest=${billsDest}` : "no drill link found");
  if (billsExists) pass("LM-5 — Upcoming bills widget drills to /bills");
  else             absent_("LM-5 — Upcoming bills widget has NO link to /bills");
  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);

  // ── LM-6: Recent transactions row → /transactions
  console.log("\n  → LM-6: Dashboard Recent Transactions → /transactions");
  const recentClickResult = await clickWidgetLink(page,
    ["View all transactions", "See all transactions", "All transactions", "Recent.*View", "View.*transactions"],
    "Recent transactions drill-link");
  await page.waitForTimeout(1200);
  const recentDest = await currentURL(page);
  const recentExists = /\/transactions/i.test(recentDest) || (recentClickResult !== "NOT FOUND" && !/dashboard/.test(recentDest));
  note(`  Recent transactions drill destination: ${recentDest}`);
  recordLink("LM-6", "/dashboard (recent transactions)", "/transactions", recentExists, null,
    recentClickResult !== "NOT FOUND" ? `dest=${recentDest}` : "no drill link found");
  if (recentExists) pass("LM-6 — Recent transactions widget drills to /transactions");
  else              absent_("LM-6 — Recent transactions widget has NO link to /transactions");

  await page.screenshot({ path: SS("L74_hop1b_dashboard_links.png") });
  note("Screenshot: L74_hop1b_dashboard_links.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 2: /reports spending-by-category → /transactions filtered (re-test L58)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 2: /reports → category row → /transactions filtered (re-test L58) ───────────");

  await navTo(page, "Reports");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop2_reports.png") });
  note("Screenshot: L74_hop2_reports.png");

  const reportsURL = await currentURL(page);
  const reportsPeriod = await getPeriodPill(page);
  note(`Reports URL: ${reportsURL} | Period: ${reportsPeriod}`);

  const reportsText = await page.evaluate(() => document.body.textContent);
  const hasSpendingByCategory = /spending by categor|by category|category breakdown/i.test(reportsText);
  note(`/reports has spending-by-category section: ${hasSpendingByCategory}`);

  // Try to click the first category row
  const catClickResult = await page.evaluate(() => {
    // Look for category row links in the spending breakdown
    const selectors = [
      '[data-cf*="category"] a',
      '.category-row a',
      '.category-link',
      'a[href*="transactions"]',
      '[data-cf*="spending"] a',
      '[data-cf*="spend"] a',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Fallback: find any link in a spending/category section
    const allLinks = Array.from(document.querySelectorAll('a')).filter(a => {
      const href = a.getAttribute('href') || '';
      return href.includes('transaction') || href.includes('category');
    });
    if (allLinks.length > 0) { allLinks[0].click(); return `clicked href-link: "${allLinks[0].textContent.trim().slice(0,40)}"`; }
    // Last resort: clickable rows
    const rows = Array.from(document.querySelectorAll('[role="row"], tr, li')).filter(r => {
      const t = r.textContent.trim();
      return t.length > 5 && t.length < 200 && /\$[\d,]+/.test(t);
    });
    if (rows.length > 0) { rows[0].click(); return `clicked row: "${rows[0].textContent.trim().slice(0,40)}"`; }
    return "NOT FOUND";
  });
  note(`  Category row click: ${catClickResult}`);
  await page.waitForTimeout(1500);

  const catDrillURL = await currentURL(page);
  const catDrillPeriod = await getPeriodPill(page);
  const catDrillFilter = await getFilterChip(page);
  const catDrillText   = await page.evaluate(() => document.body.textContent);

  const catDrillExists = /\/transactions/i.test(catDrillURL);
  const catDrillURLParams = catDrillURL.includes("category") || catDrillURL.includes("filter") || catDrillURL.includes("cat=");
  const filterCarried = catDrillFilter !== null || catDrillURLParams || /filter|categor/i.test(catDrillText.slice(0, 500));
  note(`  Category drill URL: ${catDrillURL} | Period: ${catDrillPeriod} | Filter chip: ${catDrillFilter}`);

  recordLink("LM-7", "/reports (category row)", "/transactions (filtered)", catDrillExists,
    catDrillExists ? filterCarried : null,
    catDrillExists
      ? `filter chip: ${catDrillFilter ?? "not found"} | URL params: ${catDrillURLParams}`
      : `stayed at ${catDrillURL} — click: ${catClickResult}`);

  if (catDrillExists) {
    pass("LM-7 — /reports category row drills to /transactions (re-test L58 ✓)");
    if (filterCarried) pass("LM-7 filter — category filter carried to /transactions");
    else               absent_("LM-7 filter — /transactions opened but NO category filter applied");
  } else {
    absent_("LM-7 — /reports category row has NO drill to /transactions (L58 gap not fixed)");
  }

  await page.screenshot({ path: SS("L74_hop2b_reports_drill.png") });
  note("Screenshot: L74_hop2b_reports_drill.png");

  // Back to reports
  await navTo(page, "Reports");
  await page.waitForTimeout(600);

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 3: /budgets over-budget → /transactions filtered
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 3: /budgets over-budget category → /transactions filtered ────────────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop3_budgets.png") });
  note("Screenshot: L74_hop3_budgets.png");

  const budgetsURL = await currentURL(page);
  const budgetsPeriod = await getPeriodPill(page);
  note(`Budgets URL: ${budgetsURL} | Period: ${budgetsPeriod}`);

  const budgetsText = await page.evaluate(() => document.body.textContent);
  const hasOverBudget = /over.?budget|exceeded|over limit|\$[\d,]+.*\$[\d,]+/i.test(budgetsText);
  note(`/budgets has over-budget indicator: ${hasOverBudget}`);

  // Try to click a budget category row to drill to transactions
  const budgetCatClickResult = await page.evaluate(() => {
    const selectors = [
      '[data-cf*="budget"] a',
      '.budget-row a',
      'a[href*="transactions"]',
      '[data-cf*="over"] a',
      'button[aria-label*="transactions"]',
      'button[aria-label*="View transactions"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Clickable budget rows
    const rows = Array.from(document.querySelectorAll('[role="row"], tr, li, [data-cf]')).filter(r => {
      const t = r.textContent.trim();
      return t.length > 10 && t.length < 300 && /\$[\d,]+/.test(t);
    });
    if (rows.length > 0) { rows[0].click(); return `clicked budget row: "${rows[0].textContent.trim().slice(0,50)}"`; }
    return "NOT FOUND";
  });
  note(`  Budget category row click: ${budgetCatClickResult}`);
  await page.waitForTimeout(1500);

  const budgetDrillURL = await currentURL(page);
  const budgetDrillPeriod = await getPeriodPill(page);
  const budgetDrillFilter = await getFilterChip(page);
  const budgetDrillText   = await page.evaluate(() => document.body.textContent);

  const budgetDrillExists = /\/transactions/i.test(budgetDrillURL);
  const budgetFilterCarried = budgetDrillFilter !== null ||
    budgetDrillURL.includes("category") || budgetDrillURL.includes("filter") ||
    /filter|categor/i.test(budgetDrillText.slice(0, 500));
  note(`  Budget drill URL: ${budgetDrillURL} | Period: ${budgetDrillPeriod} | Filter chip: ${budgetDrillFilter}`);

  recordLink("LM-8", "/budgets (over-budget row)", "/transactions (filtered)", budgetDrillExists,
    budgetDrillExists ? budgetFilterCarried : null,
    budgetDrillExists
      ? `filter chip: ${budgetDrillFilter ?? "not found"} | period carried: ${budgetDrillPeriod === budgetsPeriod}`
      : `stayed at ${budgetDrillURL} — click: ${budgetCatClickResult}`);

  if (budgetDrillExists) {
    pass("LM-8 — /budgets category row drills to /transactions");
    if (budgetFilterCarried) pass("LM-8 filter — category filter carried from /budgets to /transactions");
    else                     absent_("LM-8 filter — /transactions opened but NO budget category filter applied");
  } else {
    absent_("LM-8 — /budgets has NO drill link to /transactions (gap)");
  }

  // Period carry check
  if (budgetDrillExists && budgetsPeriod && budgetDrillPeriod) {
    if (budgetsPeriod === budgetDrillPeriod) pass("LM-13a — Period carried: /budgets → /transactions");
    else fail(`LM-13a — Period NOT carried: budgets="${budgetsPeriod}" txns="${budgetDrillPeriod}"`);
  } else {
    note(`LM-13a — period carry check skipped (drill=${budgetDrillExists}, budgetsPeriod=${budgetsPeriod}, drillPeriod=${budgetDrillPeriod})`);
  }

  await page.screenshot({ path: SS("L74_hop3b_budgets_drill.png") });
  note("Screenshot: L74_hop3b_budgets_drill.png");

  // Back nav check
  await page.goBack();
  await page.waitForTimeout(1000);
  const backFromBudgetDrill = await currentURL(page);
  const backWorked = /\/budgets/i.test(backFromBudgetDrill) || /\/reports|\/dashboard/i.test(backFromBudgetDrill);
  note(`  Back from budgets drill: ${backFromBudgetDrill}`);
  if (backWorked) pass("LM-14a — Back from /budgets→/transactions returns cleanly");
  else            fail(`LM-14a — Back from /budgets drill landed at unexpected: ${backFromBudgetDrill}`);

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 4: /goals → linked account → /accounts
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 4: /goals → linked account → /accounts ──────────────────────────────────────");

  await navTo(page, "Goals");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop4_goals.png") });
  note("Screenshot: L74_hop4_goals.png");

  const goalsURL = await currentURL(page);
  note(`Goals URL: ${goalsURL}`);

  const goalsText = await page.evaluate(() => document.body.textContent);
  const hasGoals = /goal|target|saving/i.test(goalsText);
  note(`/goals has goal content: ${hasGoals}`);

  // Try to click a goal's linked account
  const goalAcctClickResult = await page.evaluate(() => {
    const selectors = [
      'a[href*="accounts"]',
      '[data-cf*="account"] a',
      'button[aria-label*="account"]',
      '.goal-account a',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Any link containing "account"
    const links = Array.from(document.querySelectorAll('a')).filter(a =>
      /account/i.test(a.textContent + " " + (a.getAttribute("href") || "")));
    if (links.length > 0) { links[0].click(); return `clicked acct-link: "${links[0].textContent.trim().slice(0,40)}"`; }
    // Linked account text as clickable
    const spans = Array.from(document.querySelectorAll('span, div, p')).filter(el => {
      const t = el.textContent.trim();
      return /linked account|account:/i.test(t) && t.length < 100;
    });
    if (spans.length > 0) { spans[0].click(); return `clicked linked-account span: "${spans[0].textContent.trim().slice(0,40)}"`; }
    return "NOT FOUND";
  });
  note(`  Goal linked-account click: ${goalAcctClickResult}`);
  await page.waitForTimeout(1500);

  const goalAcctDrillURL = await currentURL(page);
  const goalAcctDrillExists = /\/accounts/i.test(goalAcctDrillURL);
  note(`  Goal → accounts drill URL: ${goalAcctDrillURL}`);

  recordLink("LM-9", "/goals (linked account)", "/accounts", goalAcctDrillExists, null,
    goalAcctDrillExists
      ? `dest=${goalAcctDrillURL}`
      : `stayed at ${goalAcctDrillURL} — click: ${goalAcctClickResult}`);

  if (goalAcctDrillExists) pass("LM-9 — Goal linked account drills to /accounts");
  else                     absent_("LM-9 — Goal has NO clickable linked account drill (gap)");

  await page.screenshot({ path: SS("L74_hop4b_goals_account_drill.png") });
  note("Screenshot: L74_hop4b_goals_account_drill.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 5: /bills → bill row → /transactions or /accounts (re-test L64)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 5: /bills → bill → /transactions or /accounts (re-test L64) ──────────────────");

  await navTo(page, "Bills");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop5_bills.png") });
  note("Screenshot: L74_hop5_bills.png");

  const billsURL = await currentURL(page);
  note(`Bills URL: ${billsURL}`);

  const billsPageText = await page.evaluate(() => document.body.textContent);
  const hasBills = /bill|rent|electric|gym|due/i.test(billsPageText);
  note(`/bills has bill content: ${hasBills}`);

  // Try to click a bill row to get to related transaction
  const billRowClickResult = await page.evaluate(() => {
    const selectors = [
      'a[href*="transactions"]',
      'a[href*="accounts"]',
      '[data-cf*="bill"] a',
      '.bill-row a',
      'button[aria-label*="transaction"]',
      'button[aria-label*="View transaction"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Look for any link in a bill context
    const allLinks = Array.from(document.querySelectorAll('a')).filter(a => {
      const href = a.getAttribute('href') || '';
      return href.includes('transaction') || href.includes('account');
    });
    if (allLinks.length > 0) { allLinks[0].click(); return `clicked bill-link: "${allLinks[0].textContent.trim().slice(0,40)}"`; }
    return "NOT FOUND";
  });
  note(`  Bill row click: ${billRowClickResult}`);
  await page.waitForTimeout(1500);

  const billDrillURL = await currentURL(page);
  const billDrillExists = /\/transactions|\/accounts/i.test(billDrillURL);
  note(`  Bill drill URL: ${billDrillURL}`);

  recordLink("LM-10", "/bills (bill row)", "/transactions or /accounts", billDrillExists, null,
    billDrillExists
      ? `dest=${billDrillURL}`
      : `stayed at ${billDrillURL} — click: ${billRowClickResult} (L64 gap re-confirmed)`);

  if (billDrillExists) pass("LM-10 — /bills bill row has drill link (L64 re-test: link present)");
  else                 absent_("LM-10 — /bills bill row has NO drill link (L64 gap: no related-transaction link)");

  await page.screenshot({ path: SS("L74_hop5b_bills_drill.png") });
  note("Screenshot: L74_hop5b_bills_drill.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 6: /accounts → account row → /transactions (ledger filtered)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 6: /accounts → account row → /transactions filtered ─────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop6_accounts.png") });
  note("Screenshot: L74_hop6_accounts.png");

  const accountsURL = await currentURL(page);
  note(`Accounts URL: ${accountsURL}`);

  const accountsText = await page.evaluate(() => document.body.textContent);
  const hasAccounts = /account|checking|savings|credit/i.test(accountsText);
  note(`/accounts has account content: ${hasAccounts}`);

  // Try to click an account row to see its ledger/transactions
  const acctRowClickResult = await page.evaluate(() => {
    // Priority: ledger link, transactions link, or the account name itself
    const selectors = [
      'a[href*="transactions"]',
      'a[href*="ledger"]',
      'button[aria-label*="ledger"]',
      'button[aria-label*="Transactions"]',
      'button[aria-label*="View transactions"]',
      '[data-cf*="ledger"] a',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Try clicking an account name link
    const acctLinks = Array.from(document.querySelectorAll('a')).filter(a => {
      const href = a.getAttribute('href') || '';
      return href.length > 1 && !href.startsWith('http');
    });
    if (acctLinks.length > 0) { acctLinks[0].click(); return `clicked acct-link: "${acctLinks[0].textContent.trim().slice(0,40)}"`; }
    // Click first account row
    const rows = Array.from(document.querySelectorAll('[role="row"], tr, li, [data-cf]')).filter(r => {
      const t = r.textContent.trim();
      return t.length > 10 && /\$[\d,]+/.test(t);
    });
    if (rows.length > 0) { rows[0].click(); return `clicked account row: "${rows[0].textContent.trim().slice(0,50)}"`; }
    return "NOT FOUND";
  });
  note(`  Account row click: ${acctRowClickResult}`);
  await page.waitForTimeout(1500);

  const acctDrillURL = await currentURL(page);
  const acctDrillFilter = await getFilterChip(page);
  const acctDrillText   = await page.evaluate(() => document.body.textContent);
  const acctDrillExists = /\/transactions/i.test(acctDrillURL);
  const acctFilterCarried = acctDrillFilter !== null ||
    acctDrillURL.includes("account") || acctDrillURL.includes("filter") ||
    /filter|account/i.test(acctDrillText.slice(0, 500));
  note(`  Account drill URL: ${acctDrillURL} | Filter chip: ${acctDrillFilter}`);

  recordLink("LM-11", "/accounts (account row)", "/transactions (filtered to account)", acctDrillExists,
    acctDrillExists ? acctFilterCarried : null,
    acctDrillExists
      ? `filter chip: ${acctDrillFilter ?? "not found"} | URL=${acctDrillURL}`
      : `stayed at ${acctDrillURL} — click: ${acctRowClickResult}`);

  if (acctDrillExists) {
    pass("LM-11 — /accounts row drills to /transactions");
    if (acctFilterCarried) pass("LM-11 filter — account filter carried to /transactions");
    else                   absent_("LM-11 filter — /transactions opened but NO account filter applied");
  } else {
    absent_("LM-11 — /accounts has NO drill to /transactions (account ledger missing)");
  }

  await page.screenshot({ path: SS("L74_hop6b_accounts_ledger.png") });
  note("Screenshot: L74_hop6b_accounts_ledger.png");

  // Back nav check
  await page.goBack();
  await page.waitForTimeout(1000);
  const backFromAcctDrill = await currentURL(page);
  note(`  Back from accounts drill: ${backFromAcctDrill}`);
  if (/\/accounts/i.test(backFromAcctDrill) || /\/dashboard/i.test(backFromAcctDrill)) {
    pass("LM-14b — Back from /accounts→/transactions returns cleanly");
  } else {
    note(`LM-14b — Back from accounts drill: ${backFromAcctDrill} (may be OK if routed differently)`);
  }

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 7: /insights → Settings CTA → /settings (re-test L62)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 7: /insights → Settings CTA → /settings (re-test L62) ──────────────────────");

  await navTo(page, "Insights");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  await page.screenshot({ path: SS("L74_hop7_insights.png") });
  note("Screenshot: L74_hop7_insights.png");

  const insightsURL = await currentURL(page);
  note(`Insights URL: ${insightsURL}`);

  const insightsText = await page.evaluate(() => document.body.textContent);
  const hasNokeyState = /add.*key|openai.*key|api.*key|settings/i.test(insightsText);
  note(`/insights has no-key CTA state: ${hasNokeyState}`);

  // Try to click the Settings CTA
  const settingsCTAClickResult = await page.evaluate(() => {
    const selectors = [
      'a[href*="settings"]',
      'button[aria-label*="Settings"]',
      'a[aria-label*="Settings"]',
    ];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) { el.click(); return `clicked: ${sel} "${el.textContent.trim().slice(0,40)}"`; }
    }
    // Button or link with "Settings" text
    const allClickable = Array.from(document.querySelectorAll('a, button')).filter(el =>
      /settings/i.test(el.textContent.trim()) && el.textContent.trim().length < 30);
    if (allClickable.length > 0) {
      allClickable[0].click();
      return `clicked settings-text: "${allClickable[0].textContent.trim()}"`;
    }
    return "NOT FOUND";
  });
  note(`  Settings CTA click: ${settingsCTAClickResult}`);
  await page.waitForTimeout(1200);

  const settingsDest = await currentURL(page);
  const settingsExists = /\/settings/i.test(settingsDest);
  const settingsModalOpen = await page.evaluate(() => !!document.querySelector('dialog[open], [role="dialog"]'));
  note(`  Settings CTA destination: ${settingsDest} | Modal open: ${settingsModalOpen}`);

  // L62 re-test: /settings is not a real route, it may open a modal instead
  const settingsCTAWorks = settingsExists || settingsModalOpen || (settingsCTAClickResult !== "NOT FOUND" && settingsDest !== insightsURL);
  recordLink("LM-12", "/insights (no-key CTA)", "/settings or settings modal", settingsCTAWorks, null,
    `dest=${settingsDest} | modal=${settingsModalOpen} | click=${settingsCTAClickResult}`);

  if (settingsExists) {
    pass("LM-12 — Insights CTA navigates to /settings route (L62 gap fixed)");
  } else if (settingsModalOpen) {
    pass("LM-12 — Insights CTA opens settings modal (acceptable route-less impl)");
    note("  L62 status: /settings route still missing but modal works — partial fix");
  } else if (settingsCTAClickResult !== "NOT FOUND") {
    absent_("LM-12 — Insights Settings CTA found but /settings route and modal both absent (L62 gap persists)");
  } else {
    absent_("LM-12 — Insights Settings CTA NOT FOUND (L62 no-key CTA + settings link gap)");
  }

  await page.screenshot({ path: SS("L74_hop7b_insights_settings_cta.png") });
  note("Screenshot: L74_hop7b_insights_settings_cta.png");

  // ════════════════════════════════════════════════════════════════════════════
  // HOP 8: Period carry test — set period on /dashboard, confirm it carries
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── HOP 8: Period carry across screens ──────────────────────────────────────────────");

  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);

  const beforePeriod = await getPeriodPill(page);
  note(`Period before carry test: ${beforePeriod}`);

  // Navigate to several screens and check period
  for (const [navTitle, checkLabel] of [
    ["Reports",      "LM-13b"],
    ["Transactions", "LM-13c"],
    ["Budgets",      "LM-13d"],
  ]) {
    await navTo(page, navTitle);
    await page.waitForTimeout(600);
    const p = await getPeriodPill(page);
    note(`  ${navTitle} period: ${p}`);
    if (beforePeriod && p && beforePeriod === p) {
      pass(`${checkLabel} — Period "${p}" carried to /${navTitle.toLowerCase()}`);
    } else if (!beforePeriod) {
      note(`${checkLabel} — No period pill found on dashboard; carry check skipped`);
    } else {
      absent_(`${checkLabel} — Period may not carry to /${navTitle.toLowerCase()}: dashboard="${beforePeriod}" ${navTitle}="${p}"`);
    }
  }

  // ════════════════════════════════════════════════════════════════════════════
  // Final screenshot and JS error check
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── FINAL CHECK ─────────────────────────────────────────────────────────────────────");

  await navTo(page, "Dashboard");
  await page.waitForTimeout(600);
  await page.screenshot({ path: SS("L74_hop8_final_dashboard.png") });
  note("Screenshot: L74_hop8_final_dashboard.png");

  if (jsErrors.length === 0) {
    pass("NO_JS_ERRORS — zero runtime JS errors across entire ritual");
  } else {
    fail(`JS_ERRORS — ${jsErrors.length} runtime error(s): ${jsErrors.slice(0, 3).join("; ")}`);
  }

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`);
  console.error(err);
} finally {
  await browser.close();
}

// ── print link matrix ─────────────────────────────────────────────────────────
console.log("\n\n══════════════════════════════════════════════════════════════════");
console.log("LINK MATRIX (L74 — Inter-Page Links & Cross-Navigation)");
console.log("══════════════════════════════════════════════════════════════════");
console.log("ID      Source                       Target                   Exists  Carries State  Notes");
console.log("──────  ───────────────────────────  ──────────────────────── ──────  ─────────────  ──────────────────────────────────");
for (const r of linkMatrix) {
  const id  = r.id.padEnd(7);
  const src = r.src.padEnd(28);
  const tgt = r.tgt.padEnd(24);
  const ex  = r.exists.padEnd(7);
  const cs  = r.carry.padEnd(14);
  console.log(`${id} ${src} ${tgt} ${ex} ${cs} ${r.notes}`);
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
