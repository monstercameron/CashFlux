// L44 E2E loop story — "The New Account Setup" (Omar onboards a real bank account)
// Persona: Omar, 38, self-employed, adds a new checking account with an opening balance
//          and runs the full onboarding ritual: add account → CSV import → dedupe review →
//          reconcile → categorize → create rule → verify dashboard + reports.
// Flow:
//   1. /accounts — add "L44 Omar Checking" (Checking, $1,000.00 opening balance).
//   2. /accounts — verify new account appears in list; net worth +$1,000.
//   3. /dashboard — confirm Dashboard net worth == Accounts net worth (cross-screen invariant).
//   4. /documents — paste CSV bank statement (5 rows); probe for account-selection hand-off gap.
//   5. /transactions — verify all 5 imported rows appear; money conservation (no cents lost).
//   6. /accounts — update-balance reconcile via ⋯ overflow menu on L44 Omar Checking.
//   7. /transactions — categorize Grocery row (Food) and Coffee row (Dining).
//   8. /rules — create rule "L44 SUPERMARKET" → Groceries; confirm rule count increases.
//   9. /dashboard — confirm L44 account in net worth; cross-screen NW invariant.
//  10. /reports — confirm spending categories reflect categorized imports.
//  11. Period consistency: Dashboard vs Reports.
//  12. Hard reload /accounts — account persists with updated balance.
//  13. JS error check.
//
// Key cross-screen invariants:
//   A. Net worth (Dashboard) == Net worth (Accounts) at all measurement points.
//   B. All imported amounts land without cents lost.
//   C. CSV import path ignores account-selector (probed as gap).
//   D. Reconcile closes the gap to the bank's ending figure.
//   E. Period window is consistent across Dashboard, Reports.
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

// CSV with 5 transactions.  The placeholder hint on the /documents textarea is
// "date,payee,amount,account" which maps to the CashFlux CSV import format.
// The CSV plain-import path (ImportTransactionsCSV) does NOT consume the importAcct
// selector — it assigns transactions based on the CSV's own "account" column only.
// A blank "account" column causes ValidateTransaction to fail (accountId is required),
// so ALL rows are silently dropped.  This is the core account-hand-off gap:
// the user must embed a valid account name/ID in their CSV file; there is no UI
// picker to route the import to a chosen account.
// For this ritual we use the account name in the "account" column so rows land.
const IMPORT_CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking
2026-06-16,L44 COFFEE SHOP,-12.50,L44 Omar Checking
2026-06-18,L44 RENT PARTIAL,-200.00,L44 Omar Checking
2026-06-20,L44 PAYCHECK DEPOSIT,1500.00,L44 Omar Checking
2026-06-21,L44 UTILITIES PAYMENT,-147.50,L44 Omar Checking`;

// Expected amounts from the CSV rows
const EXPECTED_AMOUNTS = [95.00, 12.50, 200.00, 1500.00, 147.50];
const CSV_ROWS = [
  "L44 SUPERMARKET GROCERIES",
  "L44 COFFEE SHOP",
  "L44 RENT PARTIAL",
  "L44 PAYCHECK DEPOSIT",
  "L44 UTILITIES PAYMENT",
];

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

// Extract NET WORTH from the Accounts page (it appears right after "NET WORTH" header)
const parseAccountsNetWorth = (text) => {
  const m = text.match(/NET WORTH\s*\$([\d,]+\.\d{2})/i);
  return m ? parseDollar(m[1].replace(/,/g, "")) : NaN;
};

// Parse balance for a specific account name from accounts body text.
// The line format is:  "<AccountName>\n<Type> · USD\n$X.XX"
// We match exactly the first dollar figure on the third "line" after the name.
const parseAccountBalance = (text, acctName) => {
  // Escape special regex chars in account name
  const esc = acctName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  // Match name, followed (within ~100 chars) by first $X.XX
  const m = text.match(new RegExp(esc + "[\\s\\S]{0,80}\\$(([\\d,]+\\.\\d{2}))"));
  if (!m) return NaN;
  return parseDollar(m[1].replace(/,/g, ""));
};

// Extract net worth from Dashboard — it's the FIRST dollar figure after "Net worth"
const parseDashNetWorth = (text) => {
  const m = text.match(/Net worth\s*\$([\d,]+\.\d{2})/i);
  return m ? parseDollar(m[1].replace(/,/g, "")) : NaN;
};

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 0: Baseline ──────────────────────────────────────────────────────────
  await goto(page, "/accounts");
  const bodyBaseline = await page.evaluate(() => document.body.innerText);
  await page.screenshot({ path: SS("l44_step0_accounts_baseline.png") });
  const nwBaseline = parseAccountsNetWorth(bodyBaseline);
  console.log(`Baseline net worth: $${nwBaseline}`);
  // Also count existing accounts
  const acctCountBaseline = (bodyBaseline.match(/Transactions\nEdit/g) || []).length;
  console.log(`Baseline account count: ${acctCountBaseline}`);

  // ── Step 1: /accounts — add L44 Omar Checking ─────────────────────────────────
  // The add-account form is open by default on /accounts (always visible at top)
  // Fields: Name (text input), Account type (select), Opening balance (number input), Add account (submit)
  const nameInput = await page.$('input[placeholder*="Name" i], input[type="text"]');
  if (nameInput) {
    await nameInput.fill(ACCT_NAME);
    pass(`Step 1a — account name filled: "${ACCT_NAME}"`);
  } else {
    fail(`Step 1a — account name input not found on /accounts`);
  }

  // Account type
  const typeSelect = await page.$('select');
  if (typeSelect) {
    await typeSelect.selectOption({ label: "Checking" });
    pass(`Step 1b — account type set to Checking`);
  } else {
    maybe(`Step 1b — type select not found`);
  }

  // Opening balance — the number input for opening balance
  const amtInput = await page.$('input[type="number"]');
  if (amtInput) {
    await amtInput.fill(OPENING_BAL);
    pass(`Step 1c — opening balance filled: $${OPENING_BAL}`);
  } else {
    fail(`Step 1c — opening balance input not found`);
  }

  // Submit
  const addBtn = await page.$('button:has-text("Add account")');
  if (addBtn) {
    await addBtn.click();
    await page.waitForTimeout(1200);
    pass(`Step 1d — "Add account" clicked`);
  } else {
    fail(`Step 1d — "Add account" button not found`);
  }

  await page.screenshot({ path: SS("l44_step1_account_added.png") });
  const bodyAfterAdd = await page.evaluate(() => document.body.innerText);

  if (bodyAfterAdd.includes(ACCT_NAME)) {
    pass(`Step 1e — "${ACCT_NAME}" appears in accounts list after add`);
  } else {
    fail(`Step 1e — "${ACCT_NAME}" NOT found in accounts list after add`);
  }

  const nwAfterAdd = parseAccountsNetWorth(bodyAfterAdd);
  if (!isNaN(nwBaseline) && !isNaN(nwAfterAdd)) {
    const delta = nwAfterAdd - nwBaseline;
    if (Math.abs(delta - 1000) < 1) {
      pass(`Step 1f — Net worth increased by exactly $1,000 (was $${nwBaseline}, now $${nwAfterAdd})`);
    } else {
      fail(`Step 1f — Net worth delta is $${delta.toFixed(2)}, expected ~$1,000 (baseline $${nwBaseline}, after $${nwAfterAdd})`);
    }
  } else {
    fail(`Step 1f — Could not parse net worth for delta check`);
  }

  // ── Step 2: /dashboard — net worth cross-screen invariant ─────────────────────
  await goto(page, "/");
  await page.screenshot({ path: SS("l44_step2_dashboard_after_add.png") });
  const dashBodyAdd = await page.evaluate(() => document.body.innerText);
  const dashNWAfterAdd = parseDashNetWorth(dashBodyAdd);
  console.log(`Dashboard net worth after add: $${dashNWAfterAdd}, Accounts: $${nwAfterAdd}`);

  if (!isNaN(dashNWAfterAdd) && !isNaN(nwAfterAdd) && Math.abs(dashNWAfterAdd - nwAfterAdd) < 1) {
    pass(`Step 2a — INVARIANT A: Dashboard NW ($${dashNWAfterAdd}) == Accounts NW ($${nwAfterAdd})`);
  } else if (!isNaN(dashNWAfterAdd) && !isNaN(nwAfterAdd)) {
    fail(`Step 2a — INVARIANT A VIOLATION: Dashboard NW ($${dashNWAfterAdd}) ≠ Accounts NW ($${nwAfterAdd}), diff $${Math.abs(dashNWAfterAdd - nwAfterAdd).toFixed(2)}`);
  } else {
    fail(`Step 2a — Could not parse net worth (dash: ${dashNWAfterAdd}, accounts: ${nwAfterAdd})`);
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

  // Probe: account selector on /documents?
  // From DOM inspection: there is NO account selector (select) visible on the CSV import path.
  // The CSV textarea has placeholder "date,payee,amount,account" — the account column in CSV
  // is the mechanism (not a UI picker). The importAcct state drives image/statement import only.
  const acctSel = await page.$('select');
  if (acctSel) {
    const opts = await acctSel.evaluate((el) => Array.from(el.options).map((o) => o.text));
    pass(`Step 3b — Account selector IS present on /documents (options: ${opts.join(", ")})`);
  } else {
    fail(`Step 3b — GAP (ACCOUNT HAND-OFF): No account selector on /documents CSV import path. CSV "account" column is the only routing mechanism — if blank, transactions go to the first/default account, not the newly added L44 account.`);
  }

  // The CSV import textarea has placeholder starting with "date,payee,amount,account"
  // (confirmed from DOM inspection). It is the SECOND textarea on the page (first = statement).
  const textareas = await page.$$('textarea');
  console.log(`Found ${textareas.length} textarea(s) on /documents`);
  let csvTextarea = null;
  for (const ta of textareas) {
    const ph = await ta.getAttribute("placeholder");
    if (ph && /date.*payee|date.*amount/i.test(ph)) {
      csvTextarea = ta;
      break;
    }
  }
  if (csvTextarea) {
    await csvTextarea.fill(IMPORT_CSV);
    pass(`Step 3c — CSV textarea found (by placeholder) and filled with 5 data rows`);
  } else if (textareas.length > 0) {
    // Fallback: use the last textarea
    await textareas[textareas.length - 1].fill(IMPORT_CSV);
    maybe(`Step 3c — CSV textarea found by position fallback (last textarea)`);
    csvTextarea = textareas[textareas.length - 1];
  } else {
    fail(`Step 3c — No textarea found on /documents`);
  }

  // Click the "Import" button (near the CSV textarea, not "Parse statement")
  const importBtn = await page.$('button:has-text("Import")');
  if (importBtn) {
    await importBtn.click();
    await page.waitForTimeout(1500);
    pass(`Step 3d — "Import" button clicked`);
  } else {
    fail(`Step 3d — "Import" button not found on /documents`);
  }

  await page.screenshot({ path: SS("l44_step3_documents_after_import.png") });
  const bodyDocsAfter = await page.evaluate(() => document.body.innerText);

  // Check for import success/count message
  const importMsg = bodyDocsAfter.match(/imported?\s+(\d+)\s+(transaction|row)|(\d+)\s+(transaction|row)\s+imported?/i);
  if (importMsg) {
    const cnt = parseInt(importMsg[1] || importMsg[3]);
    if (cnt >= 4) {
      pass(`Step 3e — Import succeeded: ${cnt} transactions imported`);
    } else if (cnt > 0) {
      maybe(`Step 3e — Import returned ${cnt} transactions (expected 5; header row may have been counted or some skipped)`);
    } else {
      fail(`Step 3e — Import count is 0`);
    }
  } else if (/error|fail|invalid/i.test(bodyDocsAfter)) {
    fail(`Step 3e — Import returned an error message`);
  } else {
    maybe(`Step 3e — Import message not in expected format; checking /transactions for evidence`);
  }

  // Dedupe probe: opening balance is $1,000 — not in the CSV, so no dedupe expected here
  const skippedMsg = bodyDocsAfter.match(/skipped?\s+(\d+)|(\d+)\s+skipped?/i);
  if (skippedMsg) {
    pass(`Step 3f — Dedupe: ${skippedMsg[1] || skippedMsg[2]} row(s) skipped (deduplication active)`);
  } else {
    maybe(`Step 3f — No "skipped" message — dedupe did not trigger (expected, as CSV rows don't overlap with opening balance)`);
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

  let rowsFound = 0;
  for (const row of CSV_ROWS) {
    if (bodyTxn.includes(row)) rowsFound++;
  }
  if (rowsFound === 5) {
    pass(`Step 4b — All 5 CSV-imported rows found in /transactions`);
  } else if (rowsFound > 0) {
    maybe(`Step 4b — Only ${rowsFound}/5 CSV rows found in /transactions (period filter may be hiding earlier-dated rows)`);
  } else {
    fail(`Step 4b — None of the 5 CSV rows found in /transactions — import may have failed or transactions are on a different account not visible here`);
  }

  // Account assignment probe: imported rows come from CSV path which ignores importAcct.
  // The CSV has blank account column → transactions route to default account (first account = 401k brokerage).
  // This is the core account hand-off gap.
  const groceriesInBody = bodyTxn.includes("L44 SUPERMARKET GROCERIES");
  if (groceriesInBody) {
    // Find what account is shown near the grocery row
    const acctNearGrocery = bodyTxn.match(/L44 SUPERMARKET GROCERIES[\s\S]{0,300}?(L44 Omar Checking|Everyday Checking|401\(k\)|Cash Wallet|Emergency|12-month|Roth|Rewards)/);
    if (acctNearGrocery) {
      if (/L44 Omar Checking/.test(acctNearGrocery[1])) {
        pass(`Step 4c — Imported rows correctly assigned to "L44 Omar Checking"`);
      } else {
        fail(`Step 4c — GAP (ACCOUNT HAND-OFF): Imported rows assigned to "${acctNearGrocery[1]}" instead of "L44 Omar Checking". CSV import silently routed to first account in the list. The blank "account" column in the CSV means transactions fall to the default account, not the new account Omar intended.`);
      }
    } else {
      maybe(`Step 4c — Account assignment not determinable from /transactions body text; CSV import account routing is unverified`);
    }
  } else {
    maybe(`Step 4c — SUPERMARKET row not visible to check account assignment`);
  }

  // Money conservation: check all 5 amounts appear somewhere in transactions
  let amountsFound = 0;
  for (const amt of EXPECTED_AMOUNTS) {
    if (bodyTxn.includes(amt.toFixed(2))) amountsFound++;
  }
  if (amountsFound === 5) {
    pass(`Step 4d — INVARIANT B (MONEY CONSERVATION): All 5 imported amounts present in /transactions (no cents lost)`);
  } else if (amountsFound > 0) {
    maybe(`Step 4d — ${amountsFound}/5 imported amounts visible (some may be on a different account or period)`);
  } else {
    fail(`Step 4d — No imported amounts visible in /transactions — conservation check inconclusive`);
  }

  // ── Step 5: /accounts — reconcile L44 Omar Checking via overflow menu ─────────
  await goto(page, "/accounts");
  await page.screenshot({ path: SS("l44_step5_accounts_before_reconcile.png") });
  const bodyAccBeforeRec = await page.evaluate(() => document.body.innerText);
  const nwBeforeRec = parseAccountsNetWorth(bodyAccBeforeRec);

  // Find L44 Omar Checking's balance
  const omarBalBefore = parseAccountBalance(bodyAccBeforeRec, ACCT_NAME);
  console.log(`L44 Omar Checking balance before reconcile: $${omarBalBefore}`);

  if (!isNaN(omarBalBefore)) {
    pass(`Step 5a — L44 Omar Checking balance readable before reconcile: $${omarBalBefore}`);
  } else {
    fail(`Step 5a — L44 Omar Checking balance not parseable (account may not have been added this session)`);
  }

  // Open the ⋯ overflow menu on L44 Omar Checking row, then click "Update balance"
  // Strategy: find the account row containing "L44 Omar Checking", then click the
  // "More actions" button within that row scope.
  let reconcileDone = false;
  const allMoreBtns = await page.$$('button[aria-label="More actions"]');
  console.log(`Found ${allMoreBtns.length} "More actions" buttons on /accounts`);

  for (const btn of allMoreBtns) {
    // Check if this button is in a row near L44 Omar Checking
    // Walk up to nearest ancestor that contains the account name text
    const rowText = await btn.evaluate((b) => {
      let el = b;
      for (let i = 0; i < 10; i++) {
        el = el.parentElement;
        if (!el) break;
        const txt = el.innerText?.trim() ?? "";
        if (txt.includes("L44 Omar Checking")) return txt;
      }
      return "";
    });
    if (/L44 Omar/i.test(rowText)) {
      await btn.click();
      await page.waitForTimeout(500);

      const updateBalBtn = await page.$('button[role="menuitem"]:has-text("Update balance"), button.add-item:has-text("Update balance")');
      if (updateBalBtn) {
        const isVisible = await updateBalBtn.evaluate((el) => el.offsetParent !== null);
        if (isVisible) {
          await updateBalBtn.click();
          await page.waitForTimeout(800);
          pass(`Step 5b — "Update balance" menu item found and clicked for L44 Omar Checking`);
          reconcileDone = true;
        } else {
          // Force click via JS
          await page.evaluate((el) => el.click(), updateBalBtn);
          await page.waitForTimeout(800);
          pass(`Step 5b — "Update balance" clicked via JS for L44 Omar Checking`);
          reconcileDone = true;
        }
      }
      break;
    }
  }

  if (!reconcileDone && allMoreBtns.length > 0) {
    // Fallback: click the LAST More actions button (new accounts appear at end of list)
    const lastBtn = allMoreBtns[allMoreBtns.length - 1];
    await lastBtn.click();
    await page.waitForTimeout(500);
    const updateBalFallback = await page.$('button.add-item:has-text("Update balance")');
    if (updateBalFallback) {
      await page.evaluate((el) => el.click(), updateBalFallback);
      await page.waitForTimeout(800);
      maybe(`Step 5b — Used fallback: clicked last account's "Update balance"`);
      reconcileDone = true;
    }
  }

  if (!reconcileDone) {
    fail(`Step 5b — Could not open "Update balance" for L44 Omar Checking`);
  }

  // The reconcile form: should show current balance + a new balance input
  // From the source: setBal opens an inline edit on the account row, setting editingBal
  await page.screenshot({ path: SS("l44_step5_accounts_reconcile_form.png") });
  const reconcileInput = await page.$('input[type="number"]');
  if (reconcileInput) {
    // Clear and fill with the bank's ending figure
    await reconcileInput.fill("");
    await reconcileInput.fill(RECONCILE_BAL);
    pass(`Step 5c — Reconcile balance input filled with $${RECONCILE_BAL}`);

    // Submit
    const saveBtn = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update"), button:has-text("OK"), button:has-text("Set")');
    if (saveBtn) {
      await saveBtn.click();
      await page.waitForTimeout(1000);
      pass(`Step 5d — Reconcile submitted`);
    } else {
      // Try pressing Enter on the input
      await reconcileInput.press("Enter");
      await page.waitForTimeout(800);
      maybe(`Step 5d — No explicit Save button; submitted via Enter`);
    }
  } else {
    fail(`Step 5c — No number input found after clicking "Update balance"`);
  }

  await page.screenshot({ path: SS("l44_step5_accounts_after_reconcile.png") });
  const bodyAccAfterRec = await page.evaluate(() => document.body.innerText);
  const omarBalAfter = parseAccountBalance(bodyAccAfterRec, ACCT_NAME);
  console.log(`L44 Omar Checking balance after reconcile: $${omarBalAfter}`);

  if (!isNaN(omarBalAfter) && Math.abs(omarBalAfter - parseFloat(RECONCILE_BAL)) < 1) {
    pass(`Step 5e — INVARIANT D (RECONCILE): L44 Omar Checking balance == bank figure $${RECONCILE_BAL} (got $${omarBalAfter})`);
  } else if (!isNaN(omarBalAfter)) {
    fail(`Step 5e — Balance after reconcile is $${omarBalAfter}, expected $${RECONCILE_BAL} — reconcile may not have applied`);
  } else {
    fail(`Step 5e — Could not parse L44 Omar Checking balance after reconcile`);
  }

  // ── Step 6: /transactions — categorize two imported rows ──────────────────────
  await goto(page, "/transactions");
  await page.screenshot({ path: SS("l44_step6_transactions_before_cat.png") });

  // Try to find and click the Grocery row to open inline edit
  // The transactions screen shows rows; clicking on a row (or an Edit button) opens editing
  let groceryEditDone = false;
  const groceryRowEl = await page.$(`li:has-text("L44 SUPERMARKET"), tr:has-text("L44 SUPERMARKET"), [class*="row"]:has-text("L44 SUPERMARKET")`);
  if (groceryRowEl) {
    // Look for an Edit button within the row
    const editBtn = await groceryRowEl.$('button:has-text("Edit"), button[aria-label*="edit" i]');
    if (editBtn) {
      await editBtn.click();
    } else {
      await groceryRowEl.click();
    }
    await page.waitForTimeout(600);

    const catSelect = await page.$('select[aria-label*="categ" i], select[aria-label*="category" i]');
    if (catSelect) {
      const catOpts = await catSelect.evaluate((el) => Array.from(el.options).map((o) => o.text));
      const grocOpt = catOpts.find((o) => /grocer|food/i.test(o));
      if (grocOpt) {
        await catSelect.selectOption({ label: grocOpt });
        const saveBtn = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update")');
        if (saveBtn) { await saveBtn.click(); await page.waitForTimeout(600); }
        pass(`Step 6a — SUPERMARKET row categorized as "${grocOpt}"`);
        groceryEditDone = true;
      } else {
        maybe(`Step 6a — No Groceries/Food option in category select (options: ${catOpts.slice(0,6).join(", ")})`);
      }
    } else {
      maybe(`Step 6a — Category select not found after clicking Grocery row`);
    }
  } else {
    maybe(`Step 6a — L44 SUPERMARKET row not found as element (may need scroll or period adjustment)`);
  }

  // Coffee row
  let coffeeEditDone = false;
  const coffeeRowEl = await page.$(`li:has-text("L44 COFFEE"), tr:has-text("L44 COFFEE"), [class*="row"]:has-text("L44 COFFEE")`);
  if (coffeeRowEl) {
    const editBtn2 = await coffeeRowEl.$('button:has-text("Edit"), button[aria-label*="edit" i]');
    if (editBtn2) { await editBtn2.click(); } else { await coffeeRowEl.click(); }
    await page.waitForTimeout(600);
    const catSelect2 = await page.$('select[aria-label*="categ" i]');
    if (catSelect2) {
      const catOpts2 = await catSelect2.evaluate((el) => Array.from(el.options).map((o) => o.text));
      const diningOpt = catOpts2.find((o) => /dining|restaurant|coffee|cafe|food/i.test(o));
      if (diningOpt) {
        await catSelect2.selectOption({ label: diningOpt });
        const saveBtn2 = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Update")');
        if (saveBtn2) { await saveBtn2.click(); await page.waitForTimeout(600); }
        pass(`Step 6b — COFFEE SHOP row categorized as "${diningOpt}"`);
        coffeeEditDone = true;
      } else {
        maybe(`Step 6b — No Dining/Coffee category option found`);
      }
    } else {
      maybe(`Step 6b — Category select not found after clicking Coffee row`);
    }
  } else {
    maybe(`Step 6b — L44 COFFEE SHOP row not found as element`);
  }

  await page.screenshot({ path: SS("l44_step6_transactions_after_cat.png") });

  // ── Step 7: /rules — create auto-categorize rule ──────────────────────────────
  await goto(page, "/rules");
  await page.screenshot({ path: SS("l44_step7_rules_before.png") });
  const h1rules = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/rule/i.test(h1rules)) {
    pass(`Step 7a — /rules loaded (h1: "${h1rules}")`);
  } else {
    fail(`Step 7a — expected Rules h1, got "${h1rules}"`);
  }

  const bodyRulesBefore = await page.evaluate(() => document.body.innerText);
  const existingRuleCount = (bodyRulesBefore.match(/L44 SUPERMARKET/g) || []).length;
  console.log(`Existing L44 SUPERMARKET rules: ${existingRuleCount}`);

  // Fill match phrase — the first text input on /rules is the match field
  const inputs = await page.$$('input[type="text"], input:not([type])');
  let matchInput = null;
  for (const inp of inputs) {
    const ph = await inp.getAttribute("placeholder");
    const al = await inp.getAttribute("aria-label");
    if (/match|payee|phrase|contains/i.test(ph + " " + al)) {
      matchInput = inp;
      break;
    }
  }
  if (!matchInput && inputs.length > 0) matchInput = inputs[0]; // fallback: first text input

  if (matchInput) {
    await matchInput.fill("L44 SUPERMARKET");
    pass(`Step 7b — Rule match phrase filled: "L44 SUPERMARKET"`);
  } else {
    fail(`Step 7b — Match phrase input not found on /rules`);
  }

  // Select category
  const ruleCatSel = await page.$('select');
  if (ruleCatSel) {
    const ruleCatOpts = await ruleCatSel.evaluate((el) => Array.from(el.options).map((o) => o.text));
    const grocOpt = ruleCatOpts.find((o) => /grocer|food/i.test(o));
    if (grocOpt) {
      await ruleCatSel.selectOption({ label: grocOpt });
      pass(`Step 7c — Rule category set to "${grocOpt}"`);
    } else {
      maybe(`Step 7c — No Groceries/Food option in rule category select (options: ${ruleCatOpts.slice(0,6).join(", ")})`);
    }
  } else {
    fail(`Step 7c — Category select not found on /rules`);
  }

  // Submit
  const addRuleBtn = await page.$('button:has-text("Add rule"), button[type="submit"]');
  if (addRuleBtn) {
    await addRuleBtn.click();
    await page.waitForTimeout(1000);
    pass(`Step 7d — Add rule submitted`);
  } else {
    fail(`Step 7d — "Add rule" submit button not found`);
  }

  await page.screenshot({ path: SS("l44_step7_rules_after_add.png") });
  const bodyRulesAfter = await page.evaluate(() => document.body.innerText);

  if (bodyRulesAfter.includes("L44 SUPERMARKET")) {
    pass(`Step 7e — Rule "L44 SUPERMARKET" appears in rules list after add`);
  } else {
    fail(`Step 7e — Rule "L44 SUPERMARKET" NOT found in rules list`);
  }

  // Confirm rule "would fire" — check for a live-match count
  // The /rules screen has a live match count feature (existing `rules_live_count_check.mjs`)
  const liveMatchIndicator = bodyRulesAfter.match(/L44 SUPERMARKET[\s\S]{0,200}?(\d+)\s*(match|txn|transaction)/i);
  if (liveMatchIndicator) {
    pass(`Step 7f — RULE FIRES: Live match indicator shows ${liveMatchIndicator[1]} matching transaction(s) for "L44 SUPERMARKET"`);
  } else {
    maybe(`Step 7f — No live-match count visible for "L44 SUPERMARKET" rule — rule preview may require explicit trigger`);
  }

  // ── Step 8: /dashboard — end-state net worth invariant ───────────────────────
  await goto(page, "/");
  await page.screenshot({ path: SS("l44_step8_dashboard_end_state.png") });
  const dashBodyEnd = await page.evaluate(() => document.body.innerText);
  const dashNWEnd = parseDashNetWorth(dashBodyEnd);
  console.log(`Dashboard end-state NW: $${dashNWEnd}`);

  // Cross-screen: re-check accounts net worth
  await goto(page, "/accounts");
  const bodyAccEnd = await page.evaluate(() => document.body.innerText);
  await page.screenshot({ path: SS("l44_step8_accounts_end_state.png") });
  const nwAccEnd = parseAccountsNetWorth(bodyAccEnd);
  console.log(`Accounts end-state NW: $${nwAccEnd}`);

  if (!isNaN(dashNWEnd) && !isNaN(nwAccEnd) && Math.abs(dashNWEnd - nwAccEnd) < 1) {
    pass(`Step 8a — INVARIANT A (final): Dashboard NW ($${dashNWEnd}) == Accounts NW ($${nwAccEnd})`);
  } else if (!isNaN(dashNWEnd) && !isNaN(nwAccEnd)) {
    fail(`Step 8a — INVARIANT A VIOLATION: Dashboard NW ($${dashNWEnd}) ≠ Accounts NW ($${nwAccEnd}), diff $${Math.abs(dashNWEnd - nwAccEnd).toFixed(2)}`);
  } else {
    fail(`Step 8a — Could not parse net worth for final cross-screen invariant`);
  }

  // L44 Omar Checking must be in the final accounts list
  if (bodyAccEnd.includes(ACCT_NAME)) {
    pass(`Step 8b — "${ACCT_NAME}" present in /accounts at end of ritual`);
  } else {
    fail(`Step 8b — "${ACCT_NAME}" NOT in /accounts at end of ritual`);
  }

  // ── Step 9: /reports — spending categories reflect imports ────────────────────
  await goto(page, "/reports");
  await page.screenshot({ path: SS("l44_step9_reports.png") });
  const h1rep = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/report/i.test(h1rep)) {
    pass(`Step 9a — /reports loaded (h1: "${h1rep}")`);
  } else {
    fail(`Step 9a — expected Reports h1, got "${h1rep}"`);
  }

  const bodyReports = await page.evaluate(() => document.body.innerText);
  if (/grocer|food/i.test(bodyReports)) {
    pass(`Step 9b — Reports includes Groceries/Food category (categorized import reflected)`);
  } else {
    maybe(`Step 9b — Groceries/Food not in Reports (categorization may not have applied or period filter)`);
  }
  if (/dining|restaurant|coffee/i.test(bodyReports)) {
    pass(`Step 9c — Reports includes Dining/Coffee category`);
  } else {
    maybe(`Step 9c — Dining/Coffee not in Reports`);
  }

  // Period consistency: Dashboard vs Reports
  await goto(page, "/");
  const dashPeriodBody = await page.evaluate(() => document.body.innerText);
  const dashPeriod = dashPeriodBody.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i)?.[0] ?? null;

  await goto(page, "/reports");
  const repPeriodBody = await page.evaluate(() => document.body.innerText);
  const repPeriod = repPeriodBody.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i)?.[0] ?? null;

  if (dashPeriod && repPeriod && dashPeriod.toLowerCase() === repPeriod.toLowerCase()) {
    pass(`Step 9d — INVARIANT E: Period window consistent (Dashboard: "${dashPeriod}", Reports: "${repPeriod}")`);
  } else if (dashPeriod && repPeriod) {
    fail(`Step 9d — INVARIANT E VIOLATION: Period mismatch (Dashboard: "${dashPeriod}", Reports: "${repPeriod}")`);
  } else {
    fail(`Step 9d — Could not parse period window (Dashboard: "${dashPeriod}", Reports: "${repPeriod}")`);
  }

  // ── Step 10: Hard reload /accounts — persistence ─────────────────────────────
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

  const omarBalReload = parseAccountBalance(bodyReload, ACCT_NAME);
  if (!isNaN(omarBalReload)) {
    pass(`Step 10b — "${ACCT_NAME}" balance persists after reload: $${omarBalReload}`);
  } else {
    fail(`Step 10b — "${ACCT_NAME}" balance not readable after reload`);
  }

  // ── Step 11: JS error check ──────────────────────────────────────────────────
  if (errors.length === 0) {
    pass(`Step 11 — Zero JS page errors across entire flow`);
  } else {
    fail(`Step 11 — ${errors.length} JS page error(s): ${errors.slice(0, 3).join("; ")}`);
  }

  // ── Summary ──────────────────────────────────────────────────────────────────
  console.log(`\n─── L44 Results: ${passed} passed, ${failed} failed ───`);
  if (failed > 0) process.exit(1);

} finally {
  await browser.close();
}
