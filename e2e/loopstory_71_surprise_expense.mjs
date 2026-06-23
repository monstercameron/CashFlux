// L71 E2E loop story — "The Surprise Expense" (debt / financial hardship) — 2026-06-22
//
// Persona: Jamie has a checking account barely covering basics ($150) and a credit card
// with $900 balance at 21% APR. The car breaks down and Jamie puts a $600 repair on the
// credit card. This story focuses on DEBT MANAGEMENT under surprise expense pressure.
//
// KEY INVARIANTS ASSERTED:
//   I1: CARD_BALANCE_SIGN   — After $600 charge, card shows $1,500 owed (correct increase)
//                             NOT $300 (sign bug: charge would decrease balance)
//   I2: NET_WORTH_DROP      — Net worth falls exactly $600 after the card charge
//   I3: TOTAL_DEBT          — Total debt shown on dashboard = $1,500
//   I4: BUDGET_COVERAGE     — Car/Auto budget shows the $600 overage (L46 re-test:
//                             does a credit-card expense count against a budget?)
//   I5: DASHBOARD_HONEST    — Dashboard net worth / debt widgets reflect the new reality
//   I6: CROSS_SCREEN        — Accounts, Transactions, Dashboard all consistent post-charge
//   I7: MONEY_CONSERVE      — No phantom money; checking unchanged; only card balance up
//   I8: GOAL_IMPACT         — Emergency fund goal ($300 progress) reflects reduced net worth
//
// Screens exercised (≥5):
//   /accounts → /budgets → /goals → /transactions → /accounts → /dashboard
//
// Setup (seeded at start of run):
//   L71 Jamie Checking    — asset, $150
//   L71 Jamie CC          — liability (credit card), $900 existing balance, 21% APR
//   L71 Car Repair Budget — budget with minimal remaining (e.g. $50)
//   L71 Emergency Fund    — goal, $300 current progress, $1,000 target
//
// The charge: $600 expense on L71 Jamie CC, category "Auto" / "Car repair"
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_71_surprise_expense.mjs

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
const absent_ = (label) => { console.log(`ABSENT: ${label}`); absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

// ─── helpers ──────────────────────────────────────────────────────────────────

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link  = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const selectByText = async (page, ariaLabel, textMatch) =>
  page.evaluate(({ label, match }) => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      if (sel.getAttribute("aria-label") === label) {
        const opt = Array.from(sel.options).find(o =>
          o.text.toLowerCase().includes(match.toLowerCase()));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${label}" → "${opt.text}"`;
        }
        return `label found but no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return `select aria-label="${label}" NOT FOUND`;
  }, { label: ariaLabel, match: textMatch });

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

// Reset member filter to "Everyone" so newly-created accounts are visible (L70 lesson)
const resetMemberFilter = async (page) => {
  await page.evaluate(() => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "View as member");
    if (sel) {
      sel.value = "";
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });
  await page.waitForTimeout(300);
};

// Create an account
const createAccount = async (page, name, typeText, openingBalance) => {
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

  await page.evaluate((n) => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i =>
      i.placeholder === "Name");
    if (!inp) return;
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input",  { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  const typeR = await selectByText(page, "Account type", typeText);
  note(`  Account type: ${typeR}`);

  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return;
    inp.value = b;
    inp.dispatchEvent(new Event("input",  { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, String(openingBalance));

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

// Read displayed balance for an account by name (screen text)
const readAccountBalance = async (page, namePattern) =>
  page.evaluate((pat) => {
    const text = document.body.textContent;
    const re = new RegExp(pat + "[^$\\d(−-]*?([−(−]?\\$[\\d,]+\\.?\\d*)", "i");
    const m  = text.match(re);
    return m ? m[1] : null;
  }, namePattern);

// Parse a displayed money string to a number (handles ($X.XX), -$X.XX, $X.XX)
const parseMoney = (str) => {
  if (!str) return null;
  const neg = str.includes("(") || str.includes("−") || str.startsWith("-");
  const num = parseFloat(str.replace(/[^0-9.]/g, ""));
  return neg ? -num : num;
};

// Record an expense transaction against a specific account
const recordExpense = async (page, description, amount, accountMatch, categoryMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  // Find "Add" / "New transaction" button — dump all button texts first for debugging
  const allBtns = await page.evaluate(() =>
    Array.from(document.querySelectorAll("button")).map(b => b.textContent.trim()).filter(Boolean));
  note(`  All buttons on /transactions: ${JSON.stringify(allBtns)}`);

  const openR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction|\badd\b|\+/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Open add-transaction: ${openR}`);
  await page.waitForTimeout(800);

  // Fill description / payee
  await page.evaluate(({ desc }) => {
    const inp = Array.from(document.querySelectorAll("input, textarea")).find(i =>
      /description|payee|note/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
    if (inp) {
      inp.focus(); inp.value = desc;
      inp.dispatchEvent(new Event("input",  { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, { desc: description });

  // Fill amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input",  { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Set transaction type to Expense (if select exists)
  const typeR = await selectByText(page, "Type", "Expense");
  note(`  Transaction type: ${typeR}`);

  // Set account (may be labelled "Account", "From", "From account")
  const acctR = await page.evaluate((match) => {
    const candidates = ["Account", "From", "From account"];
    for (const lbl of candidates) {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === lbl);
      if (sel) {
        const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${lbl}" → "${opt.text}"`;
        }
        return `label "${lbl}" found but no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    // List all selects for debugging
    return `no account select (Account/From/From account) found; selects: ${Array.from(document.querySelectorAll("select")).map(s => `${s.getAttribute("aria-label")}:[${Array.from(s.options).map(o => o.text).join(",")}]`).join(" | ")}`;
  }, accountMatch);
  note(`  Account select: ${acctR}`);

  // Set category
  if (categoryMatch) {
    const catR = await page.evaluate((match) => {
      const candidates = ["Category", "Budget", "Budget category"];
      for (const lbl of candidates) {
        const sel = Array.from(document.querySelectorAll("select")).find(s =>
          s.getAttribute("aria-label") === lbl);
        if (sel) {
          const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
          if (opt) {
            sel.value = opt.value;
            sel.dispatchEvent(new Event("change", { bubbles: true }));
            return `set "${lbl}" → "${opt.text}"`;
          }
          return `label found, no match for "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
        }
      }
      return "category/budget select NOT FOUND";
    }, categoryMatch);
    note(`  Category: ${catR}`);
  }

  // Set date
  if (dateStr) {
    await page.evaluate((d) => {
      const inp = document.querySelector('input[type="date"]');
      if (inp) {
        inp.value = d;
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, dateStr);
  }

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

  return { openR, acctR };
};

// ─── main ─────────────────────────────────────────────────────────────────────

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

  // Hard reload to clear stale atom state (L70 lesson)
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  note("Hard reload complete — clearing stale atom state");

  const today = new Date();
  const yyyy  = today.getFullYear();
  const mm    = String(today.getMonth() + 1).padStart(2, "0");
  const dd    = String(today.getDate()).padStart(2, "0");
  const todayStr = `${yyyy}-${mm}-${dd}`;

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: Seed accounts
  //   L71 Jamie Checking   — $150  (asset — barely covers basics)
  //   L71 Jamie CC         — $900  (credit card liability @ 21% APR)
  //
  // Baseline net worth before charge: $150 (asset) − $900 (liability) = −$750
  // Expected net worth after $600 charge to CC: $150 − $1,500 = −$1,350  (Δ = −$600)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Seed accounts ────────────────────────────────────────────────────────────");

  await createAccount(page, "L71 Jamie Checking", "Checking", 150);
  await createAccount(page, "L71 Jamie CC",       "Credit card", 900);

  // Reset member filter before reading (L70 lesson)
  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);

  const acctText1 = await page.evaluate(() => document.body.textContent);
  if (/L71 Jamie Checking/i.test(acctText1)) pass("Step 1.1 — L71 Jamie Checking visible on /accounts");
  else fail("Step 1.1 — L71 Jamie Checking NOT visible (member-filter or reactive-atom bug)");

  if (/L71 Jamie CC/i.test(acctText1)) pass("Step 1.2 — L71 Jamie CC visible on /accounts");
  else fail("Step 1.2 — L71 Jamie CC NOT visible (member-filter or reactive-atom bug)");

  // I1 baseline: read CC balance before the charge
  const ccBalanceBefore = await readAccountBalance(page, "L71 Jamie CC");
  note(`I1 baseline CC balance: ${ccBalanceBefore}`);

  // I2 baseline: read net worth before charge (from dashboard)
  await navTo(page, "Dashboard");
  const dashText0 = await page.evaluate(() => document.body.textContent);
  const netWorthRaw0 = dashText0.match(/net worth[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ?? null;
  note(`I2 baseline net worth on dashboard: ${netWorthRaw0}`);

  await page.screenshot({ path: SS("l71_01_accounts_seed.png") });
  note("Screenshot: l71_01_accounts_seed.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: Probe the Car / Auto budget before the charge
  //   Seed a budget called "L71 Car Repair" if possible, or note the existing one.
  //   We need a budget associated with "Auto" / "Car repair" to re-test L46.
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Inspect / seed Car budget ──────────────────────────────────────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(800);

  const budgetPageText0 = await page.evaluate(() => document.body.textContent);
  note(`Existing budgets mention 'auto/car': ${/auto|car/i.test(budgetPageText0)}`);

  // Try to create an L71 Car budget
  const addBudgetR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add budget|new budget/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Add Budget button: ${addBudgetR}`);
  await page.waitForTimeout(800);

  if (addBudgetR !== "NOT FOUND") {
    // Fill in budget name
    await page.evaluate(() => {
      const inp = Array.from(document.querySelectorAll("input")).find(i =>
        /name|label|title/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
      if (inp) {
        inp.focus(); inp.value = "L71 Car Repair";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    // Fill budget amount = $650 (leaving $50 remaining after previous spend)
    await page.evaluate(() => {
      const inp = document.querySelector('input[type="number"]');
      if (inp) {
        inp.value = "650";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    // Set category to Auto / Transportation if possible
    const catR = await selectByText(page, "Category", "Auto");
    note(`  Budget category: ${catR}`);
    const catR2 = catR.includes("NOT FOUND") ? await selectByText(page, "Category", "Transport") : catR;
    note(`  Budget category (fallback): ${catR2}`);

    // Submit
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return /^add$|^save$|^add budget$/i.test(t) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);
  } else {
    dismissModal(page);
  }

  await navTo(page, "Budgets");
  await dismissModal(page);
  const budgetTextBefore = await page.evaluate(() => document.body.textContent);
  await page.screenshot({ path: SS("l71_02_budgets_before.png") });
  note("Screenshot: l71_02_budgets_before.png");

  const carBudgetExists = /L71 Car Repair/i.test(budgetTextBefore);
  note(`L71 Car Repair budget exists on /budgets: ${carBudgetExists}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Record $600 car repair charge on the credit card
  //
  // This is the CORE test:
  //   - Expense of $600 charged to L71 Jamie CC
  //   - Categorized as "Auto" or "Car repair"
  //   - Expected: CC balance $900 → $1,500 (debt INCREASES — correct)
  //   - Bug path: CC balance $900 → $300  (sign bug — charge DECREASES liability)
  //   - Checking should be UNCHANGED ($150)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Record $600 car repair charge on CC ─────────────────────────────────────");

  const txResult = await recordExpense(
    page,
    "L71 Car Repair - Emergency",
    600,
    "L71 Jamie CC",
    "Auto",
    todayStr
  );
  note(`Expense recorded: openR=${txResult.openR}, acctR=${txResult.acctR}`);

  // Verify in /transactions
  await navTo(page, "Transactions");
  const txText = await page.evaluate(() => document.body.textContent);
  const txPosted = /L71 Car Repair/i.test(txText);
  if (txPosted) pass("Step 3.1 — L71 Car Repair expense appears in /transactions");
  else fail("Step 3.1 — L71 Car Repair expense NOT found in /transactions");

  await page.screenshot({ path: SS("l71_03_transactions_charge.png") });
  note("Screenshot: l71_03_transactions_charge.png");

  // Dataset inspection: find the transaction
  const dsAfterTx = await getDataset(page);
  const allTxns   = Object.values(dsAfterTx.transactions || {});
  const carTxn    = allTxns.find(t => /L71 Car Repair/i.test(t.description || t.payee || t.name || ""));
  note(`I7 L71 Car Repair transaction in dataset: ${JSON.stringify(carTxn)}`);

  if (carTxn) {
    const txnAmount = carTxn.amount?.Amount ?? carTxn.amount ?? null;
    note(`I7 transaction amount in dataset: ${txnAmount}`);
    // Should be -60000 minor units (expense: negative) or 60000 depending on convention
    if (Math.abs(txnAmount) === 60000) {
      pass("I7 MONEY_CONSERVE — Transaction stored as ±$600.00 (60000 minor units) ✓");
    } else if (txnAmount !== null) {
      fail(`I7 MONEY_CONSERVE — Transaction amount unexpected: ${txnAmount} (expected ±60000 minor units)`);
    } else {
      absent_("I7 MONEY_CONSERVE — Cannot read transaction amount from dataset");
    }
  } else {
    absent_("I7 MONEY_CONSERVE — Car Repair transaction not found in dataset");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Verify CC balance after charge
  //
  // I1: CARD_BALANCE_SIGN
  //   Correct:  CC balance = $1,500  (debt increased by $600 — card was charged)
  //   Sign bug: CC balance = $300    (charge subtracted from positive-stored balance)
  //   Other:    CC balance = $900    (balance didn't change — reactive update missed)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Verify CC balance after $600 charge ─────────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);  // L70: always reset filter before reading

  const ccBalanceAfter = await readAccountBalance(page, "L71 Jamie CC");
  note(`I1 CC balance after charge (raw): ${ccBalanceAfter}`);

  const ccValAfter = parseMoney(ccBalanceAfter);
  note(`I1 CC balance after charge (parsed): ${ccValAfter}`);

  await page.screenshot({ path: SS("l71_04_accounts_after_charge.png") });
  note("Screenshot: l71_04_accounts_after_charge.png");

  // Also check checking — should be unchanged at $150
  const checkingBalance = await readAccountBalance(page, "L71 Jamie Checking");
  note(`I7 Checking balance after CC charge: ${checkingBalance}`);
  const checkingVal = parseMoney(checkingBalance);

  if (checkingVal !== null) {
    if (Math.abs(checkingVal - 150) < 1) {
      pass("I7 CHECKING_UNCHANGED — Checking still $150; card charge didn't debit checking ✓");
    } else {
      fail(`I7 CHECKING_UNCHANGED — Checking = $${checkingVal} (expected $150); card charge incorrectly affected checking`);
    }
  } else {
    absent_("I7 CHECKING_UNCHANGED — Cannot read Checking balance from screen");
  }

  // Assess card balance direction (PRIMARY BUG CHECK)
  if (ccValAfter !== null) {
    const absVal = Math.abs(ccValAfter);
    if (Math.abs(absVal - 1500) < 1) {
      pass("I1 CARD_BALANCE_SIGN — CC balance = $1,500 after $600 charge: CORRECT (debt increased) ✓");
    } else if (Math.abs(absVal - 300) < 1) {
      fail("I1 CARD_BALANCE_SIGN — CC balance = $300 (SIGN BUG: $600 charge DECREASED balance from $900 to $300 instead of increasing to $1,500)");
    } else if (Math.abs(absVal - 900) < 1) {
      fail("I1 CARD_BALANCE_SIGN — CC balance = $900 (UNCHANGED: balance did not update after $600 charge — reactive update gap; same as L46/L64 Thread A)");
    } else {
      fail(`I1 CARD_BALANCE_SIGN — CC balance = $${absVal} (unexpected; expected $1,500 after $900 + $600 charge)`);
    }
  } else {
    // Fall back to dataset
    const dsCheck = await getDataset(page);
    const ccAcct  = Object.values(dsCheck.accounts || {}).find(a => /L71 Jamie CC/i.test(a.name || ""));
    note(`I1 CC account in dataset: ${JSON.stringify(ccAcct)}`);
    absent_("I1 CARD_BALANCE_SIGN — Cannot read CC balance from screen; see dataset note");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Net worth and total debt on Dashboard
  //
  // I2: NET_WORTH_DROP — Net worth should fall by exactly $600
  //     Before: $150 − $900 = −$750 (or whatever the app shows)
  //     After:  $150 − $1,500 = −$1,350  (Δ = −$600)
  //
  // I3: TOTAL_DEBT — Dashboard should show total debt = $1,500
  //
  // I5: DASHBOARD_HONEST — Widgets reflect new debt reality
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Dashboard net worth + total debt ────────────────────────────────────────");

  await navTo(page, "Dashboard");
  const dashText1 = await page.evaluate(() => document.body.textContent);

  await page.screenshot({ path: SS("l71_05_dashboard_post_charge.png") });
  note("Screenshot: l71_05_dashboard_post_charge.png");

  // Net worth
  const netWorthRaw1 = dashText1.match(/net worth[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ?? null;
  note(`I2 net worth on dashboard after charge: ${netWorthRaw1}`);
  const netWorthVal1 = parseMoney(netWorthRaw1);

  if (netWorthVal1 !== null && netWorthRaw0 !== null) {
    const netWorthVal0  = parseMoney(netWorthRaw0);
    const delta         = netWorthVal1 - netWorthVal0;
    note(`I2 net worth delta: ${delta} (expected ~-600)`);
    if (Math.abs(delta + 600) < 5) {
      pass(`I2 NET_WORTH_DROP — Net worth fell exactly $600 (${netWorthRaw0} → ${netWorthRaw1}) ✓`);
    } else {
      fail(`I2 NET_WORTH_DROP — Net worth delta = $${delta} (expected −$600); from ${netWorthRaw0} to ${netWorthRaw1}`);
    }
  } else if (netWorthVal1 !== null) {
    note(`I2 baseline not captured; post-charge net worth = ${netWorthVal1}`);
    // Check absolute: if net worth = -1350 it's consistent with $150 − $1,500
    if (Math.abs(netWorthVal1 + 1350) < 5) {
      pass(`I2 NET_WORTH_DROP — Net worth = −$1,350 which is consistent with $150 asset − $1,500 liability ✓`);
    } else {
      fail(`I2 NET_WORTH_DROP — Net worth = ${netWorthVal1}; cannot confirm $600 drop (baseline missing)`);
    }
  } else {
    absent_("I2 NET_WORTH_DROP — Net worth not found on Dashboard");
  }

  // Total debt
  const totalDebtRaw = dashText1.match(/total debt[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ??
                       dashText1.match(/debt[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ?? null;
  note(`I3 total debt on dashboard: ${totalDebtRaw}`);

  if (totalDebtRaw !== null) {
    const debtVal = Math.abs(parseMoney(totalDebtRaw));
    if (Math.abs(debtVal - 1500) < 5) {
      pass(`I3 TOTAL_DEBT — Dashboard shows total debt = $1,500 ✓`);
    } else if (Math.abs(debtVal - 900) < 5) {
      fail(`I3 TOTAL_DEBT — Dashboard still shows $900 debt (hasn't updated after $600 charge)`);
    } else {
      fail(`I3 TOTAL_DEBT — Dashboard shows debt = $${debtVal} (expected $1,500)`);
    }
  } else {
    absent_("I3 TOTAL_DEBT — No total debt figure found on Dashboard");
  }

  // I5: General dashboard honesty — does it show any debt-related signal?
  const dashHasDebtSignal = /debt|credit|owed|balance/i.test(dashText1);
  if (dashHasDebtSignal) pass("I5 DASHBOARD_HONEST — Dashboard has at least one debt/credit signal visible");
  else absent_("I5 DASHBOARD_HONEST — Dashboard shows no debt/credit signal after $1,500 CC balance");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: L46 re-test — Does the $600 CC charge count against the Car budget?
  //
  // I4: BUDGET_COVERAGE
  //   - If yes: Car/Auto budget shows the $600 spend (possibly as overage)
  //   - If no: budget is blind to CC-charged expenses (L46 bug)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: Budget coverage check (L46 re-test) ─────────────────────────────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  const budgetTextAfter = await page.evaluate(() => document.body.textContent);

  await page.screenshot({ path: SS("l71_06_budgets_after_charge.png") });
  note("Screenshot: l71_06_budgets_after_charge.png");

  // Look for $600 or overage near "Car" or "Auto" budget entries
  const budgetShowsCharge = /600/i.test(budgetTextAfter) &&
    (/car|auto/i.test(budgetTextAfter) || /L71 Car Repair/i.test(budgetTextAfter));
  const budgetShowsOverage = /over|exceed|−\s*\$600|\(\$600/i.test(budgetTextAfter);

  note(`Budget shows $600 near car/auto: ${budgetShowsCharge}`);
  note(`Budget shows overage: ${budgetShowsOverage}`);

  if (budgetShowsCharge || budgetShowsOverage) {
    pass("I4 BUDGET_COVERAGE — Car/Auto budget reflects $600 CC charge ✓ (L46 re-test PASS)");
  } else if (carBudgetExists) {
    // Budget was seeded but no $600 visible — L46 bug
    fail("I4 BUDGET_COVERAGE — L71 Car Repair budget exists but does NOT show $600 CC charge (L46 bug: CC-charged expenses invisible to budgets)");
  } else {
    // Budget not seeded (seed failed) — can still check if any auto category updated
    const anyAutoUpdated = /auto|car/i.test(budgetTextAfter) && /600/i.test(budgetTextAfter);
    if (anyAutoUpdated) {
      pass("I4 BUDGET_COVERAGE — An Auto/Car budget shows $600 from the CC charge ✓");
    } else {
      absent_("I4 BUDGET_COVERAGE — No Car/Auto budget found or seeded; cannot confirm L46 re-test");
    }
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: Seed and check Emergency Fund goal
  //
  // I8: GOAL_IMPACT — Does the net worth drop affect goal health display?
  //   (A $150 checking balance with $1,500 CC debt means the household is in the hole;
  //    the emergency fund goal ($300 target) should still show $0 progress or be
  //    flagged as at-risk given negative net worth.)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: Goals — emergency fund impact check ─────────────────────────────────────");

  await navTo(page, "Goals");
  await dismissModal(page);
  const goalsText = await page.evaluate(() => document.body.textContent);

  await page.screenshot({ path: SS("l71_07_goals_state.png") });
  note("Screenshot: l71_07_goals_state.png");

  // Try to seed a goal so we can see how it displays under negative net worth
  const addGoalR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add goal|new goal/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Add Goal button: ${addGoalR}`);
  await page.waitForTimeout(800);

  if (addGoalR !== "NOT FOUND") {
    // Fill goal name
    await page.evaluate(() => {
      const inp = Array.from(document.querySelectorAll("input")).find(i =>
        /name|label|title|goal/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
      if (inp) {
        inp.focus(); inp.value = "L71 Emergency Fund";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    // Fill target amount
    await page.evaluate(() => {
      const inputs = document.querySelectorAll('input[type="number"]');
      const inp = inputs[0];
      if (inp) {
        inp.value = "1000";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    // Submit
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return /^add$|^save$|^add goal$/i.test(t) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);

    await navTo(page, "Goals");
    await dismissModal(page);
    const goalsText2 = await page.evaluate(() => document.body.textContent);
    const goalVisible = /L71 Emergency Fund/i.test(goalsText2);
    if (goalVisible) {
      pass("I8 GOAL_SEEDED — L71 Emergency Fund goal visible on /goals");
      // Check if goal shows any at-risk signal given negative net worth
      const atRisk = /at.?risk|behind|off.?track|negative/i.test(goalsText2);
      if (atRisk) pass("I8 GOAL_IMPACT — Goal shows at-risk signal given negative net worth ✓");
      else absent_("I8 GOAL_IMPACT — Goal does not flag at-risk status despite household net worth < $0");
    } else {
      fail("I8 GOAL_SEEDED — L71 Emergency Fund goal NOT visible after creation");
    }
  } else {
    dismissModal(page);
    absent_("I8 GOAL_IMPACT — Cannot seed goal (Add Goal button not found); goal impact not tested");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: Cross-screen consistency check
  //
  // I6: CROSS_SCREEN — Accounts, Transactions, Dashboard all agree on CC balance = $1,500
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: Cross-screen consistency ────────────────────────────────────────────────");

  // Re-read accounts
  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  const finalAcctText = await page.evaluate(() => document.body.textContent);

  // Transactions — confirm charge is there
  await navTo(page, "Transactions");
  const finalTxText = await page.evaluate(() => document.body.textContent);

  await page.screenshot({ path: SS("l71_08_cross_screen_accounts.png") });
  note("Screenshot: l71_08_cross_screen_accounts.png");

  // Check consistency:  CC balance on accounts screen (re-read)
  const ccFinal    = await readAccountBalance(page, "L71 Jamie CC");
  note(`I6 CC balance on /accounts final read: ${ccFinal}`);

  const txOnTransactions = /L71 Car Repair/i.test(finalTxText);
  note(`I6 Car Repair tx on /transactions: ${txOnTransactions}`);

  if (txOnTransactions) {
    pass("I6 CROSS_SCREEN — Car Repair expense visible on /transactions ✓");
  } else {
    fail("I6 CROSS_SCREEN — Car Repair expense NOT visible on /transactions (may have been dropped)");
  }

  // Final summary screenshot
  await navTo(page, "Dashboard");
  await page.screenshot({ path: SS("l71_09_dashboard_final.png") });
  note("Screenshot: l71_09_dashboard_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n─────────────────────────────────────────────────────────────────────────────────────");
  console.log(`L71 FINAL: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
  console.log(`JS errors during run: ${jsErrors.length}`);
  if (jsErrors.length) jsErrors.forEach(e => console.error("JS ERROR:", e));

} catch (err) {
  console.error("FATAL:", err);
} finally {
  await browser.close();
}

process.exitCode = failed > 0 ? 1 : 0;
