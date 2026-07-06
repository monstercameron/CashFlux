// L68 E2E loop story — "The Gig Worker's Lean Month" (Devon) — 2026-06-22
//
// Persona: Devon is a gig worker (rideshare + delivery) with irregular income.
// Opening checking balance: $120. No steady paycheck — just four deposits that
// trickle in at different dates across June 2026:
//   Deposit 1: $180.00 — June 3  (Lyft payout)
//   Deposit 2: $95.00  — June 9  (DoorDash payout)
//   Deposit 3: $240.00 — June 17 (combined gig payout)
//   Deposit 4: $130.00 — June 24 (Instacart payout)
//   TOTAL INCOME: $645.00
//
// Budgets / obligations:
//   Rent $800 (over-budget — Devon can't make full rent this month)
//   Food $200, Gas $60, Phone $45, Credit card minimum $35
//   One credit card account (minimum payment only)
//
// KEY INVARIANTS ASSERTED:
//   C1: INCOME_SUM — 4 deposits sum to exactly $645.00 everywhere they appear
//       (Dashboard income stat, Budgets income view, Reports) — none dropped or doubled
//   I2: PIECEMEAL_BUDGET — Does the budgets screen handle income arriving in pieces?
//       Can Devon see "left to allocate" from irregular deposits, not just a single salary?
//   I3: FORECAST_BASIS — Does the 12-month forecast use a sane basis for irregular income?
//       Does it still ignore recurring/scheduled items (Thread B re-confirm from irregular angle)?
//   I4: PERIOD_CONSISTENCY — All touched screens agree on the period (current month = June 2026)
//       and show the same $645 total, not different windows.
//
// Screens exercised:
//   /accounts (create Devon Checking + Devon Credit Card) →
//   /transactions (4 income deposits, 1 CC minimum payment expense) →
//   /budgets (income view, "left to allocate") →
//   /planning (forecast card, recurring wiring) →
//   /dashboard (income stat widget) →
//   /reports (income total)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_68_gig_lean_month.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

// Screenshots go in e2e/screenshots/
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (name) => path.join(SSDIR, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass    = (label) => { console.log(`PASS:   ${label}`); passed++; };
const fail    = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_ = (label) => { console.log(`ABSENT: ${label}`); absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

// ─── helpers ──────────────────────────────────────────────────────────────────

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const selectByText = async (page, ariaLabel, textMatch) => {
  return page.evaluate(({ label, match }) => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      if (sel.getAttribute("aria-label") === label) {
        const opt = Array.from(sel.options).find(o => o.text.toLowerCase().includes(match.toLowerCase()));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${sel.getAttribute("aria-label")}" → "${opt.text}"`;
        }
        return `label found but no option matching "${match}"; options: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return `select with aria-label="${label}" NOT found`;
  }, { label: ariaLabel, match: textMatch });
};

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
};

const getDataset = (page) => page.evaluate(() => {
  try { return JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"); } catch { return {}; }
});

const dismissModal = async (page) => {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  await page.waitForTimeout(200);
};

// Create a checking account
const createCheckingAccount = async (page, name, openingBalance) => {
  await navTo(page, "Accounts");
  await dismissModal(page);

  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add account|new account/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add Account button: ${addR}`);
  await page.waitForTimeout(800);

  // Name
  await page.evaluate((n) => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
    if (!inp) return "NOT FOUND";
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  // Type = Checking
  const typeR = await selectByText(page, "Account type", "Checking");
  note(`  Account type: ${typeR}`);

  // Opening balance
  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return "NOT FOUND";
    inp.value = b;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, String(openingBalance));

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add account$|^add$|^save$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// Create a credit card account
const createCreditCardAccount = async (page, name, openingBalance) => {
  await navTo(page, "Accounts");
  await dismissModal(page);

  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add account|new account/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add Account button: ${addR}`);
  await page.waitForTimeout(800);

  // Name
  await page.evaluate((n) => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
    if (!inp) return "NOT FOUND";
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  // Type = Credit card
  const typeR = await selectByText(page, "Account type", "Credit card");
  note(`  Account type: ${typeR}`);

  // Opening balance
  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return "NOT FOUND";
    inp.value = b;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, String(openingBalance));

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add account$|^add$|^save$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// Record an income transaction on a specific date
const recordIncome = async (page, label, amount, accountMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

  // Description
  await page.evaluate(({ desc }) => {
    const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
      i.getAttribute("aria-label") === "Description" ||
      i.getAttribute("placeholder") === "Description" ||
      i.getAttribute("aria-label") === "Payee" ||
      i.getAttribute("placeholder") === "Payee");
    if (inp) {
      inp.focus(); inp.value = desc;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, { desc: label });

  // Amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Type = Income
  await selectByText(page, "Type", "Income");

  // Account
  const acctR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "Account" || s.getAttribute("aria-label") === "From account");
    if (!sel) return "Account select NOT FOUND";
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, accountMatch);
  note(`  ${label} account: ${acctR}`);

  // Date — gig deposits happen on specific dates throughout June
  await page.evaluate((d) => {
    const inp = document.querySelector('input[type="date"]');
    if (inp) {
      inp.value = d;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, dateStr);

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// Record an expense transaction
const recordExpense = async (page, label, amount, accountMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

  // Description
  await page.evaluate(({ desc }) => {
    const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
      i.getAttribute("aria-label") === "Description" ||
      i.getAttribute("placeholder") === "Description" ||
      i.getAttribute("aria-label") === "Payee" ||
      i.getAttribute("placeholder") === "Payee");
    if (inp) {
      inp.focus(); inp.value = desc;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, { desc: label });

  // Amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Type = Expense
  await selectByText(page, "Type", "Expense");

  // Account
  const acctR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "Account" || s.getAttribute("aria-label") === "From account");
    if (!sel) return "Account select NOT FOUND";
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, accountMatch);
  note(`  ${label} account: ${acctR}`);

  // Date
  await page.evaluate((d) => {
    const inp = document.querySelector('input[type="date"]');
    if (inp) {
      inp.value = d;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, dateStr);

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// ─── main ─────────────────────────────────────────────────────────────────────

const jsErrors = [];

// Gig deposit schedule — irregular, spread across June 2026
const GIG_DEPOSITS = [
  { label: "L68 Devon Gig - Lyft payout",      amount: 180, date: "2026-06-03" },
  { label: "L68 Devon Gig - DoorDash payout",   amount: 95,  date: "2026-06-09" },
  { label: "L68 Devon Gig - Combined payout",   amount: 240, date: "2026-06-17" },
  { label: "L68 Devon Gig - Instacart payout",  amount: 130, date: "2026-06-24" },
];
const EXPECTED_GIG_TOTAL = 645; // 180 + 95 + 240 + 130
const EXPECTED_GIG_TOTAL_MINOR = 64500; // cents

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

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /accounts — Create Devon Checking ($120 opening) + Devon Credit Card ($0)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Create Devon Checking ($120) ────────────────────────────────────────────");

  await createCheckingAccount(page, "L68 Devon Checking", 120);

  await navTo(page, "Accounts");
  const checkingText = await page.evaluate(() => document.body.textContent);
  if (/L68 Devon Checking/i.test(checkingText)) {
    pass("Step 1.1 — L68 Devon Checking appears on /accounts");
  } else {
    fail("Step 1.1 — L68 Devon Checking NOT found on /accounts");
  }

  await page.screenshot({ path: SS("story68_01_accounts_checking_created.png") });
  pass("Step 1.2 — screenshot story68_01_accounts_checking_created.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: Create Devon Credit Card (liability, $0 opening for minimums)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Create Devon Credit Card (liability) ────────────────────────────────────");

  await createCreditCardAccount(page, "L68 Devon Credit Card", 0);

  await navTo(page, "Accounts");
  const acctPageText = await page.evaluate(() => document.body.textContent);
  if (/L68 Devon Credit Card/i.test(acctPageText)) {
    pass("Step 2.1 — L68 Devon Credit Card appears on /accounts");
  } else {
    fail("Step 2.1 — L68 Devon Credit Card NOT found on /accounts");
  }

  await page.screenshot({ path: SS("story68_02_accounts_both_created.png") });
  pass("Step 2.2 — screenshot story68_02_accounts_both_created.png (both accounts visible)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: /transactions — Seed 4 gig income deposits on different dates
  // KEY TEST C1: multi-deposit income — does each deposit land separately?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Seed 4 gig income deposits (C1 multi-deposit re-test) ───────────────────");

  for (const dep of GIG_DEPOSITS) {
    note(`  Recording ${dep.label} — $${dep.amount} on ${dep.date}`);
    await recordIncome(page, dep.label, dep.amount, "L68 Devon Checking", dep.date);
  }

  // Verify all 4 are in the dataset
  const dsAfterDeposits = await getDataset(page);
  const allTxns = Object.values(dsAfterDeposits.transactions || {});
  const l68Deposits = allTxns.filter(t => {
    const desc = (t.desc || t.description || t.payee || "");
    return /L68 Devon Gig/i.test(desc);
  });
  note(`L68 gig deposit transactions found in dataset: ${l68Deposits.length}`);

  if (l68Deposits.length === 4) {
    pass("Step 3.1 (C1) — All 4 gig deposits found in dataset (none dropped)");
  } else if (l68Deposits.length > 4) {
    fail(`Step 3.1 (C1) — ${l68Deposits.length} deposits in dataset — possible DUPLICATION (expected 4)`);
  } else {
    fail(`Step 3.1 (C1) — Only ${l68Deposits.length} deposits found in dataset (expected 4 — some may have been DROPPED)`);
  }

  // Sum the deposits from dataset
  const datasetSum = l68Deposits.reduce((acc, t) => {
    const raw = t.amount?.Amount ?? t.amount?.amount ?? Number(t.amount || 0);
    return acc + Math.abs(Number(raw));
  }, 0);
  note(`Dataset sum of L68 gig deposits: ${datasetSum} minor units (expected ${EXPECTED_GIG_TOTAL_MINOR})`);

  if (Math.abs(datasetSum - EXPECTED_GIG_TOTAL_MINOR) <= 1) {
    pass(`Step 3.2 (C1) INCOME_SUM — Dataset total = $${(datasetSum/100).toFixed(2)} (expected $${EXPECTED_GIG_TOTAL}.00) — exact match`);
  } else if (datasetSum === 0) {
    absent_("Step 3.2 (C1) INCOME_SUM — ABSENT: Dataset sum is 0 (key format unknown — amount field not resolved)");
  } else {
    fail(`Step 3.2 (C1) INCOME_SUM — Dataset total = ${datasetSum} minor units ($${(datasetSum/100).toFixed(2)}) — expected $${EXPECTED_GIG_TOTAL}.00 — MISMATCH`);
  }

  await navTo(page, "Transactions");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  const txnPageText = await page.evaluate(() => document.body.textContent);

  // Check all 4 deposits visible on screen
  const depositVisibility = GIG_DEPOSITS.map(d => ({
    label: d.label,
    visible: txnPageText.includes(d.label) || txnPageText.toLowerCase().includes(d.label.toLowerCase())
  }));
  const allVisible = depositVisibility.every(d => d.visible);
  const visCount = depositVisibility.filter(d => d.visible).length;
  note(`Deposits visible on /transactions screen: ${visCount}/4`);
  depositVisibility.forEach(d => note(`  "${d.label}": ${d.visible ? "VISIBLE" : "NOT VISIBLE"}`));

  if (allVisible) {
    pass("Step 3.3 (C1) — All 4 gig deposits visible on /transactions screen");
  } else {
    fail(`Step 3.3 (C1) — Only ${visCount}/4 deposits visible on /transactions screen — some may be filtered or paginated out`);
  }

  await page.screenshot({ path: SS("story68_03_transactions_4_deposits.png") });
  pass("Step 3.4 — screenshot story68_03_transactions_4_deposits.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Record CC minimum payment expense ($35) from checking
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Record CC minimum payment expense ($35) ─────────────────────────────────");

  await recordExpense(page, "L68 Devon CC Minimum Payment", 35, "L68 Devon Checking", "2026-06-22");

  await navTo(page, "Transactions");
  await dismissModal(page);
  const txnPageText2 = await page.evaluate(() => document.body.textContent);
  if (/L68 Devon CC Minimum Payment/i.test(txnPageText2)) {
    pass("Step 4.1 — CC minimum payment expense visible on /transactions");
  } else {
    fail("Step 4.1 — CC minimum payment expense NOT visible on /transactions");
  }

  await page.screenshot({ path: SS("story68_04_transactions_with_expense.png") });
  pass("Step 4.2 — screenshot story68_04_transactions_with_expense.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: /budgets — inspect income view and "left to allocate"
  // KEY TEST I2: piecemeal income budgeting — can Devon see his irregular income total?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: /budgets — income view + left to allocate (I2 PIECEMEAL_BUDGET) ─────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const budgetsText = await page.evaluate(() => document.body.textContent);

  // Does budgets show any income figure?
  const hasIncomeSection = /income|earned|received/i.test(budgetsText);
  note(`Budgets screen has income section: ${hasIncomeSection}`);

  // Look for $645 total anywhere
  const has645 = /645/i.test(budgetsText);
  const budgetAmounts = (budgetsText.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Budget screen amounts found: ${budgetAmounts.join(", ")}`);

  if (has645) {
    pass("Step 5.1 (I2) PIECEMEAL_BUDGET — $645 income total visible on /budgets (all 4 deposits summed)");
  } else {
    absent_("Step 5.1 (I2) PIECEMEAL_BUDGET — ABSENT: $645 NOT found on /budgets — budgets may not surface income from multiple piecemeal deposits, or no income budget category exists");
  }

  // Does budgets have "left to allocate" or similar affordance?
  const hasLeftToAllocate = /left to allocate|unallocated|available to budget|to assign/i.test(budgetsText);
  if (hasLeftToAllocate) {
    pass("Step 5.2 (I2) — Budgets shows 'left to allocate' / 'unallocated' affordance for irregular income");
  } else {
    absent_("Step 5.2 (I2) PIECEMEAL_BUDGET — ABSENT: No 'left to allocate' / 'unallocated income' affordance found — gig worker cannot see how much of their irregular income remains to budget");
  }

  // Does budgets show total income at all?
  if (hasIncomeSection) {
    pass("Step 5.3 (I2) — Budgets has an income section (not expenses-only)");
  } else {
    absent_("Step 5.3 (I2) PIECEMEAL_BUDGET — ABSENT: No income section on /budgets — budgets screen may be expenses-only, blocking a gig worker from seeing income vs. budget alignment");
  }

  await page.screenshot({ path: SS("story68_05_budgets_income_view.png") });
  pass("Step 5.4 — screenshot story68_05_budgets_income_view.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: /planning — forecast card + Thread B re-confirm
  // KEY TEST I3: Does forecast reflect irregular income? Does it ignore recurring?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /planning — forecast card + irregular income basis (I3 FORECAST_BASIS) ──");

  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const planText = await page.evaluate(() => document.body.textContent);

  // Is a forecast card present?
  const hasForecastCard = /net worth in 12 months|12.month|forecast|projected/i.test(planText);
  if (hasForecastCard) {
    pass("Step 6.1 — Forecast / 12-month net worth card present on /planning");
  } else {
    absent_("Step 6.1 (I3) — ABSENT: No forecast / net worth projection card on /planning");
  }

  // Is there a hint about what basis the forecast uses?
  const forecastHintMatch = planText.match(/if this month.s net cash flow \(([^)]+)\) continues/i) ||
                            planText.match(/monthly net[^.]*\$/i) ||
                            planText.match(/net cash flow[^.]*\$/i);
  if (forecastHintMatch) {
    note(`Forecast hint text found: "${forecastHintMatch[0]}"`);
    pass("Step 6.2 (I3) FORECAST_BASIS — Forecast hint text found (basis is readable)");

    // Thread B re-confirm: does the hint reference recurring items?
    const hintRefersRecurring = /recurring|scheduled|bills|fixed expenses/i.test(forecastHintMatch[0]);
    if (!hintRefersRecurring) {
      fail("Step 6.3 (I3) FORECAST_BASIS THREAD-B RECONFIRM — Forecast hint does NOT reference recurring/scheduled items — confirms Thread B: forecast still uses historical transactions only, ignores scheduled recurring (same gap as L54/L55)");
    } else {
      pass("Step 6.3 (I3) — Forecast hint references recurring/scheduled items (Thread B may be fixed)");
    }
  } else {
    // Try to read any numeric hint
    const planAmounts = (planText.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 20);
    note(`Planning screen amounts: ${planAmounts.join(", ")}`);
    absent_("Step 6.2 (I3) FORECAST_BASIS — ABSENT: No 'net cash flow' hint found on /planning — cannot determine what basis the forecast uses");
    note("Step 6.3 (I3) — Thread B re-confirm: forecast basis unknown from hint text alone");
  }

  // Does the forecast figure plausibly reflect the $645 gig income?
  // Given ~$89K sample data, Devon's $645 is submerged — but we can note
  const has645InPlan = /645/i.test(planText);
  note(`$645 income visible on /planning: ${has645InPlan}`);
  if (!has645InPlan) {
    note("Step 6.4 (I3) — $645 not isolated on /planning (submerged in sample data aggregate) — expected given seed data");
  }

  // Is there a cash runway card?
  const hasRunwayCard = /cash runway|runway/i.test(planText);
  if (hasRunwayCard) {
    pass("Step 6.5 — Cash runway card present on /planning");
  } else {
    absent_("Step 6.5 — ABSENT: No cash runway card on /planning");
  }

  await page.screenshot({ path: SS("story68_06_planning_forecast.png") });
  pass("Step 6.6 — screenshot story68_06_planning_forecast.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: /dashboard — income stat widget
  // KEY TEST C1 re-check: does dashboard income stat show $645?
  // KEY TEST I4: period consistency — same June 2026 period
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: /dashboard — income stat widget (C1 + I4) ─────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const dashText = await page.evaluate(() => document.body.textContent);

  // Check for income widget
  const hasIncomeWidget = /income|earnings|money in/i.test(dashText);
  if (hasIncomeWidget) {
    pass("Step 7.1 — Dashboard has income widget / income text");
  } else {
    absent_("Step 7.1 (C1) — ABSENT: No income widget on Dashboard");
  }

  // C1: Does dashboard show $645?
  const dashHas645 = /645/i.test(dashText);
  const dashAmounts = (dashText.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Dashboard amounts: ${dashAmounts.join(", ")}`);

  if (dashHas645) {
    pass("Step 7.2 (C1) INCOME_SUM — Dashboard income stat shows $645 (all 4 gig deposits summed)");
  } else {
    absent_("Step 7.2 (C1) INCOME_SUM — ABSENT: $645 NOT found on Dashboard — income stat may aggregate differently or Devon's $645 is submerged in sample data");
  }

  // I4: Period consistency — does dashboard show June 2026?
  const dashHasJune2026 = /june 2026|jun 2026/i.test(dashText);
  const dashHasCurrentPeriod = /this month|current month|june/i.test(dashText);
  if (dashHasJune2026 || dashHasCurrentPeriod) {
    pass("Step 7.3 (I4) PERIOD_CONSISTENCY — Dashboard shows current period (June 2026)");
  } else {
    note("Step 7.3 (I4) — No explicit period label found on Dashboard (may not display period text)");
    absent_("Step 7.3 (I4) PERIOD_CONSISTENCY — ABSENT: Cannot confirm Dashboard period from text alone");
  }

  await page.screenshot({ path: SS("story68_07_dashboard_income.png") });
  pass("Step 7.4 — screenshot story68_07_dashboard_income.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: /reports — income total
  // KEY TEST C1 re-check: does reports show $645 income for the period?
  // KEY TEST I4: period consistency — reports period matches other screens
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: /reports — income total (C1 + I4) ─────────────────────────────────────");

  await navTo(page, "Reports");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const reportsText = await page.evaluate(() => document.body.textContent);
  const reportAmounts = (reportsText.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Reports amounts: ${reportAmounts.join(", ")}`);

  // C1: Does reports show $645?
  const reportsHas645 = /645/i.test(reportsText);
  if (reportsHas645) {
    pass("Step 8.1 (C1) INCOME_SUM — Reports shows $645 income (all 4 gig deposits summed)");
  } else {
    absent_("Step 8.1 (C1) INCOME_SUM — ABSENT: $645 NOT found on /reports — income total may be aggregated with sample data or period not aligned to June 2026");
  }

  // I4: Period label on reports
  const reportsHasJune = /june 2026|jun 2026|this month/i.test(reportsText);
  if (reportsHasJune) {
    pass("Step 8.2 (I4) PERIOD_CONSISTENCY — /reports shows June 2026 period");
  } else {
    note("Step 8.2 (I4) — /reports period not explicit from text (may use a date picker)");
    absent_("Step 8.2 (I4) PERIOD_CONSISTENCY — ABSENT: Cannot confirm /reports period matches Dashboard/Budgets/Planning period");
  }

  // Does reports have income breakdown vs expense breakdown?
  const hasIncomeInReports = /income/i.test(reportsText);
  if (hasIncomeInReports) {
    pass("Step 8.3 — /reports has income section (gig worker can see their income total)");
  } else {
    absent_("Step 8.3 — ABSENT: /reports has no income section — gig worker cannot see their income total from reports");
  }

  await page.screenshot({ path: SS("story68_08_reports_income.png") });
  pass("Step 8.4 — screenshot story68_08_reports_income.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: Dataset final audit — C1 INCOME_SUM across all L68 transactions
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: Final dataset audit (C1 full verification) ─────────────────────────────");

  const dsFinal = await getDataset(page);
  const allFinalTxns = Object.values(dsFinal.transactions || {});
  const l68All = allFinalTxns.filter(t => {
    const desc = (t.desc || t.description || t.payee || "");
    return /L68/i.test(desc);
  });
  note(`All L68 transactions in dataset: ${l68All.length}`);

  const l68Incomes = l68All.filter(t => {
    const desc = (t.desc || t.description || t.payee || "");
    return /L68 Devon Gig/i.test(desc);
  });
  const l68Expenses = l68All.filter(t => {
    const desc = (t.desc || t.description || t.payee || "");
    return /L68 Devon CC Minimum Payment/i.test(desc);
  });
  note(`L68 income transactions: ${l68Incomes.length} (expected 4)`);
  note(`L68 expense transactions: ${l68Expenses.length} (expected 1)`);

  // Re-sum incomes with Amount (capital) awareness
  const incomeSum = l68Incomes.reduce((acc, t) => {
    const raw = t.amount?.Amount ?? t.amount?.amount ?? Number(t.amount || 0);
    return acc + Math.abs(Number(raw));
  }, 0);
  note(`Final income sum from dataset: ${incomeSum} minor units (expected ${EXPECTED_GIG_TOTAL_MINOR})`);

  if (l68Incomes.length === 4 && Math.abs(incomeSum - EXPECTED_GIG_TOTAL_MINOR) <= 1) {
    pass(`Step 9.1 (C1) INCOME_SUM FINAL — 4 deposits, sum $${(incomeSum/100).toFixed(2)} — EXACT MATCH with $${EXPECTED_GIG_TOTAL}.00 expected`);
  } else if (l68Incomes.length === 4 && incomeSum === 0) {
    // Amount key format issue — count correct but sum can't be verified
    pass("Step 9.1 (C1) — 4 deposits present in dataset (count correct); sum unverifiable due to amount key format");
    absent_("Step 9.1a (C1) INCOME_SUM — ABSENT: Amount field format not resolved — could not verify $645 sum from dataset");
  } else if (l68Incomes.length !== 4) {
    fail(`Step 9.1 (C1) INCOME_SUM — ${l68Incomes.length} income transactions in dataset (expected 4)`);
  } else {
    fail(`Step 9.1 (C1) INCOME_SUM — Sum ${incomeSum} minor units ($${(incomeSum/100).toFixed(2)}) ≠ expected $${EXPECTED_GIG_TOTAL}.00`);
  }

  if (l68Expenses.length === 1) {
    pass("Step 9.2 — CC minimum payment expense (1 expected) present in dataset");
  } else {
    note(`Step 9.2 — CC minimum payment count: ${l68Expenses.length} (expected 1)`);
  }

  // Final screenshot at /transactions for the full picture
  await navTo(page, "Transactions");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("story68_09_transactions_final.png") });
  pass("Step 9.3 — screenshot story68_09_transactions_final.png (final ledger view)");

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n════════════════════════════════════════════════════════════════════════════════════");
  console.log(`SUMMARY: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
  console.log(`Real JS errors: ${jsErrors.length}`);
  if (jsErrors.length > 0) jsErrors.forEach(e => console.error(`  ERROR: ${e}`));

  console.log(`\nGIG INCOME VERDICT:`);
  console.log(`  Deposit count in dataset: ${l68Incomes.length} (expected 4)`);
  console.log(`  Sum from dataset: ${incomeSum} minor units ($${(incomeSum/100).toFixed(2)}) — expected $${EXPECTED_GIG_TOTAL}.00`);
  if (l68Incomes.length === 4 && Math.abs(incomeSum - EXPECTED_GIG_TOTAL_MINOR) <= 1) {
    console.log(`  C1 VERDICT: HELD — 4 deposits, exact $645.00 total, no drop or doubling`);
  } else if (l68Incomes.length === 4 && incomeSum === 0) {
    console.log(`  C1 VERDICT: PARTIAL — 4 deposits present, sum unverifiable (amount key format)`);
  } else {
    console.log(`  C1 VERDICT: VIOLATED — deposit count ${l68Incomes.length}, sum ${incomeSum}`);
  }

  console.log("════════════════════════════════════════════════════════════════════════════════════");

} finally {
  await browser.close();
}

const exitCode = failed > 0 ? 1 : 0;
process.exit(exitCode);
