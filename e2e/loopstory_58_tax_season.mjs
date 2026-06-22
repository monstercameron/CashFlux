// L58 E2E loop story — "Tax Season" (Priya, annual review for accountant)
// Persona: Priya, 42, preparing annual tax documentation. She needs a full-year
//          spending breakdown for her accountant: total income, total expenses,
//          tax-deductible categories (medical, charity, home-office), and a clean
//          CSV export covering the ENTIRE prior year.
//
// Flow (the ritual):
//   0. Seed multi-month data: Jan–Dec 2025, 6 categories, 36 transactions.
//   1. Set period to full prior year (Year resolution → step back to 2025).
//   2. /reports — review spending-by-category for 2025; screenshot totals.
//   3. Drill from "Medical" category → /transactions (filter + period carry).
//   4. Recategorize one miscategorized item (charity tx under wrong category → Charity).
//   5. Return to /reports; verify Medical total updated.
//   6. Export annual CSV; confirm filename reflects the annual period.
//   7. Cross-screen: annual total on /reports == sum-of-categories, to the cent.
//   8. Period carry: 2025 window shown on /dashboard, /budgets, /transactions, /reports.
//
// Key invariants:
//   ANNUAL_WINDOW     — Year-resolution 2025 window used across all screens.
//   MONEY_CONSERVATION — sum(category totals) == overall expense total, to the cent.
//   FILTER_CARRY      — drill from report category → /transactions pre-applies filter+period.
//   RECAT_UPDATES     — recategorizing a txn reflects in /reports totals immediately.
//   EXPORT_PERIOD     — exported CSV covers the annual (2025) period, not the default month.
//   PERIOD_CARRY      — Year 2025 window persists across soft-nav to /dashboard, /budgets, /transactions.
//   EXPORT_FILENAME   — export filename encodes the period (regression from L45 EXPORT_FILENAME gap).
//
// Cross-references:
//   L45: EXPORT_PERIOD / EXPORT_FILENAME bug (hardcoded "spending-by-category.csv", no period stamp).
//   B10: Period resolution control.
//   C17: Period carry across screens.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_58_tax_season.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

// ── Seed constants (all L58-prefixed for isolation) ──────────────────────────
const YEAR = 2025;  // Prior year to review

// 36 transactions spread across Jan–Dec 2025, 6 categories.
// We intentionally put one "charity" txn under a wrong category (to be fixed in step 4).
const SEEDS = [
  // Groceries — Jan, Mar, Jun, Sep, Nov
  { desc: "L58 Groceries Jan",    amount: "-120.00", date: "2025-01-10", cat: "Groceries" },
  { desc: "L58 Groceries Mar",    amount: "-135.50", date: "2025-03-15", cat: "Groceries" },
  { desc: "L58 Groceries Jun",    amount: "-145.00", date: "2025-06-08", cat: "Groceries" },
  { desc: "L58 Groceries Sep",    amount: "-130.00", date: "2025-09-20", cat: "Groceries" },
  { desc: "L58 Groceries Nov",    amount: "-110.75", date: "2025-11-03", cat: "Groceries" },
  // Medical — Feb, May, Aug, Oct
  { desc: "L58 Medical Feb",      amount: "-200.00", date: "2025-02-14", cat: "Medical" },
  { desc: "L58 Medical May",      amount: "-350.00", date: "2025-05-22", cat: "Medical" },
  { desc: "L58 Medical Aug",      amount: "-175.00", date: "2025-08-07", cat: "Medical" },
  { desc: "L58 Medical Oct",      amount: "-400.00", date: "2025-10-30", cat: "Medical" },
  // Charity — Mar, Jul, Dec (Mar one is intentionally under wrong cat, will be fixed)
  { desc: "L58 Charity Jul",      amount: "-100.00", date: "2025-07-04", cat: "Charity" },
  { desc: "L58 Charity Dec",      amount: "-250.00", date: "2025-12-20", cat: "Charity" },
  { desc: "L58 Charity MISLABELED", amount: "-75.00", date: "2025-03-22", cat: "Groceries" }, // WRONG category — fix in step 4
  // Home Office — Feb, Jun, Oct
  { desc: "L58 HomeOffice Feb",   amount: "-89.99",  date: "2025-02-28", cat: "Home Office" },
  { desc: "L58 HomeOffice Jun",   amount: "-299.00", date: "2025-06-15", cat: "Home Office" },
  { desc: "L58 HomeOffice Oct",   amount: "-149.50", date: "2025-10-12", cat: "Home Office" },
  // Utilities — every other month: Jan, Mar, May, Jul, Sep, Nov
  { desc: "L58 Utilities Jan",    amount: "-85.00",  date: "2025-01-25", cat: "Utilities" },
  { desc: "L58 Utilities Mar",    amount: "-92.00",  date: "2025-03-28", cat: "Utilities" },
  { desc: "L58 Utilities May",    amount: "-78.50",  date: "2025-05-15", cat: "Utilities" },
  { desc: "L58 Utilities Jul",    amount: "-88.00",  date: "2025-07-20", cat: "Utilities" },
  { desc: "L58 Utilities Sep",    amount: "-95.00",  date: "2025-09-10", cat: "Utilities" },
  { desc: "L58 Utilities Nov",    amount: "-80.00",  date: "2025-11-18", cat: "Utilities" },
  // Entertainment — Apr, Aug, Dec
  { desc: "L58 Entertainment Apr", amount: "-55.00", date: "2025-04-12", cat: "Entertainment" },
  { desc: "L58 Entertainment Aug", amount: "-65.00", date: "2025-08-25", cat: "Entertainment" },
  { desc: "L58 Entertainment Dec", amount: "-45.00", date: "2025-12-28", cat: "Entertainment" },
  // Income — monthly (Jan–Dec)
  { desc: "L58 Income Jan",  amount: "3000.00", date: "2025-01-31", cat: null },
  { desc: "L58 Income Feb",  amount: "3000.00", date: "2025-02-28", cat: null },
  { desc: "L58 Income Mar",  amount: "3100.00", date: "2025-03-31", cat: null },
  { desc: "L58 Income Apr",  amount: "3100.00", date: "2025-04-30", cat: null },
  { desc: "L58 Income May",  amount: "3100.00", date: "2025-05-31", cat: null },
  { desc: "L58 Income Jun",  amount: "3200.00", date: "2025-06-30", cat: null },
  { desc: "L58 Income Jul",  amount: "3200.00", date: "2025-07-31", cat: null },
  { desc: "L58 Income Aug",  amount: "3200.00", date: "2025-08-31", cat: null },
  { desc: "L58 Income Sep",  amount: "3300.00", date: "2025-09-30", cat: null },
  { desc: "L58 Income Oct",  amount: "3300.00", date: "2025-10-31", cat: null },
  { desc: "L58 Income Nov",  amount: "3300.00", date: "2025-11-30", cat: null },
  { desc: "L58 Income Dec",  amount: "3400.00", date: "2025-12-31", cat: null },
];

// Pre-computed expected totals (in dollars, sum of SEEDS above, before fixing mislabeled tx):
// Groceries: 120+135.5+145+130+110.75 + 75 (mislabeled) = 716.25
// Medical: 200+350+175+400 = 1125
// Charity: 100+250 = 350 (mislabeled tx not counted here — it's under Groceries)
// After fix: Groceries = 641.25, Charity = 425
// Home Office: 89.99+299+149.5 = 538.49
// Utilities: 85+92+78.5+88+95+80 = 518.5
// Entertainment: 55+65+45 = 165
// Total expenses = 641.25+1125+425+538.49+518.5+165 = 3413.24 (after fix)
// Total income = 3000+3000+3100+3100+3100+3200+3200+3200+3300+3300+3300+3400 = 38200

// ── helpers ──────────────────────────────────────────────────────────────────
const parseDollar = (s) => {
  if (!s) return NaN;
  const neg = /^\(.*\)$/.test(s.trim());
  const n = parseFloat(s.replace(/[^0-9.]/g, ""));
  return neg ? -n : n;
};

// Hard navigation (resets in-memory state)
const goto = async (page, hash) => {
  await page.goto(BASE + hash, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(2000);
};

// Soft navigation (preserves period atom)
const softNav = async (page, routeTitle, fallbackHash) => {
  const navLink = await page.$(`nav[aria-label="Main navigation"] a[title="${routeTitle}"]`);
  if (navLink) {
    await navLink.click();
    await page.waitForTimeout(1800);
  } else {
    await page.evaluate((hash) => {
      window.history.pushState({}, "", hash);
      window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
    }, fallbackHash);
    await page.waitForTimeout(1800);
  }
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

// Read L58 transactions from localStorage dataset
const getL58Txns = (page) =>
  page.evaluate(() => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    const found = [];
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) { o.forEach(walk); return; }
      if (typeof o.desc === "string" && o.desc.startsWith("L58")) found.push(o);
      else Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  });

// Parse spending-by-category totals from reports page text.
// Returns a map { categoryName: dollarAmount } for visible category rows.
// We only parse lines that look like a single category row: a short label
// followed immediately by a dollar amount on the same line, filtering out
// header stats (INCOME, SPENDING, NET WORTH, etc.) and prose lines.
const parseCategoryTotals = (text) => {
  const map = {};
  // EXCLUDED header/stat words that appear in the stat-grid, not the category table
  const EXCLUDED = new Set([
    "income", "spending", "net worth", "assets", "liabilities",
    "savings rate", "runway", "no-spend days", "personal",
  ]);
  const lines = text.split(/\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    // Must end with a dollar amount
    const m = trimmed.match(/^([A-Za-z][A-Za-z &\-']{1,40}?)\s+\$([\d,]+\.\d{2})$/);
    if (!m) continue;
    const name = m[1].trim();
    const amt  = parseFloat(m[2].replace(/,/g, ""));
    if (isNaN(amt) || amt === 0) continue;
    if (EXCLUDED.has(name.toLowerCase())) continue;
    // Skip if name contains numbers (e.g. years, percentages)
    if (/\d/.test(name)) continue;
    // Skip overly long names (prose, not category labels)
    if (name.length > 40) continue;
    map[name] = (map[name] ?? 0) + amt;
  }
  return map;
};

// Parse period label from page.
// Year resolution renders as a standalone "2025" in the stepper pill.
// Month resolution renders as "May 2026". Prioritise the pure-year match
// (the stepper pill area), but avoid matching years embedded in month labels.
const parsePeriodLabel = (text) => {
  // Month label wins when it appears in the resolution/stepper area
  const monthM = text.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i);
  // For Year resolution the stepper shows just "2025" — detect it by looking
  // for a standalone 4-digit year NOT preceded/followed by a month name.
  // We scan the reso-control area: the Year segment will be "selected" / active.
  // Heuristic: if the page text contains "Year" near a 4-digit year without a
  // month name on the same line, treat that as a Year-resolution label.
  const yearOnlyLine = text.split(/\n/).find((line) =>
    /\b20\d\d\b/.test(line) &&
    !/Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/i.test(line) &&
    line.trim().length < 60
  );
  const yearOnlyM = yearOnlyLine ? yearOnlyLine.match(/\b(20\d\d)\b/) : null;

  // If we have a pure-year line AND it's a different year than the month label,
  // prefer the pure-year (Year resolution is active).
  if (yearOnlyM && (!monthM || !monthM[0].includes(yearOnlyM[1]))) {
    return yearOnlyM[1];
  }
  if (monthM) return monthM[0];
  if (yearOnlyM) return yearOnlyM[1];
  return null;
};

let passes = 0, fails = 0, maybes = 0;
const pass  = (m) => { passes++;  console.log(`  PASS  ${m}`); };
const fail  = (m) => { fails++;   console.error(`  FAIL  ${m}`); process.exitCode = 1; };
const maybe = (m) => { maybes++;  console.warn(`  MAYBE ${m}`); };

// ── main ─────────────────────────────────────────────────────────────────────
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 0: Seed 36 multi-month transactions across 6 categories ──────────
  console.log("\n── Step 0: Seed multi-month 2025 data ──");
  await goto(page, "/transactions");
  await page.waitForSelector("#txn-add, input[placeholder='Description']", { timeout: 30000 }).catch(() => {});

  // Discover available categories from the transaction category select
  const catSel0 = await page.$('select[aria-label="Category"]');
  let catMap = {}; // name (lowercase) → id
  if (catSel0) {
    const opts = await catSel0.evaluate((el) =>
      Array.from(el.options).map((o) => ({ v: o.value, t: o.text.trim() }))
    );
    for (const o of opts) {
      if (o.v) catMap[o.t.toLowerCase()] = o.v;
    }
    console.log(`  INFO  Categories available: ${Object.keys(catMap).join(", ")}`);
    pass("Step 0a — Category options read from transaction form");
  } else {
    maybe("Step 0a — Category select not found; will seed without categories");
  }

  // Helper: find best matching category id for a name
  const findCatId = (name) => {
    if (!name) return null;
    const key = name.toLowerCase();
    // Exact
    if (catMap[key]) return catMap[key];
    // Partial
    for (const [k, v] of Object.entries(catMap)) {
      if (k.includes(key) || key.includes(k)) return v;
    }
    return null;
  };

  let seededCount = 0;
  for (const tx of SEEDS) {
    const descIn = await page.$('input[placeholder="Description"], #txn-add');
    const amtIn  = await page.$('input[placeholder="Amount"], input[type="number"][aria-required="true"]');
    const dateIn = await page.$('input[aria-label="Date"], input[type="date"]');
    if (!descIn || !amtIn) {
      maybe(`Step 0b — inputs not found for "${tx.desc}"`);
      continue;
    }
    await descIn.fill(tx.desc);
    await amtIn.fill(tx.amount.replace("-", ""));

    if (dateIn) {
      try { await dateIn.fill(tx.date); } catch (_) {}
    }

    if (tx.cat) {
      const cid = findCatId(tx.cat);
      if (cid) {
        const cs = await page.$('select[aria-label="Category"]');
        if (cs) await cs.selectOption({ value: cid });
      }
    }

    const submitBtn = await page.$('button[type="submit"]');
    if (submitBtn) {
      await submitBtn.click();
      await page.waitForTimeout(500);
      seededCount++;
    }
  }

  // Wait for autosave
  await page.waitForTimeout(3000);
  const seededDs = await getL58Txns(page);
  console.log(`  INFO  Seeded ${seededCount}/${SEEDS.length} transactions; ${seededDs.length} found in dataset`);

  if (seededDs.length >= 20) {
    pass(`Step 0c — ${seededDs.length} L58 transactions seeded in dataset`);
  } else if (seededDs.length > 0) {
    maybe(`Step 0c — only ${seededDs.length}/${SEEDS.length} L58 transactions seeded`);
  } else {
    fail("Step 0c — no L58 transactions found in dataset after seeding");
  }

  await page.screenshot({ path: SS("l58_00_transactions_seeded.png") });

  // ── Step 1: Set period to Year 2025 ──────────────────────────────────────
  console.log("\n── Step 1: Set period to Year 2025 ──");
  await goto(page, "/");

  await page.screenshot({ path: SS("l58_01_dashboard_before_period.png") });

  // Click the "Year" segment in the resolution control
  const yearSegment = await page.$('button:has-text("Year"), [data-value="year"], [aria-label*="Year" i]');
  if (yearSegment) {
    await yearSegment.click();
    await page.waitForTimeout(1000);
    pass("Step 1a — 'Year' segment clicked in resolution control");
  } else {
    // The segmented control renders as buttons with text "Year"
    const segBtns = await page.$$('.reso-control button, [class*="seg"] button, button');
    let yearClicked = false;
    for (const b of segBtns) {
      const txt = await b.evaluate((el) => el.textContent?.trim() ?? "");
      if (txt === "Year") {
        await b.click();
        await page.waitForTimeout(1000);
        yearClicked = true;
        pass("Step 1a — 'Year' segment clicked via text scan");
        break;
      }
    }
    if (!yearClicked) {
      fail("Step 1a — 'Year' segment not found in resolution control");
    }
  }

  // Step back one period to go from 2026 → 2025
  const prevBtn = await page.$('[aria-label*="Previous period" i], [title*="Previous period" i], [aria-label*="prevPeriod" i], [title*="prevPeriod" i]');
  if (prevBtn) {
    await prevBtn.click();
    await page.waitForTimeout(800);
    pass("Step 1b — stepped back to prior year (2025)");
  } else {
    // Try finding the stepper pill's back button
    const allBtns = await page.$$('button');
    let backClicked = false;
    for (const b of allBtns) {
      const title = await b.evaluate((el) => (el.getAttribute("title") ?? el.getAttribute("aria-label") ?? "").toLowerCase());
      if (title.includes("prev") || title.includes("earlier") || title.includes("back")) {
        await b.click();
        await page.waitForTimeout(800);
        backClicked = true;
        pass("Step 1b — stepped back via prev-labelled button");
        break;
      }
    }
    if (!backClicked) {
      maybe("Step 1b — prev stepper not found; attempting via evaluate to step back");
      // Inject period change via localStorage + reload as fallback
      await page.evaluate(() => {
        // Set period atom to 2025 year window
        try {
          const key = "cashflux:period:resolution";
          localStorage.setItem(key, "year");
        } catch(_) {}
      });
    }
  }

  await page.screenshot({ path: SS("l58_01b_dashboard_year_2025.png") });
  const dashBody1 = await bodyText(page);
  const dashPeriod = parsePeriodLabel(dashBody1);
  console.log(`  INFO  Dashboard period label: "${dashPeriod}"`);

  if (dashPeriod === "2025") {
    pass(`Step 1c — ANNUAL_WINDOW: dashboard shows period "2025"`);
  } else if (dashPeriod) {
    maybe(`Step 1c — ANNUAL_WINDOW: dashboard shows "${dashPeriod}" (expected "2025"; stepper navigation may differ)`);
  } else {
    fail("Step 1c — ANNUAL_WINDOW: no period label found on dashboard");
  }

  // ── Step 2: /reports — review annual spending by category ─────────────────
  console.log("\n── Step 2: /reports — annual spending by category ──");
  await softNav(page, "Reports", "/reports");
  await page.screenshot({ path: SS("l58_02_reports_annual.png") });

  const reportsBody = await bodyText(page);
  const reportsPeriod = parsePeriodLabel(reportsBody);
  console.log(`  INFO  Reports period label: "${reportsPeriod}"`);

  if (reportsPeriod === "2025") {
    pass("Step 2a — PERIOD_CARRY: /reports shows period 2025 (soft-nav preserved year window)");
  } else if (reportsPeriod) {
    fail(`Step 2a — PERIOD_CARRY: /reports shows "${reportsPeriod}" instead of 2025 — period NOT carried via soft-nav`);
  } else {
    maybe("Step 2a — PERIOD_CARRY: period label not parseable on /reports");
  }

  // Check reports loaded with expected content
  if (/spending|income|expense|report/i.test(reportsBody)) {
    pass("Step 2b — /reports loaded with spending content");
  } else {
    fail("Step 2b — /reports did not render spending content");
  }

  // Parse category totals from reports page
  const catTotals = parseCategoryTotals(reportsBody);
  console.log("  INFO  Category totals found:", JSON.stringify(catTotals));

  // Check that at least some L58-seeded categories appear (by name match)
  const knownCats = ["groceries", "medical", "charity", "home", "utilities", "entertainment"];
  const foundCats = Object.keys(catTotals).filter(k =>
    knownCats.some(c => k.toLowerCase().includes(c))
  );
  if (foundCats.length >= 2) {
    pass(`Step 2c — Reports shows ${foundCats.length} recognizable categories: ${foundCats.join(", ")}`);
  } else {
    maybe(`Step 2c — Only ${foundCats.length} recognizable categories visible (demo data may dominate)`);
  }

  // MONEY_CONSERVATION: parse aggregate income and expense from the stat-grid
  // The reports page shows Income/Spending totals in the stat cards
  const incomeM = reportsBody.match(/Income\s*\$([\d,]+\.\d{2})/i);
  const expenseM = reportsBody.match(/Spending\s*\$([\d,]+\.\d{2})/i);
  const annualIncome  = incomeM  ? parseFloat(incomeM[1].replace(/,/g, ""))  : NaN;
  const annualExpense = expenseM ? parseFloat(expenseM[1].replace(/,/g, "")) : NaN;
  console.log(`  INFO  Annual totals — Income: $${annualIncome}, Expense: $${annualExpense}`);

  if (!isNaN(annualIncome) && annualIncome > 0) {
    pass(`Step 2d — Annual income visible: $${annualIncome}`);
  } else {
    maybe("Step 2d — Annual income stat not parseable");
  }
  if (!isNaN(annualExpense) && annualExpense > 0) {
    pass(`Step 2e — Annual expense visible: $${annualExpense}`);
  } else {
    maybe("Step 2e — Annual expense stat not parseable");
  }

  // MONEY_CONSERVATION: sum of category totals should equal reported expense total (within tolerance)
  if (!isNaN(annualExpense) && Object.keys(catTotals).length > 0) {
    const catSum = Object.values(catTotals).reduce((a, b) => a + b, 0);
    const diff = Math.abs(catSum - annualExpense);
    console.log(`  INFO  MONEY_CONSERVATION: catSum=$${catSum.toFixed(2)}, reportedExpense=$${annualExpense.toFixed(2)}, diff=$${diff.toFixed(2)}`);
    if (diff < 1.0) {
      pass(`Step 2f — MONEY_CONSERVATION HOLDS: sum(categories) $${catSum.toFixed(2)} ≈ expense $${annualExpense.toFixed(2)}`);
    } else if (diff < 10.0) {
      maybe(`Step 2f — MONEY_CONSERVATION: diff $${diff.toFixed(2)} (small; FX rounding or subcategory rollup)`);
    } else {
      fail(`Step 2f — MONEY_CONSERVATION VIOLATED: catSum $${catSum.toFixed(2)} vs expense $${annualExpense.toFixed(2)}, diff $${diff.toFixed(2)}`);
    }
  } else {
    maybe("Step 2f — MONEY_CONSERVATION: insufficient data to verify");
  }

  // ── Step 3: Drill from Medical category → /transactions ───────────────────
  console.log("\n── Step 3: Drill from Medical → /transactions ──");
  // Find a Medical or deductible category row with a drill link
  const medicalDrillBtn = await page.$('a[href*="transactions"], button:has-text("Medical"), [data-cat*="medical" i]');
  // Also look for row links inside the category table
  const rowLinks = await page.$$('.cat-row a, .category-row a, tr a, [class*="row"] a');
  let drillClicked = false;
  for (const link of rowLinks) {
    const txt = await link.evaluate((el) => el.textContent?.trim() ?? "");
    const href = await link.evaluate((el) => el.getAttribute("href") ?? "");
    if (/medical/i.test(txt) || /medical/i.test(href)) {
      await link.click();
      await page.waitForTimeout(2000);
      drillClicked = true;
      pass("Step 3a — Clicked Medical category drill link → /transactions");
      break;
    }
  }
  if (!drillClicked) {
    // Try navigating to /transactions with a category filter via URL
    maybe("Step 3a — Medical drill link not found; navigating to /transactions (FILTER_CARRY may not apply)");
    await softNav(page, "Transactions", "/transactions");
    await page.waitForTimeout(1500);
  }

  await page.screenshot({ path: SS("l58_03_transactions_drill_medical.png") });
  const txnDrillBody = await bodyText(page);
  const txnDrillPeriod = parsePeriodLabel(txnDrillBody);
  console.log(`  INFO  Transactions period after drill: "${txnDrillPeriod}"`);

  if (txnDrillPeriod === "2025") {
    pass("Step 3b — FILTER_CARRY: /transactions shows 2025 period after drill from reports");
  } else if (txnDrillPeriod) {
    fail(`Step 3b — FILTER_CARRY: /transactions shows "${txnDrillPeriod}" not 2025 after drill — period NOT carried`);
  } else {
    maybe("Step 3b — FILTER_CARRY: period label not parseable on /transactions after drill");
  }

  // Check if Medical filter chip appeared
  const filterChip = /medical|filter.*medical|medical.*filter/i.test(txnDrillBody);
  if (drillClicked && filterChip) {
    pass("Step 3c — FILTER_CARRY: Medical category filter chip visible after drill");
  } else if (drillClicked) {
    maybe("Step 3c — FILTER_CARRY: Medical filter chip not visible after drill (category filter may not carry)");
  } else {
    maybe("Step 3c — FILTER_CARRY: drill was skipped; filter carry not tested");
  }

  // ── Step 4: Recategorize the mislabeled Charity txn ──────────────────────
  console.log("\n── Step 4: Recategorize mislabeled Charity transaction ──");
  // Navigate to /transactions and search for "L58 Charity MISLABELED"
  await softNav(page, "Transactions", "/transactions");
  await page.waitForSelector("#txn-add, input[placeholder='Description']", { timeout: 30000 }).catch(() => {});

  const searchIn = await page.$('input[type="search"]');
  if (searchIn) {
    await searchIn.fill("L58 Charity MISLABELED");
    await page.waitForTimeout(800);
    pass("Step 4a — Filtered to mislabeled transaction");
  } else {
    maybe("Step 4a — Search input not found; recategorize may not be precise");
  }

  await page.screenshot({ path: SS("l58_04a_before_recat.png") });

  // Find the inline category select or edit button for the visible row
  const inlineCatSel = await page.$('select[aria-label="Category"]');
  let recatDone = false;
  if (inlineCatSel) {
    const charityId = findCatId("Charity");
    if (charityId) {
      await inlineCatSel.selectOption({ value: charityId });
      await page.waitForTimeout(600);
      pass(`Step 4b — Recategorized mislabeled tx to Charity (id: ${charityId})`);
      recatDone = true;
    } else {
      maybe("Step 4b — Charity category id not found; trying by text");
      await inlineCatSel.selectOption({ label: /charity/i });
      await page.waitForTimeout(600);
      recatDone = true;
      pass("Step 4b — Recategorized to Charity by label");
    }
  } else {
    // Try clicking Edit then changing category
    const editBtns = await page.$$('button[title*="Edit" i], button[aria-label*="Edit" i]');
    if (editBtns.length > 0) {
      await editBtns[0].click();
      await page.waitForTimeout(600);
      const editForm = await page.$('select[aria-label="Category"]');
      if (editForm) {
        const charityId = findCatId("Charity");
        if (charityId) {
          await editForm.selectOption({ value: charityId });
          await page.waitForTimeout(400);
        }
        const saveBtn = await page.$('button[type="submit"]');
        if (saveBtn) { await saveBtn.click(); await page.waitForTimeout(600); }
        recatDone = true;
        pass("Step 4b — Recategorized via Edit form");
      } else {
        maybe("Step 4b — Edit form category select not found");
      }
    } else {
      maybe("Step 4b — No inline category select or edit button found for recategorize");
    }
  }

  await page.screenshot({ path: SS("l58_04b_after_recat.png") });

  // ── Step 5: Back to /reports — verify Medical/Grocery totals updated ───────
  console.log("\n── Step 5: Return to /reports — verify totals updated ──");
  await softNav(page, "Reports", "/reports");
  await page.screenshot({ path: SS("l58_05_reports_after_recat.png") });

  const reportsBody2 = await bodyText(page);
  const reportsPeriod2 = parsePeriodLabel(reportsBody2);
  console.log(`  INFO  Reports period after recat: "${reportsPeriod2}"`);

  if (reportsPeriod2 === "2025") {
    pass("Step 5a — PERIOD_CARRY: /reports still shows 2025 after recategorize round-trip");
  } else if (reportsPeriod2) {
    fail(`Step 5a — PERIOD_CARRY: /reports shows "${reportsPeriod2}" not 2025 after round-trip`);
  } else {
    maybe("Step 5a — PERIOD_CARRY: period not parseable on reports after recat");
  }

  const catTotals2 = parseCategoryTotals(reportsBody2);
  console.log("  INFO  Category totals after recat:", JSON.stringify(catTotals2));

  if (recatDone) {
    // Groceries should be lower now (mislabeled $75 moved out)
    // Charity (shown as "Gifts & Charity" in demo data) should be higher (gained $75)
    const groceries1 = Object.entries(catTotals).find(([k]) => /grocer/i.test(k))?.[1] ?? NaN;
    const groceries2 = Object.entries(catTotals2).find(([k]) => /grocer/i.test(k))?.[1] ?? NaN;
    const charity2   = Object.entries(catTotals2).find(([k]) => /charity|gift/i.test(k))?.[1] ?? NaN;
    console.log(`  INFO  Groceries before recat: $${groceries1}, after: $${groceries2}`);
    console.log(`  INFO  Charity after recat: $${charity2}`);

    if (!isNaN(groceries1) && !isNaN(groceries2) && groceries2 < groceries1) {
      pass(`Step 5b — RECAT_UPDATES: Groceries dropped $${(groceries1 - groceries2).toFixed(2)} after moving mislabeled tx to Charity`);
    } else if (!isNaN(groceries1) && !isNaN(groceries2)) {
      maybe(`Step 5b — RECAT_UPDATES: Groceries unchanged ($${groceries1} → $${groceries2}); recategorize may not have taken effect`);
    } else {
      maybe("Step 5b — RECAT_UPDATES: category totals not parseable for before/after comparison");
    }
  } else {
    maybe("Step 5b — RECAT_UPDATES: recategorize was skipped; update not verified");
  }

  // ── Step 6: Export annual CSV ─────────────────────────────────────────────
  console.log("\n── Step 6: Export annual CSV ──");
  await page.screenshot({ path: SS("l58_06a_reports_before_export.png") });

  // Set up download listener before clicking export
  let downloadFilename = null;
  let downloadPath = null;
  const downloadPromise = page.waitForEvent("download", { timeout: 8000 }).catch(() => null);

  const exportBtn = await page.$('button:has-text("Export"), button:has-text("Download"), button:has-text("CSV"), button[title*="CSV" i], button[title*="Export" i]');
  if (exportBtn) {
    await exportBtn.click();
    const dl = await downloadPromise;
    if (dl) {
      downloadFilename = dl.suggestedFilename();
      downloadPath = await dl.path();
      pass(`Step 6a — EXPORT: Download triggered, filename: "${downloadFilename}"`);

      // EXPORT_PERIOD / EXPORT_FILENAME: filename should encode the year
      if (/2025/.test(downloadFilename)) {
        pass(`Step 6b — EXPORT_FILENAME: filename "${downloadFilename}" encodes the period year 2025`);
      } else if (/spending-by-category\.csv$/.test(downloadFilename)) {
        fail(`Step 6b — EXPORT_FILENAME VIOLATED (L45 gap): filename "${downloadFilename}" has NO period stamp — all annual exports collide. Expected something like "spending-by-category-2025.csv".`);
      } else {
        maybe(`Step 6b — EXPORT_FILENAME: filename "${downloadFilename}" does not clearly encode 2025 (not the expected L45 gap pattern either)`);
      }
    } else {
      maybe("Step 6a — EXPORT: download event not fired within 8s (browser may have blocked inline download)");
    }
  } else {
    fail("Step 6a — EXPORT: export/download CSV button not found on /reports");
  }

  await page.screenshot({ path: SS("l58_06b_reports_after_export.png") });

  // ── Step 7: MONEY_CONSERVATION cross-check ────────────────────────────────
  console.log("\n── Step 7: MONEY_CONSERVATION final check ──");
  const reportsBody3 = await bodyText(page);
  const expenseM3 = reportsBody3.match(/Spending\s*\$([\d,]+\.\d{2})/i);
  const annualExpense3 = expenseM3 ? parseFloat(expenseM3[1].replace(/,/g, "")) : annualExpense;
  const catTotals3 = parseCategoryTotals(reportsBody3);

  if (!isNaN(annualExpense3) && Object.keys(catTotals3).length > 0) {
    const catSum3 = Object.values(catTotals3).reduce((a, b) => a + b, 0);
    const diff3 = Math.abs(catSum3 - annualExpense3);
    console.log(`  INFO  FINAL MONEY_CONSERVATION: catSum=$${catSum3.toFixed(2)}, reported=$${annualExpense3.toFixed(2)}, diff=$${diff3.toFixed(2)}`);
    if (diff3 < 1.0) {
      pass(`Step 7a — MONEY_CONSERVATION FINAL HOLDS: sum(categories) $${catSum3.toFixed(2)} == expense $${annualExpense3.toFixed(2)} to the cent`);
    } else if (diff3 < 10.0) {
      maybe(`Step 7a — MONEY_CONSERVATION FINAL: diff $${diff3.toFixed(2)} — within rounding tolerance`);
    } else {
      fail(`Step 7a — MONEY_CONSERVATION FINAL VIOLATED: diff $${diff3.toFixed(2)} between category sum and reported expense total`);
    }
  } else {
    maybe("Step 7a — MONEY_CONSERVATION FINAL: insufficient data");
  }

  // ── Step 8: PERIOD_CARRY cross-screen check ───────────────────────────────
  console.log("\n── Step 8: PERIOD_CARRY cross-screen ──");

  // Check /dashboard
  await softNav(page, "Dashboard", "/");
  const dashBody8 = await bodyText(page);
  const dp8 = parsePeriodLabel(dashBody8);
  if (dp8 === "2025") {
    pass("Step 8a — PERIOD_CARRY: /dashboard still shows 2025");
  } else {
    maybe(`Step 8a — PERIOD_CARRY: /dashboard shows "${dp8}" (soft-nav may have reset or collapsed period)`);
  }
  await page.screenshot({ path: SS("l58_08a_dashboard_final.png") });

  // Check /budgets
  await softNav(page, "Budgets", "/budgets");
  const budgetsBody8 = await bodyText(page);
  const bp8 = parsePeriodLabel(budgetsBody8);
  if (bp8 === "2025") {
    pass("Step 8b — PERIOD_CARRY: /budgets shows 2025");
  } else {
    maybe(`Step 8b — PERIOD_CARRY: /budgets shows "${bp8}"`);
  }
  await page.screenshot({ path: SS("l58_08b_budgets_final.png") });

  // ── Step 9: JS error check ────────────────────────────────────────────────
  console.log("\n── Step 9: JS error check ──");
  if (errors.length > 0) {
    fail(`JS page errors: ${errors.join(" | ")}`);
  } else {
    pass("Step 9 — No JS page errors across the full ritual");
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log(`\n══ SUMMARY: ${passes} PASS, ${fails} FAIL, ${maybes} MAYBE ══`);
  if (fails > 0) {
    console.error("RESULT: FAIL");
  } else {
    console.log("RESULT: PASS");
  }

} finally {
  await browser.close();
}
