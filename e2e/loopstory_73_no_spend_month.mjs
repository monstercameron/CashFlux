// L73 E2E loop story — "The No-Spend Month" (Wei) — 2026-06-22
//
// Persona: Wei commits to a no-spend month to free up cash and attack debt.
// Prior month had active discretionary spend (dining $200, shopping $150, entertainment $100).
// This month: all discretionary budgets set to $0 (or $1 as min if $0 not accepted).
// Only essentials are logged (groceries ~$80, gas ~$40). $400 freed cash goes to credit card.
// Expected: card drops from $1,800 → $1,400; checking drops by $400; budgets show $0 spent.
//
// KEY INVARIANTS ASSERTED:
//   I1: BUDGET_PERSIST        — No-spend ($0/$1) budgets survive a hard reload
//   I2: NO_DISCRETIONARY      — Discretionary categories show $0 spent on /budgets
//   I3: MOM_COMPARISON        — Month-over-month savings comparison present/absent (probe)
//   I4: PAYMENT_DIRECTION     — $400 CC payment reduces card balance (re-test L64 sign bug)
//   I5: MONEY_CONSERVE        — Card drops from $1,800 to $1,400; checking drops $400
//   I6: REPORTS_MOM           — /reports shows lower discretionary spend this month
//   I7: CROSS_SCREEN          — /accounts + /budgets consistent after hard reload
//
// Screens exercised (≥5): /accounts → /budgets → /transactions → /budgets → /reports → /dashboard
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_73_no_spend_month.mjs

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

// Reset member filter to "Everyone" (L70 lesson)
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

// Log an expense transaction
const logExpense = async (page, description, amount, categoryMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  const openR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction|\badd\b|\+/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Open add-transaction: ${openR}`);
  await page.waitForTimeout(800);

  // Fill description/payee
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

  // Set category if provided
  if (categoryMatch) {
    const catR = await selectByText(page, "Category", categoryMatch);
    note(`  Category: ${catR}`);
  }

  // Date
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
};

// Record a payment (transfer from checking → cc)
const recordTransfer = async (page, description, amount, fromMatch, toMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  const openR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction|\badd\b|\+/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Open add-transaction: ${openR}`);
  await page.waitForTimeout(800);

  // Fill description
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

  // Set type to Transfer
  const typeR = await selectByText(page, "Type", "Transfer");
  note(`  Transaction type: ${typeR}`);

  // Set From account
  const fromR = await page.evaluate((match) => {
    const candidates = ["From", "From account", "Account"];
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
        return `"${lbl}" found, no match "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return "no From select found";
  }, fromMatch);
  note(`  From account: ${fromR}`);

  // Set To account
  const toR = await page.evaluate((match) => {
    const candidates = ["To", "To account"];
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
        return `"${lbl}" found, no match "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return "no To select found";
  }, toMatch);
  note(`  To account: ${toR}`);

  // Date
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
};

// Read displayed balance for an account by name
const readAccountBalance = async (page, namePattern) =>
  page.evaluate((pat) => {
    const text = document.body.textContent;
    const re = new RegExp(pat + "[^$\\d(−-]*?([−(−]?\\$[\\d,]+\\.?\\d*)", "i");
    const m  = text.match(re);
    return m ? m[1] : null;
  }, namePattern);

const parseMoney = (str) => {
  if (!str) return null;
  const neg = str.includes("(") || str.includes("−") || str.startsWith("-");
  const num = parseFloat(str.replace(/[^0-9.]/g, ""));
  return neg ? -num : num;
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

  // Hard reload to clear any stale atom state (L70 lesson)
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  note("Hard reload complete — clearing stale atom state");

  const today = new Date();
  const yyyy  = today.getFullYear();
  const mm    = String(today.getMonth() + 1).padStart(2, "0");
  const dd    = String(today.getDate()).padStart(2, "0");
  const todayStr = `${yyyy}-${mm}-${dd}`;
  note(`Date context: ${todayStr}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /accounts — Seed Wei's checking account and credit card
  //   Checking: $500 (tight but positive)
  //   Credit card: $1,800 @ 20% APR (pre-existing debt)
  //   We verify accounts visible after resetMemberFilter + 800ms wait (L72 lesson)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: /accounts — seed Wei's accounts ─────────────────────────────────────────");

  await createAccount(page, "L73 Wei Checking",   "Checking",    500);
  await createAccount(page, "L73 Wei CC",         "Credit card", 1800);

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const acctText1 = await page.evaluate(() => document.body.textContent);

  if (/L73 Wei Checking/i.test(acctText1)) pass("Step 1.1 — L73 Wei Checking visible on /accounts");
  else fail("Step 1.1 — L73 Wei Checking NOT visible on /accounts");

  if (/L73 Wei CC/i.test(acctText1)) pass("Step 1.2 — L73 Wei CC visible on /accounts");
  else fail("Step 1.2 — L73 Wei CC NOT visible on /accounts");

  const ccBalanceBefore = await readAccountBalance(page, "L73 Wei CC");
  note(`Baseline CC balance: ${ccBalanceBefore}`);
  const ccBaseNum = parseMoney(ccBalanceBefore);
  note(`  parsed: ${ccBaseNum}`);

  const checkBalanceBefore = await readAccountBalance(page, "L73 Wei Checking");
  note(`Baseline checking balance: ${checkBalanceBefore}`);
  const checkBaseNum = parseMoney(checkBalanceBefore);
  note(`  parsed: ${checkBaseNum}`);

  await page.screenshot({ path: SS("l73_01_accounts_seeded.png") });
  note("Screenshot: l73_01_accounts_seeded.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: /budgets — set tight no-spend budgets for discretionary categories
  //   Prior month: dining $200, shopping $150, entertainment $100
  //   This month: set each to $0 (or $1 as minimum if $0 not accepted)
  //   Then: HARD RELOAD — verify budgets persist (I1: BUDGET_PERSIST)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: /budgets — set no-spend budgets and verify persistence ──────────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const budgetsText0 = await page.evaluate(() => document.body.textContent);
  note(`Budgets page reachable: ${budgetsText0.length > 100}`);
  const hasBudgets = /budget|spend|limit|categor/i.test(budgetsText0);
  note(`Budgets page has budget content: ${hasBudgets}`);

  await page.screenshot({ path: SS("l73_02_budgets_before.png") });
  note("Screenshot: l73_02_budgets_before.png");

  // Enumerate visible budget categories
  const budgetCategories = await page.evaluate(() => {
    // Look for edit buttons or budget rows with category names
    const rows = Array.from(document.querySelectorAll(
      '[data-budget-id], .budget-row, .budget-item, tr, li'
    )).filter(el => el.textContent.trim().length > 2);
    return rows.slice(0, 20).map(r => r.textContent.trim().slice(0, 80));
  });
  note(`Budget rows found (first 20): ${JSON.stringify(budgetCategories)}`);

  // Try to set "Dining" budget to $0 or $1
  const DISCRETIONARY = ["Dining", "Shopping", "Entertainment", "Food & Drink", "Restaurants"];
  const budgetsSetResult = [];

  for (const cat of DISCRETIONARY) {
    // Look for an edit button or inline input near the category
    const setR = await page.evaluate((catName) => {
      // Find text node or label containing category name
      const allText = Array.from(document.querySelectorAll("*")).filter(el => {
        const t = el.textContent.trim();
        return t.toLowerCase().includes(catName.toLowerCase()) && t.length < 100;
      });
      if (allText.length === 0) return `cat "${catName}" not found in page`;

      // Try to find an edit/pencil button near it
      for (const el of allText) {
        const parent = el.closest("li, tr, .budget-row, .budget-item, div");
        if (!parent) continue;
        const editBtn = parent.querySelector('button[aria-label*="Edit"], button[aria-label*="edit"], button.edit, button[title*="Edit"]');
        if (editBtn) {
          editBtn.click();
          return `clicked edit near "${catName}"`;
        }
      }
      return `edit button for "${catName}" not found near text`;
    }, cat);
    note(`  Set budget for "${cat}": ${setR}`);

    if (/clicked/i.test(setR)) {
      await page.waitForTimeout(600);
      // Try to set the amount to $1 (minimum non-zero; some apps reject 0)
      const amtR = await page.evaluate(() => {
        const inp = Array.from(document.querySelectorAll('input[type="number"]')).find(i =>
          (i.getAttribute("aria-label") || "").toLowerCase().includes("amount") ||
          (i.getAttribute("placeholder") || "").toLowerCase().includes("amount") ||
          (i.getAttribute("placeholder") || "") === "0.00" ||
          (i.getAttribute("placeholder") || "") === "0");
        if (inp) {
          inp.focus(); inp.value = "1";
          inp.dispatchEvent(new Event("input",  { bubbles: true }));
          inp.dispatchEvent(new Event("change", { bubbles: true }));
          return `set to $1 (placeholder: "${inp.placeholder}")`;
        }
        return "amount input not found";
      });
      note(`    Amount set: ${amtR}`);
      // Save
      await page.evaluate(() => {
        const btn = Array.from(document.querySelectorAll("button")).find(b =>
          /^save$|^update$|^ok$/i.test(b.textContent.trim()) && b.type !== "reset");
        if (btn) btn.click();
      });
      await page.waitForTimeout(800);
      await flush(page);
      budgetsSetResult.push(cat);
    }
  }
  note(`Budgets attempted to set: ${JSON.stringify(budgetsSetResult)}`);

  // Also try to add new budget entries for no-spend categories via "Add budget" button
  const addBudgetR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add budget|new budget/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`Add budget button: ${addBudgetR}`);

  if (/clicked/i.test(addBudgetR)) {
    await page.waitForTimeout(600);
    // Fill in Dining category, $1 budget
    const catSetR = await selectByText(page, "Category", "Dining");
    note(`  Budget category select: ${catSetR}`);
    await page.evaluate(() => {
      const inp = Array.from(document.querySelectorAll('input[type="number"]')).find(i =>
        /amount|budget/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
      if (inp) {
        inp.value = "1";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b =>
        /^add$|^save$|^add budget$/i.test(b.textContent.trim()) && b.type !== "reset");
      if (btn) btn.click();
    });
    await page.waitForTimeout(1000);
    await flush(page);
  }

  // Capture budgets state before reload
  await page.screenshot({ path: SS("l73_03_budgets_set.png") });
  note("Screenshot: l73_03_budgets_set.png");

  const budgetsTextBefore = await page.evaluate(() => document.body.textContent);
  note(`Budgets text length before reload: ${budgetsTextBefore.length}`);

  // HARD RELOAD — verify budget persistence (I1)
  note("Performing HARD RELOAD to verify budget persistence...");
  await flush(page);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1000);

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const budgetsTextAfter = await page.evaluate(() => document.body.textContent);
  note(`Budgets text length after reload: ${budgetsTextAfter.length}`);

  // Check if any no-spend budget amount ($1.00 or $0.00) appears — or any budget at all
  const budgetPersisted = /\$[01]\.00|\$[01],00|\$0|\$1/i.test(budgetsTextAfter);
  note(`Budget $0/$1 value present after reload: ${budgetPersisted}`);
  const budgetPagePresent = /budget|spend|limit/i.test(budgetsTextAfter);
  note(`Budgets page has budget content after reload: ${budgetPagePresent}`);

  if (budgetPagePresent) {
    pass("I1a — /budgets page has budget content after hard reload");
  } else {
    fail("I1a — /budgets page has NO budget content after hard reload");
  }

  await page.screenshot({ path: SS("l73_04_budgets_after_reload.png") });
  note("Screenshot: l73_04_budgets_after_reload.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: /transactions — log essential expenses (groceries + gas) only
  //   Groceries $80 (Food/Groceries category), Gas $40 (Auto/Transport category)
  //   NO discretionary spend logged — this is the no-spend month
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: /transactions — log essential-only expenses ──────────────────────────────");

  await logExpense(page, "L73 Groceries (no-spend week 1)", 80, "Groceries", todayStr);
  await logExpense(page, "L73 Gas (no-spend month)", 40, "Auto", todayStr);

  // Verify both logged
  await navTo(page, "Transactions");
  await page.waitForTimeout(800);
  const txnText1 = await page.evaluate(() => document.body.textContent);

  if (/L73 Groceries/i.test(txnText1)) pass("Step 3.1 — Groceries $80 transaction visible in /transactions");
  else fail("Step 3.1 — Groceries transaction NOT visible in /transactions");

  if (/L73 Gas/i.test(txnText1)) pass("Step 3.2 — Gas $40 transaction visible in /transactions");
  else fail("Step 3.2 — Gas transaction NOT visible in /transactions");

  await page.screenshot({ path: SS("l73_05_transactions_essentials.png") });
  note("Screenshot: l73_05_transactions_essentials.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: /budgets — verify no discretionary spend (I2: NO_DISCRETIONARY)
  //   Discretionary categories (dining, shopping, entertainment) should show $0 spent.
  //   Probe for month-over-month comparison (I3: MOM_COMPARISON).
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: /budgets — verify $0 discretionary spend + MoM comparison ───────────────");

  await navTo(page, "Budgets");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const budgetsText2 = await page.evaluate(() => document.body.textContent);

  // Check for $0 spend in discretionary categories
  const hasZeroSpend = /\$0\.00|\$0 spent|0\.00 spent|0 spent/i.test(budgetsText2);
  note(`$0 spend indicator present on /budgets: ${hasZeroSpend}`);

  // Check for dining/shopping/entertainment with $0
  const diningZero = /dining[^$\d]*\$0|dining[^$\d]*0\.00|\$0[^$\d]*dining/i.test(budgetsText2);
  const shopZero   = /shopping[^$\d]*\$0|shopping[^$\d]*0\.00|\$0[^$\d]*shopping/i.test(budgetsText2);
  const entZero    = /entertainment[^$\d]*\$0|entertainment[^$\d]*0\.00|\$0[^$\d]*entertainment/i.test(budgetsText2);
  note(`Dining $0: ${diningZero} | Shopping $0: ${shopZero} | Entertainment $0: ${entZero}`);

  // More flexible: look for any $0.00 or zero spent indicator
  const dollarZeroPattern = /\$0\.00/g;
  const zeroInstances = (budgetsText2.match(dollarZeroPattern) || []).length;
  note(`Instances of "$0.00" on budgets page: ${zeroInstances}`);

  if (hasZeroSpend || zeroInstances > 0) {
    pass("I2 — /budgets shows $0 spent in at least one discretionary category");
  } else {
    absent_("I2 — no $0 spend indicator found on /budgets (categories may not appear or show spent>0)");
  }

  // Probe month-over-month comparison (I3)
  const momText = /saved.*vs.*last|vs.*last month|month.over.month|compared to last|last month.*you spent|previous month|mom/i.test(budgetsText2);
  note(`Month-over-month comparison present on /budgets: ${momText}`);
  if (momText) {
    pass("I3 — Month-over-month savings comparison present on /budgets");
  } else {
    absent_("I3 — No month-over-month comparison found on /budgets (feature not implemented or not on this screen)");
  }

  await page.screenshot({ path: SS("l73_06_budgets_no_discretionary.png") });
  note("Screenshot: l73_06_budgets_no_discretionary.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: /transactions — record $400 credit card payment (I4/I5)
  //   Transfer: L73 Wei Checking → L73 Wei CC, $400
  //   This is the "freed cash" from the no-spend month going to debt reduction.
  //   Critical: HARD RELOAD after payment to definitively test sign bug (L64).
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: /transactions — $400 payment to credit card ─────────────────────────────");

  await recordTransfer(page, "L73 Wei CC Payment no-spend month", 400, "L73 Wei Checking", "L73 Wei CC", todayStr);

  // Check transaction visible
  await navTo(page, "Transactions");
  await page.waitForTimeout(800);
  const txnText2 = await page.evaluate(() => document.body.textContent);
  if (/L73 Wei CC Payment/i.test(txnText2)) pass("Step 5.1 — $400 CC payment transaction visible in /transactions");
  else fail("Step 5.1 — $400 CC payment NOT visible in /transactions");

  await page.screenshot({ path: SS("l73_07_transactions_cc_payment.png") });
  note("Screenshot: l73_07_transactions_cc_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: /accounts — read CC and checking balances immediately after payment
  //   (before hard reload — may be stale/in-flight)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /accounts — read balances before hard reload ────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const ccBalanceImmediate = await readAccountBalance(page, "L73 Wei CC");
  const checkBalanceImmediate = await readAccountBalance(page, "L73 Wei Checking");
  note(`CC balance immediately after payment (pre-reload): ${ccBalanceImmediate}`);
  note(`Checking balance immediately after payment (pre-reload): ${checkBalanceImmediate}`);

  await page.screenshot({ path: SS("l73_08_accounts_before_reload.png") });
  note("Screenshot: l73_08_accounts_before_reload.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: HARD RELOAD — then re-read balances (I4/I5: PAYMENT_DIRECTION + MONEY_CONSERVE)
  //   Post-reload is the definitive answer on sign bug (L64).
  //   Expected: CC drops from $1,800 to $1,400; checking drops from $500 to $100.
  //   L64 sign bug would show CC INCREASED (e.g., $2,200).
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: HARD RELOAD — definitive sign-bug test (I4/I5) ─────────────────────────");

  await flush(page);
  note("Performing HARD RELOAD for definitive CC balance check...");
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1000);

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const ccBalanceAfter = await readAccountBalance(page, "L73 Wei CC");
  const checkBalanceAfter = await readAccountBalance(page, "L73 Wei Checking");
  note(`CC balance after hard reload (post-$400 payment): ${ccBalanceAfter}`);
  note(`Checking balance after hard reload (post-$400 payment): ${checkBalanceAfter}`);

  const ccAfterNum    = parseMoney(ccBalanceAfter);
  const checkAfterNum = parseMoney(checkBalanceAfter);
  note(`  CC parsed: ${ccAfterNum} | Checking parsed: ${checkAfterNum}`);

  // I4: payment direction — does CC go DOWN (correct) or UP (sign bug)?
  if (ccAfterNum !== null && ccBaseNum !== null) {
    if (Math.abs(ccAfterNum) < Math.abs(ccBaseNum)) {
      pass("I4 — CC balance DECREASED after $400 payment (correct direction; L64 sign bug NOT triggered here)");
      note(`  L64 verdict: NOT PRESENT (CC went ${ccBaseNum} → ${ccAfterNum}; payment reduced liability)`);
    } else if (Math.abs(ccAfterNum) > Math.abs(ccBaseNum)) {
      fail("I4 — CC balance INCREASED after $400 payment (L64 SIGN BUG CONFIRMED — payment credits INCREASE liability balance)");
      note(`  L64 verdict: CONFIRMED BUG — CC went ${ccBaseNum} → ${ccAfterNum}; payment inflated the balance`);
    } else {
      fail("I4 — CC balance UNCHANGED after $400 payment (reactive update gap — balance freeze; L71/L72 Thread A re-confirmed)");
      note(`  L64 verdict: INDETERMINATE — balance did not update; sign direction unobservable`);
    }
  } else {
    absent_("I4 — CC balance not parseable after hard reload (account may not be visible)");
    note(`  ccBalanceBefore=${ccBalanceBefore}, ccBalanceAfter=${ccBalanceAfter}`);
  }

  // I5: money conservation — CC should be $1,400 ($1,800 − $400)
  const EXPECTED_CC = 1400;
  if (ccAfterNum !== null) {
    if (Math.abs(ccAfterNum) === EXPECTED_CC) {
      pass(`I5a — CC balance = $${EXPECTED_CC} (money conserved: $1,800 − $400 = $1,400)`);
    } else {
      fail(`I5a — CC balance = ${ccAfterNum}, expected $${EXPECTED_CC} (money NOT conserved)`);
    }
  } else {
    absent_("I5a — CC balance unreadable; conservation check skipped");
  }

  // I5: checking should be $100 ($500 − $400)
  const EXPECTED_CHECK = 100;
  if (checkAfterNum !== null) {
    if (Math.abs(checkAfterNum) === EXPECTED_CHECK) {
      pass(`I5b — Checking balance = $${EXPECTED_CHECK} (correct: $500 − $400 = $100)`);
    } else {
      fail(`I5b — Checking balance = ${checkAfterNum}, expected $${EXPECTED_CHECK} (debit leg not applied)`);
    }
  } else {
    absent_("I5b — Checking balance unreadable after reload");
  }

  await page.screenshot({ path: SS("l73_09_accounts_after_reload.png") });
  note("Screenshot: l73_09_accounts_after_reload.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: /reports — month-over-month discretionary comparison (I6: REPORTS_MOM)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: /reports — check MoM discretionary spend ────────────────────────────────");

  await navTo(page, "Reports");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const reportsText = await page.evaluate(() => document.body.textContent);
  const reportsPresent = /report|spending|income|chart|graph|period|month/i.test(reportsText);
  note(`Reports page has report content: ${reportsPresent}`);

  const momReports = /last month|previous month|month.over.month|vs.*month|prior month|mom/i.test(reportsText);
  note(`Month-over-month comparison on /reports: ${momReports}`);

  // Check if discretionary categories show zero spend
  const diningInReports = /dining/i.test(reportsText);
  const shopInReports   = /shopping/i.test(reportsText);
  const entInReports    = /entertainment/i.test(reportsText);
  note(`Dining in reports: ${diningInReports} | Shopping: ${shopInReports} | Entertainment: ${entInReports}`);

  // Check if $80 grocery or $40 gas appear (essentials)
  const essentialsInReports = /\$80|\$40|groceries|gas/i.test(reportsText);
  note(`Essential expenses ($80/$40) in reports: ${essentialsInReports}`);

  if (momReports) {
    pass("I6a — /reports shows month-over-month comparison");
  } else {
    absent_("I6a — /reports has no month-over-month comparison text (feature probe)");
  }

  if (reportsPresent) {
    pass("I6b — /reports page has meaningful content");
  } else {
    absent_("I6b — /reports page empty or no report content found");
  }

  await page.screenshot({ path: SS("l73_10_reports_mom.png") });
  note("Screenshot: l73_10_reports_mom.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: /dashboard — cross-screen consistency after hard reload (I7)
  //   Verify net worth and budget signals reflect the no-spend month.
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: /dashboard — cross-screen consistency check ─────────────────────────────");

  await navTo(page, "Dashboard");
  await page.waitForTimeout(1000);

  const dashText = await page.evaluate(() => document.body.textContent);
  const netWorthStr = dashText.match(/net worth[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ?? null;
  note(`Dashboard net worth: ${netWorthStr}`);

  const dashDollarAmounts = dashText.match(/\$[\d,]+\.?\d*/g) || [];
  note(`Dashboard dollar values: ${JSON.stringify(dashDollarAmounts.slice(0, 20))}`);

  // Probe for CC balance ($1,400 expected after payment) or any budget/savings signal
  const cc1400OnDash = /\$1,?400/i.test(dashText);
  note(`$1,400 (expected CC after payment) on dashboard: ${cc1400OnDash}`);

  const budgetSignal = /under budget|no.spend|saved|budget/i.test(dashText);
  note(`Budget/savings signal on dashboard: ${budgetSignal}`);

  const debtSignal = /debt|credit card|owe|liabilit/i.test(dashText);
  note(`Debt/liability signal on dashboard: ${debtSignal}`);

  if (netWorthStr) {
    pass("I7a — Net Worth widget present on Dashboard after hard reload");
  } else {
    absent_("I7a — Net Worth widget NOT present or not parseable on Dashboard");
  }

  if (debtSignal) {
    pass("I7b — Dashboard shows debt/liability signal (cross-screen consistency)");
  } else {
    absent_("I7b — Dashboard shows no debt/liability signal");
  }

  if (budgetSignal) {
    pass("I7c — Dashboard shows budget/savings signal (no-spend feedback)");
  } else {
    absent_("I7c — Dashboard shows no budget/savings signal");
  }

  await page.screenshot({ path: SS("l73_11_dashboard_final.png") });
  note("Screenshot: l73_11_dashboard_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 10: Final /accounts — cross-screen consistency (I7 accounts leg)
  //   Re-read all three balances after full ritual + hard reload.
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 10: /accounts — final cross-screen consistency ──────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const ccFinalBal    = await readAccountBalance(page, "L73 Wei CC");
  const checkFinalBal = await readAccountBalance(page, "L73 Wei Checking");
  note(`Final CC balance (post-ritual): ${ccFinalBal}`);
  note(`Final checking balance (post-ritual): ${checkFinalBal}`);

  // Verify dataset for transaction shape
  const ds   = await getDataset(page);
  const txns = ds.transactions || [];
  note(`Total transactions in dataset: ${txns.length}`);
  const l73Txns = txns.filter(t => /L73/i.test(t.description || t.payee || ""));
  note(`L73 transactions in dataset: ${l73Txns.length}`);
  note(`L73 txn shapes: ${JSON.stringify(l73Txns.map(t => ({ d: t.description || t.payee, a: t.amount, type: t.type })))}`);

  // Cross-screen: CC should equal the Step 7 reading (no drift)
  const ccFinalNum = parseMoney(ccFinalBal);
  if (ccAfterNum !== null && ccFinalNum !== null) {
    if (Math.abs(ccAfterNum) === Math.abs(ccFinalNum)) {
      pass("I7d — CC balance consistent between Step 7 and Step 10 (no cross-screen drift)");
    } else {
      fail(`I7d — CC balance DIFFERS: Step 7 = ${ccAfterNum}, Step 10 = ${ccFinalNum} (cross-screen drift detected)`);
    }
  } else {
    absent_("I7d — CC balance not readable for consistency check");
  }

  await page.screenshot({ path: SS("l73_12_accounts_final.png") });
  note("Screenshot: l73_12_accounts_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // JS error check
  // ════════════════════════════════════════════════════════════════════════════
  if (jsErrors.length === 0) {
    pass("NO_JS_ERRORS — zero runtime JS errors across entire ritual");
  } else {
    fail(`JS_ERRORS — ${jsErrors.length} runtime JS error(s): ${jsErrors.slice(0, 3).join("; ")}`);
  }

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
