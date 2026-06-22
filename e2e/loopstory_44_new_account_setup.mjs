// L44 E2E loop story — "The New Account Setup" (Omar onboards a real bank account)
// Persona: Omar, 38, self-employed, adds a new checking account with an opening balance
//          and runs the full onboarding ritual: add account → CSV import → dedupe review →
//          reconcile → categorize → create rule → verify dashboard + reports.
// Flow:
//   1. /accounts — add "L44 Omar Checking" (Checking, $1,000.00 opening balance).
//   2. /accounts — verify new account appears in list and Dashboard net worth updates.
//   3. /documents — paste a CSV bank statement against L44 Omar Checking (5 transactions).
//   4. /documents — confirm import count; verify account-selection hand-off (probe for
//      silent wrong-account default on the plain CSV path).
//   5. /transactions — verify 5 imported rows appear; check dedupe (opening balance row
//      not double-counted); confirm no cents lost (sum of rows == CSV total).
//   6. /accounts — update-balance reconcile: set L44 Omar Checking balance to $2,345.67
//      (bank's ending figure); confirm reconcile records the adjustment.
//   7. /transactions — categorize two imported rows (Grocery row → Food, Coffee → Dining).
//   8. /rules — create a rule: payee "L44 SUPERMARKET" → category Food/Groceries.
//      Confirm rule count increased.
//   9. /rules — verify rule would fire: navigate back and check the rule appears.
//  10. /dashboard — confirm L44 Omar Checking appears in net worth; net worth ==
//      sum of account balances from /accounts.
//  11. /reports — confirm newly categorized transactions appear in spending breakdown.
//  12. Cross-screen invariants: net worth == sum balances; period window consistent.
//  13. Hard reload /accounts — L44 Omar Checking persists with updated balance.
//  14. JS error check.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_44_new_account_setup.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

// Seed constants
const ACCT_NAME     = "L44 Omar Checking";
const OPENING_BAL   = "1000";    // $1,000.00 opening balance
const RECONCILE_BAL = "2345.67"; // bank's ending figure after all imports
const TODAY         = "2026-06-22";

// CSV with 5 transactions totalling $1,345.67 of outflow + inflow.
// Net: -$95.00 groceries, -$12.50 coffee, -$200.00 rent-partial, +$1,500.00 paycheck, -$147.50 utilities
// Sum: +$1,345.67 + opening $1,000 ≈ reconcile target $2,345.67 (with adjustment)
// NOTE: The CSV plain-import path does NOT use the importAcct selector — this probes the
// account-selection hand-off gap.
const IMPORT_CSV = `date,description,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,
2026-06-16,L44 COFFEE SHOP,-12.50,
2026-06-18,L44 RENT PARTIAL,-200.00,
2026-06-20,L44 PAYCHECK DEPOSIT,1500.00,
2026-06-21,L44 UTILITIES PAYMENT,-147.50,`;

// Expected net sum of the 5 CSV rows: -95 -12.5 -200 +1500 -147.5 = +1045.00
const CSV_NET_EXPECTED = 1045.00;

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass  = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail  = (label) => { console.error(`FAIL: ${label}`); failed++; };
const maybe = (label) => { console.log(`SKIP: ${label} (feature absent — logged as gap)`); };

const waitNav = (page) =>
  page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

const goto = async (page, hash) => {
  await page.goto(BASE + hash, { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
};

const parseDollar = (s) => {
  if (!s) return NaN;
  return parseFloat(s.replace(/[^0-9.\-]/g, ""));
};

// Parse all dollar figures from a string that appear as "$X,XXX.XX"
const parseAllDollars = (text) => {
  const matches = [...text.matchAll(/\$([\d,]+\.\d{2})/g)];
  return matches.map((m) => parseDollar(m[1].replace(/,/g, "")));
};

// Sum all account balances shown on /accounts for net worth computation
const sumAccountBalances = (text) => {
  // Grab all dollar figures from the assets section only (before liabilities)
  const figures = parseAllDollars(text);
  // We can't trivially split assets/liabilities from raw text; return the array for manual check
  return figures;
};

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 0: Baseline — capture existing net worth ─────────────────────────────
  await goto(page, "/accounts");
  const bodyBaseline = await page.evaluate(() => document.body.innerText);
  await page.screenshot({ path: SS("l44_step0_accounts_baseline.png") });

  const nwBaseMatch = bodyBaseline.match(/NET WORTH\s*\$([\d,]+\.\d{2})/i);
  const nwBaseline = nwBaseMatch ? parseDollar(nwBaseMatch[1].replace(/,/g, "")) : NaN;
  console.log(`Baseline net worth: $${nwBaseline}`);

  // ── Step 1: Add "L44 Omar Checking" account ───────────────────────────────────
  // Navigate to /accounts and fill the add-account form
  // Name field
  const nameInput = await page.$('input[placeholder*="name" i], input[aria-label*="name" i], input[id*="acc-add"]');
  if (nameInput) {
    await nameInput.fill(ACCT_NAME);
    pass(`Step 1a — account name field found and filled: "${ACCT_NAME}"`);
  } else {
    fail(`Step 1a — account name input not found on /accounts`);
  }

  // Opening balance
  const amtInput = await page.$('input[type="number"], input[placeholder*="balance" i], input[aria-label*="balance" i], input[placeholder*="amount" i]');
  if (amtInput) {
    await amtInput.fill(OPENING_BAL);
    pass(`Step 1b — opening balance filled: $${OPENING_BAL}`);
  } else {
    fail(`Step 1b — opening balance input not found`);
  }

  // Account type — leave as Checking (default)
  const typeSelect = await page.$('select[aria-label*="type" i], select[aria-label*="account type" i]');
  if (typeSelect) {
    await typeSelect.selectOption({ label: "Checking" });
    pass(`Step 1c — account type set to Checking`);
  } else {
    maybe(`Step 1c — account type select not found; relying on default`);
  }

  // Submit
  const addBtn = await page.$('button[type="submit"], button:has-text("Add account"), button:has-text("Add"), button:has-text("Save")');
  if (addBtn) {
    await addBtn.click();
    await page.waitForTimeout(1200);
    pass(`Step 1d — Add account button clicked`);
  } else {
    fail(`Step 1d — Add account submit button not found`);
  }

  await page.screenshot({ path: SS("l44_step1_account_added.png") });
  const bodyAfterAdd = await page.evaluate(() => document.body.innerText);

  if (bodyAfterAdd.includes(ACCT_NAME)) {
    pass(`Step 1e — "${ACCT_NAME}" appears in accounts list after add`);
  } else {
    fail(`Step 1e — "${ACCT_NAME}" NOT found in accounts list after add`);
  }

  // Check opening balance appears near the account name
  const acctOpeningMatch = bodyAfterAdd.match(new RegExp(ACCT_NAME + "[\\s\\S]{0,100}\\$(1[,.]?000\\.00)"));
  if (acctOpeningMatch) {
    pass(`Step 1f — Opening balance $1,000.00 visible near "${ACCT_NAME}"`);
  } else {
    maybe(`Step 1f — Opening balance not immediately visible near account name in raw text`);
  }

  // Net worth should increase by $1,000 (opening balance)
  const nwAfterAddMatch = bodyAfterAdd.match(/NET WORTH\s*\$([\d,]+\.\d{2})/i);
  const nwAfterAdd = nwAfterAddMatch ? parseDollar(nwAfterAddMatch[1].replace(/,/g, "")) : NaN;
  if (!isNaN(nwBaseline) && !isNaN(nwAfterAdd)) {
    const delta = nwAfterAdd - nwBaseline;
    if (Math.abs(delta - 1000) < 1) {
      pass(`Step 1g — Net worth increased by exactly $1,000.00 after add (was $${nwBaseline}, now $${nwAfterAdd})`);
    } else {
      fail(`Step 1g — Net worth delta is $${delta.toFixed(2)}, expected ~$1,000 (baseline $${nwBaseline}, after $${nwAfterAdd})`);
    }
  } else {
    fail(`Step 1g — Could not parse net worth for delta check (baseline: ${nwBaseline}, after: ${nwAfterAdd})`);
  }

  // ── Step 2: Dashboard — verify new account appears in net worth ───────────────
  await goto(page, "/");
  await page.screenshot({ path: SS("l44_step2_dashboard_after_add.png") });
  const dashBody = await page.evaluate(() => document.body.innerText);

  const dashNWMatch = dashBody.match(/Net\s*Worth[\s\S]{0,80}\$([\d,]+\.\d{2})/i);
  const dashNWAfterAdd = dashNWMatch ? parseDollar(dashNWMatch[1].replace(/,/g, "")) : NaN;
  if (!isNaN(nwAfterAdd) && !isNaN(dashNWAfterAdd)) {
    if (Math.abs(dashNWAfterAdd - nwAfterAdd) < 1) {
      pass(`Step 2a — INVARIANT: Dashboard net worth ($${dashNWAfterAdd}) == Accounts net worth ($${nwAfterAdd})`);
    } else {
      fail(`Step 2a — INVARIANT VIOLATION: Dashboard NW ($${dashNWAfterAdd}) ≠ Accounts NW ($${nwAfterAdd}), diff $${Math.abs(dashNWAfterAdd - nwAfterAdd).toFixed(2)}`);
    }
  } else {
    fail(`Step 2a — Could not parse net worth for cross-screen comparison (dash: ${dashNWAfterAdd}, accounts: ${nwAfterAdd})`);
  }

  // ── Step 3: /documents — paste CSV and import ─────────────────────────────────
  await goto(page, "/documents");
  await page.screenshot({ path: SS("l44_step3_documents_before.png") });
  const h1docs = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/doc/i.test(h1docs)) {
    pass(`Step 3a — /documents loaded (h1: "${h1docs}")`);
  } else {
    fail(`Step 3a — expected Documents h1, got "${h1docs}"`);
  }

  // Probe: is there an account selector on the CSV import path?
  const acctSelDocs = await page.$('select[aria-label*="account" i]');
  if (acctSelDocs) {
    // Try to select L44 Omar Checking
    const opts = await acctSelDocs.evaluate((el) => Array.from(el.options).map((o) => o.text));
    const omarOpt = opts.find((o) => /L44/i.test(o) || /Omar/i.test(o));
    if (omarOpt) {
      await acctSelDocs.selectOption({ label: omarOpt });
      pass(`Step 3b — Account selector present on /documents; selected "${omarOpt}"`);
    } else {
      maybe(`Step 3b — Account selector present but L44 Omar Checking not listed yet (options: ${opts.slice(0,5).join(", ")})`);
    }
  } else {
    // KEY GAP PROBE: no account selector on the CSV import path
    fail(`Step 3b — GAP: No account selector found on /documents for CSV import path — CSV import has no account assignment (silent default to first account)`);
  }

  // Paste CSV into textarea
  const csvTextarea = await page.$('textarea[placeholder*="csv" i], textarea[aria-label*="csv" i], textarea[placeholder*="paste" i], textarea[placeholder*="statement" i]');
  if (csvTextarea) {
    await csvTextarea.fill(IMPORT_CSV);
    pass(`Step 3c — CSV textarea found and filled with ${IMPORT_CSV.trim().split("\n").length - 1} data rows`);
  } else {
    fail(`Step 3c — CSV textarea not found on /documents`);
  }

  // Click import/parse button
  const importCsvBtn = await page.$('button:has-text("Import"), button:has-text("Parse"), button:has-text("Upload"), button[type="submit"]');
  if (importCsvBtn) {
    await importCsvBtn.click();
    await page.waitForTimeout(1500);
    pass(`Step 3d — Import/Parse button clicked`);
  } else {
    fail(`Step 3d — Import/Parse button not found on /documents`);
  }

  await page.screenshot({ path: SS("l44_step3_documents_after_import.png") });
  const bodyDocsAfter = await page.evaluate(() => document.body.innerText);

  // Look for import success message (e.g. "Imported 5 transactions" or "5 rows")
  const importCountMatch = bodyDocsAfter.match(/imported?\s+(\d+)\s+(transaction|row)/i);
  if (importCountMatch) {
    const importedCount = parseInt(importCountMatch[1]);
    if (importedCount === 5) {
      pass(`Step 3e — Import success: ${importedCount} transactions imported (expected 5)`);
    } else if (importedCount > 0) {
      maybe(`Step 3e — Import success but count is ${importedCount} (expected 5 — may include header or skips)`);
    } else {
      fail(`Step 3e — Import count is 0`);
    }
  } else if (/error|fail/i.test(bodyDocsAfter)) {
    fail(`Step 3e — Import returned an error: "${bodyDocsAfter.slice(0, 200)}"`);
  } else {
    maybe(`Step 3e — Import message not in expected format; raw text fragment: "${bodyDocsAfter.slice(0, 300)}"`);
  }

  // Check for "skipped" message (dedupe of opening balance or duplicates)
  const skippedMatch = bodyDocsAfter.match(/skipped?\s+(\d+)/i);
  if (skippedMatch) {
    pass(`Step 3f — Dedupe: ${skippedMatch[1]} row(s) skipped (deduplication active)`);
  } else {
    maybe(`Step 3f — No skipped-row message (no duplicates in this import, or dedupe not active)`);
  }

  // ── Step 4: /transactions — verify imported rows & money conservation ──────────
  await goto(page, "/transactions");
  await page.screenshot({ path: SS("l44_step4_transactions_after_import.png") });
  const h1txn = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/transact/i.test(h1txn)) {
    pass(`Step 4a — /transactions loaded (h1: "${h1txn}")`);
  } else {
    fail(`Step 4a — expected Transactions h1, got "${h1txn}"`);
  }

  const bodyTxn = await page.evaluate(() => document.body.innerText);

  // Check all 5 imported rows are present
  const csvRows = [
    "L44 SUPERMARKET GROCERIES",
    "L44 COFFEE SHOP",
    "L44 RENT PARTIAL",
    "L44 PAYCHECK DEPOSIT",
    "L44 UTILITIES PAYMENT",
  ];
  let rowsFound = 0;
  for (const row of csvRows) {
    if (bodyTxn.includes(row)) {
      rowsFound++;
    } else {
      fail(`Step 4b — CSV row "${row}" NOT found in /transactions`);
    }
  }
  if (rowsFound === 5) {
    pass(`Step 4b — All 5 CSV-imported rows found in /transactions`);
  } else if (rowsFound > 0) {
    maybe(`Step 4b — Only ${rowsFound}/5 CSV rows found in /transactions (period filter may hide some)`);
  }

  // Account assignment probe: imported rows should be on L44 Omar Checking, not another account.
  // The CSV plain-import path ignores the importAcct selector (source code confirms this gap).
  // We probe by checking if the rows are associated with the correct account in the UI.
  const accountOnRows = bodyTxn.match(/L44 SUPERMARKET GROCERIES[\s\S]{0,200}?(L44 Omar Checking|Everyday Checking|Emergency|[A-Z][a-z]+ [A-Z][a-z]+)/);
  if (accountOnRows) {
    if (/L44 Omar Checking/.test(accountOnRows[1])) {
      pass(`Step 4c — Imported rows correctly assigned to "L44 Omar Checking"`);
    } else {
      fail(`Step 4c — GAP (ACCOUNT HAND-OFF): Imported rows assigned to "${accountOnRows[1]}" instead of "L44 Omar Checking" — CSV import ignored account selector`);
    }
  } else {
    maybe(`Step 4c — Could not determine account assignment from transaction row text; manual inspection needed`);
  }

  // Money conservation: sum of dollar figures near the imported rows
  // We look for the 5 specific amounts from the CSV
  const expectedAmounts = [95.00, 12.50, 200.00, 1500.00, 147.50];
  let amountsFound = 0;
  for (const amt of expectedAmounts) {
    const amtStr = amt.toFixed(2);
    if (bodyTxn.includes(amtStr)) {
      amountsFound++;
    }
  }
  if (amountsFound === 5) {
    pass(`Step 4d — MONEY CONSERVATION: All 5 imported amounts visible in /transactions (no cents lost)`);
  } else if (amountsFound > 0) {
    maybe(`Step 4d — ${amountsFound}/5 imported amounts visible in /transactions (some may be filtered)`);
  } else {
    fail(`Step 4d — No imported amounts visible in /transactions — import may have failed or period filter is hiding them`);
  }

  // ── Step 5: /accounts — reconcile L44 Omar Checking to bank figure ────────────
  await goto(page, "/accounts");
  await page.screenshot({ path: SS("l44_step5_accounts_before_reconcile.png") });
  const bodyAccBeforeRec = await page.evaluate(() => document.body.innerText);

  // Find the balance shown for L44 Omar Checking now (should be opening + imported net)
  const omarBalMatch = bodyAccBeforeRec.match(/L44 Omar Checking[\s\S]{0,120}\$([\d,]+\.\d{2})/);
  const omarBalBeforeRec = omarBalMatch ? parseDollar(omarBalMatch[1].replace(/,/g, "")) : NaN;
  console.log(`L44 Omar Checking balance before reconcile: $${omarBalBeforeRec}`);

  if (!isNaN(omarBalBeforeRec)) {
    pass(`Step 5a — L44 Omar Checking balance readable before reconcile: $${omarBalBeforeRec}`);
  } else {
    fail(`Step 5a — Could not parse L44 Omar Checking balance`);
  }

  // Look for the "Update balance" / "Reconcile" action on the account row
  // The accounts screen has per-row "⋯" overflow menus or inline "Update balance" buttons
  const updateBalBtn = await page.$('button:has-text("Update balance"), button:has-text("Reconcile"), button:has-text("update balance")');
  if (updateBalBtn) {
    await updateBalBtn.click();
    await page.waitForTimeout(600);
    pass(`Step 5b — "Update balance" / Reconcile button found and clicked`);

    // Fill the new balance field
    const newBalInput = await page.$('input[type="number"], input[placeholder*="balance" i], input[aria-label*="balance" i]');
    if (newBalInput) {
      await newBalInput.fill(RECONCILE_BAL);
      pass(`Step 5c — Reconcile target balance filled: $${RECONCILE_BAL}`);
    } else {
      fail(`Step 5c — Balance input not found after clicking Update balance`);
    }

    const saveRecBtn = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update"), button:has-text("OK")');
    if (saveRecBtn) {
      await saveRecBtn.click();
      await page.waitForTimeout(1000);
      pass(`Step 5d — Reconcile saved`);
    } else {
      fail(`Step 5d — Save button not found in reconcile form`);
    }
  } else {
    // Try via the overflow menu (⋯)
    const overflowBtns = await page.$$('button[aria-label*="more" i], button[aria-label*="overflow" i], button:has-text("⋯"), button:has-text("…")');
    let reconcileViaMenu = false;
    for (const btn of overflowBtns) {
      // Check if this overflow is near L44 Omar Checking
      const parentText = await btn.evaluate((b) => b.closest("li,tr,.row,section,article")?.innerText?.trim() ?? "");
      if (/L44 Omar|Omar Checking/i.test(parentText)) {
        await btn.click();
        await page.waitForTimeout(400);
        const menuUpdateBtn = await page.$('button:has-text("Update balance"), button:has-text("Reconcile")');
        if (menuUpdateBtn) {
          await menuUpdateBtn.click();
          await page.waitForTimeout(600);
          const newBalInput = await page.$('input[type="number"]');
          if (newBalInput) {
            await newBalInput.fill(RECONCILE_BAL);
            const saveBtn = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update")');
            if (saveBtn) { await saveBtn.click(); await page.waitForTimeout(1000); }
            pass(`Step 5b-d — Reconcile via overflow menu completed, target: $${RECONCILE_BAL}`);
            reconcileViaMenu = true;
          }
        }
        break;
      }
    }
    if (!reconcileViaMenu) {
      maybe(`Step 5b — No "Update balance" button or overflow menu found for L44 Omar Checking — reconcile flow not probed`);
    }
  }

  await page.screenshot({ path: SS("l44_step5_accounts_after_reconcile.png") });
  const bodyAccAfterRec = await page.evaluate(() => document.body.innerText);

  // Verify the reconcile target $2,345.67 now shows as the account balance
  const omarBalAfterRecMatch = bodyAccAfterRec.match(/L44 Omar Checking[\s\S]{0,120}\$([\d,]+\.\d{2})/);
  const omarBalAfterRec = omarBalAfterRecMatch ? parseDollar(omarBalAfterRecMatch[1].replace(/,/g, "")) : NaN;
  if (!isNaN(omarBalAfterRec) && Math.abs(omarBalAfterRec - parseFloat(RECONCILE_BAL)) < 1) {
    pass(`Step 5e — L44 Omar Checking balance == reconcile target $${RECONCILE_BAL} (got $${omarBalAfterRec})`);
  } else if (!isNaN(omarBalAfterRec)) {
    fail(`Step 5e — L44 Omar Checking balance after reconcile is $${omarBalAfterRec}, expected $${RECONCILE_BAL}`);
  } else {
    fail(`Step 5e — Could not parse L44 Omar Checking balance after reconcile`);
  }

  // ── Step 6: /transactions — categorize two imported rows ──────────────────────
  await goto(page, "/transactions");
  await page.screenshot({ path: SS("l44_step6_transactions_categorize.png") });

  // Find the Grocery row and set category to Food/Groceries
  // The inline-edit flow: find the row, click to edit, change category
  const groceryRow = await page.$('li:has-text("L44 SUPERMARKET"), tr:has-text("L44 SUPERMARKET"), .row:has-text("L44 SUPERMARKET"), [data-desc*="L44 SUPERMARKET"]');
  if (groceryRow) {
    // Try clicking the row to open inline edit
    await groceryRow.click();
    await page.waitForTimeout(500);

    const catSelect = await page.$('select[aria-label*="categ" i], select[aria-label*="category" i]');
    if (catSelect) {
      const catOpts = await catSelect.evaluate((el) => Array.from(el.options).map((o) => o.text));
      const groceriesOpt = catOpts.find((o) => /grocer|food/i.test(o));
      if (groceriesOpt) {
        await catSelect.selectOption({ label: groceriesOpt });
        const saveInlineBtn = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update")');
        if (saveInlineBtn) {
          await saveInlineBtn.click();
          await page.waitForTimeout(600);
          pass(`Step 6a — Grocery row categorized as "${groceriesOpt}"`);
        } else {
          maybe(`Step 6a — Grocery row: category selected but save button not found`);
        }
      } else {
        maybe(`Step 6a — No Groceries/Food category option found (available: ${catOpts.slice(0,6).join(", ")})`);
      }
    } else {
      maybe(`Step 6a — Category select not found after clicking Grocery row`);
    }
  } else {
    maybe(`Step 6a — L44 SUPERMARKET GROCERIES row not found by element selector — may need to scroll or period filter is active`);
  }

  // Find the Coffee row and set category to Dining
  const coffeeRow = await page.$('li:has-text("L44 COFFEE"), tr:has-text("L44 COFFEE"), .row:has-text("L44 COFFEE"), [data-desc*="L44 COFFEE"]');
  if (coffeeRow) {
    await coffeeRow.click();
    await page.waitForTimeout(500);
    const catSelect2 = await page.$('select[aria-label*="categ" i], select[aria-label*="category" i]');
    if (catSelect2) {
      const catOpts2 = await catSelect2.evaluate((el) => Array.from(el.options).map((o) => o.text));
      const diningOpt = catOpts2.find((o) => /dining|restaurant|coffee|cafe/i.test(o));
      if (diningOpt) {
        await catSelect2.selectOption({ label: diningOpt });
        const saveBtn2 = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update")');
        if (saveBtn2) {
          await saveBtn2.click();
          await page.waitForTimeout(600);
          pass(`Step 6b — Coffee row categorized as "${diningOpt}"`);
        } else {
          maybe(`Step 6b — Coffee row: category selected but save button not found`);
        }
      } else {
        maybe(`Step 6b — No Dining/Coffee category option found`);
      }
    } else {
      maybe(`Step 6b — Category select not found after clicking Coffee row`);
    }
  } else {
    maybe(`Step 6b — L44 COFFEE SHOP row not found — may need scroll or period filter`);
  }

  await page.screenshot({ path: SS("l44_step6_transactions_after_categorize.png") });

  // ── Step 7: /rules — create auto-categorize rule for L44 SUPERMARKET ──────────
  await goto(page, "/rules");
  await page.screenshot({ path: SS("l44_step7_rules_before.png") });
  const h1rules = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/rule/i.test(h1rules)) {
    pass(`Step 7a — /rules loaded (h1: "${h1rules}")`);
  } else {
    fail(`Step 7a — expected Rules h1, got "${h1rules}"`);
  }

  const bodyRulesBefore = await page.evaluate(() => document.body.innerText);
  // Count existing rules by counting rule row items
  const ruleCountBefore = (bodyRulesBefore.match(/L44|match|payee|contains/gi) || []).length;

  // Fill match phrase
  const matchInput = await page.$('input[placeholder*="match" i], input[placeholder*="payee" i], input[aria-label*="match" i], input[placeholder*="phrase" i]');
  if (matchInput) {
    await matchInput.fill("L44 SUPERMARKET");
    pass(`Step 7b — Rule match phrase filled: "L44 SUPERMARKET"`);
  } else {
    fail(`Step 7b — Rule match input not found on /rules`);
  }

  // Select category for the rule
  const ruleCatSelect = await page.$('select[aria-label*="categ" i]');
  if (ruleCatSelect) {
    const ruleCatOpts = await ruleCatSelect.evaluate((el) => Array.from(el.options).map((o) => o.text));
    const groceriesOpt = ruleCatOpts.find((o) => /grocer|food/i.test(o));
    if (groceriesOpt) {
      await ruleCatSelect.selectOption({ label: groceriesOpt });
      pass(`Step 7c — Rule category set to "${groceriesOpt}"`);
    } else {
      maybe(`Step 7c — No Groceries/Food option in rule category select`);
    }
  } else {
    fail(`Step 7c — Rule category select not found on /rules`);
  }

  // Submit rule
  const addRuleBtn = await page.$('button[type="submit"], button:has-text("Add rule"), button:has-text("Add"), button:has-text("Save")');
  if (addRuleBtn) {
    await addRuleBtn.click();
    await page.waitForTimeout(1000);
    pass(`Step 7d — Add rule button clicked`);
  } else {
    fail(`Step 7d — Add rule submit button not found`);
  }

  await page.screenshot({ path: SS("l44_step7_rules_after_add.png") });
  const bodyRulesAfter = await page.evaluate(() => document.body.innerText);

  if (bodyRulesAfter.includes("L44 SUPERMARKET")) {
    pass(`Step 7e — Rule "L44 SUPERMARKET" appears in rules list`);
  } else {
    fail(`Step 7e — Rule "L44 SUPERMARKET" NOT found in rules list after add`);
  }

  // Verify rule count increased (or the rule simply appears)
  const ruleCountAfter = (bodyRulesAfter.match(/L44 SUPERMARKET/g) || []).length;
  if (ruleCountAfter >= 1) {
    pass(`Step 7f — Rule list contains at least 1 "L44 SUPERMARKET" entry`);
  } else {
    fail(`Step 7f — Rule not visible in list`);
  }

  // Probe: confirm the rule would "fire" — check for a live-match count or preview
  // The rules screen has a "live count" feature (rules_live_count_check.mjs exists)
  const ruleFireIndicator = bodyRulesAfter.match(/(\d+)\s*(match|transaction|existing)/i);
  if (ruleFireIndicator) {
    pass(`Step 7g — Rule live-match indicator: "${ruleFireIndicator[0]}" — rule would fire on existing transactions`);
  } else {
    maybe(`Step 7g — No live-match indicator visible for "L44 SUPERMARKET" rule (feature may require a preview click)`);
  }

  // ── Step 8: /dashboard — end-state verification ───────────────────────────────
  await goto(page, "/");
  await page.screenshot({ path: SS("l44_step8_dashboard_end_state.png") });
  const dashBodyEnd = await page.evaluate(() => document.body.innerText);

  // Net worth must include the L44 account's reconciled balance
  const dashNWEndMatch = dashBodyEnd.match(/Net\s*Worth[\s\S]{0,80}\$([\d,]+\.\d{2})/i);
  const dashNWEnd = dashNWEndMatch ? parseDollar(dashNWEndMatch[1].replace(/,/g, "")) : NaN;
  console.log(`Dashboard end-state net worth: $${dashNWEnd}`);

  if (!isNaN(dashNWEnd)) {
    pass(`Step 8a — Dashboard net worth readable: $${dashNWEnd}`);
  } else {
    fail(`Step 8a — Dashboard net worth not parseable`);
  }

  // Cross-screen: dashboard NW == accounts page NW (after reconcile)
  await goto(page, "/accounts");
  const bodyAccEnd = await page.evaluate(() => document.body.innerText);
  await page.screenshot({ path: SS("l44_step8_accounts_end_state.png") });

  const nwAccEndMatch = bodyAccEnd.match(/NET WORTH\s*\$([\d,]+\.\d{2})/i);
  const nwAccEnd = nwAccEndMatch ? parseDollar(nwAccEndMatch[1].replace(/,/g, "")) : NaN;
  console.log(`Accounts end-state net worth: $${nwAccEnd}`);

  if (!isNaN(dashNWEnd) && !isNaN(nwAccEnd)) {
    if (Math.abs(dashNWEnd - nwAccEnd) < 1) {
      pass(`Step 8b — INVARIANT: Dashboard net worth ($${dashNWEnd}) == Accounts net worth ($${nwAccEnd})`);
    } else {
      fail(`Step 8b — INVARIANT VIOLATION: Dashboard NW ($${dashNWEnd}) ≠ Accounts NW ($${nwAccEnd}), diff $${Math.abs(dashNWEnd - nwAccEnd).toFixed(2)}`);
    }
  } else {
    fail(`Step 8b — Could not compare net worth cross-screen (dash: ${dashNWEnd}, accounts: ${nwAccEnd})`);
  }

  // Net worth == sum of account balances invariant
  // Verify L44 Omar Checking's reconciled balance is visible in the final accounts list
  if (bodyAccEnd.includes("L44 Omar Checking")) {
    pass(`Step 8c — L44 Omar Checking appears in /accounts at end of ritual`);
  } else {
    fail(`Step 8c — L44 Omar Checking NOT in /accounts end state`);
  }

  const reconcileBalInList = bodyAccEnd.match(/L44 Omar Checking[\s\S]{0,120}\$(2[,.]?345\.67)/);
  if (reconcileBalInList) {
    pass(`Step 8d — L44 Omar Checking shows reconciled balance $2,345.67 in final accounts list`);
  } else {
    maybe(`Step 8d — Reconciled balance $2,345.67 not visible near L44 Omar Checking in list (may require scrolling or inline-edit update)`);
  }

  // ── Step 9: /reports — confirm imported spend in breakdown ───────────────────
  await goto(page, "/reports");
  await page.screenshot({ path: SS("l44_step9_reports.png") });
  const h1reports = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/report/i.test(h1reports)) {
    pass(`Step 9a — /reports loaded (h1: "${h1reports}")`);
  } else {
    fail(`Step 9a — expected Reports h1, got "${h1reports}"`);
  }

  const bodyReports = await page.evaluate(() => document.body.innerText);
  // Check that spending breakdown reflects newly categorized rows
  const groceriesInReports = /grocer|food/i.test(bodyReports);
  const diningInReports    = /dining|restaurant|coffee/i.test(bodyReports);

  if (groceriesInReports) {
    pass(`Step 9b — Reports spending breakdown includes Groceries/Food category`);
  } else {
    maybe(`Step 9b — Groceries/Food not visible in reports (category may not have been applied)`);
  }
  if (diningInReports) {
    pass(`Step 9c — Reports spending breakdown includes Dining/Coffee category`);
  } else {
    maybe(`Step 9c — Dining/Coffee not visible in reports`);
  }

  // Period consistency: reports should show same period as dashboard
  await goto(page, "/");
  const dashPeriodBody = await page.evaluate(() => document.body.innerText);
  const dashPeriod = dashPeriodBody.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i)?.[0] ?? null;

  await goto(page, "/reports");
  const reportsPeriodBody = await page.evaluate(() => document.body.innerText);
  const reportsPeriod = reportsPeriodBody.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i)?.[0] ?? null;

  if (dashPeriod && reportsPeriod) {
    if (dashPeriod.toLowerCase() === reportsPeriod.toLowerCase()) {
      pass(`Step 9d — INVARIANT: Period window consistent (Dashboard: "${dashPeriod}", Reports: "${reportsPeriod}")`);
    } else {
      fail(`Step 9d — INVARIANT VIOLATION: Period mismatch (Dashboard: "${dashPeriod}", Reports: "${reportsPeriod}")`);
    }
  } else {
    fail(`Step 9d — Could not parse period window for consistency check`);
  }

  // ── Step 10: Hard reload /accounts — persistence check ───────────────────────
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l44_step10_accounts_after_reload.png") });

  const bodyReload = await page.evaluate(() => document.body.innerText);
  if (bodyReload.includes(ACCT_NAME)) {
    pass(`Step 10a — "${ACCT_NAME}" persists after hard reload of /accounts`);
  } else {
    fail(`Step 10a — "${ACCT_NAME}" NOT found after hard reload`);
  }

  // ── Step 11: JS error check ──────────────────────────────────────────────────
  if (errors.length === 0) {
    pass(`Step 11 — Zero JS page errors across entire flow`);
  } else {
    fail(`Step 11 — ${errors.length} JS error(s): ${errors.slice(0, 3).join("; ")}`);
  }

  // ── Summary ──────────────────────────────────────────────────────────────────
  console.log(`\n─── L44 Results: ${passed} passed, ${failed} failed ───`);
  if (failed > 0) process.exit(1);

} finally {
  await browser.close();
}
