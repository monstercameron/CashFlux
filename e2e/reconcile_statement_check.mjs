// L30 gate — reconcile to statement. Seeds sample data, opens one account's
// reconcile-statement panel, enters a statement balance equal to the current
// cleared balance plus one uncleared transaction's amount, marks that
// transaction cleared, and asserts the difference reaches 0 and the
// "Reconciled ✓" confirmation is visible.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const dataset = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function flush(page) { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(400); }

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1) Load sample data and navigate to accounts.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid], button[type="submit"]', { timeout: 60000 });

  // Load sample data if no accounts exist yet.
  const loadBtn = page.locator('button:has-text("Load sample data")');
  if ((await loadBtn.count()) > 0) {
    await loadBtn.first().click();
    await flush(page);
  }
  // The app auto-seeds sample data on an empty store, but that only lands in
  // localStorage on a flush — force one before reading the dataset.
  await flush(page);

  // 2) Find an account that has at least one uncleared transaction.
  const ds = await dataset(page);
  const accounts = ds.accounts || [];
  const transactions = ds.transactions || [];

  let targetAccId = null;
  let unclearedTxn = null;
  for (const acc of accounts) {
    if (acc.archived) continue;
    const uncleared = transactions.filter((t) => t.accountId === acc.id && !t.cleared);
    if (uncleared.length > 0) {
      targetAccId = acc.id;
      unclearedTxn = uncleared[0];
      break;
    }
  }
  if (!targetAccId || !unclearedTxn) {
    fail("could not find an account with at least one uncleared transaction in sample data");
    process.exit(1);
  }

  // 3) Open the "Reconcile to statement" menu item for that account. The button
  //    is always in the DOM but lives inside a collapsed overflow menu, so open
  //    the row's ⋯ menu first to make it clickable.
  const reconcileBtn = page.locator(`[data-testid="reconcile-start-btn-${targetAccId}"]`);
  const acc0 = accounts.find((a) => a.id === targetAccId);
  const row = page.locator(`.row:has-text("${acc0.name}")`).first();
  if ((await row.count()) === 0) { fail("could not find account row for " + acc0.name); process.exit(1); }
  if (!(await reconcileBtn.isVisible().catch(() => false))) {
    await row.locator('button[aria-haspopup="menu"]').first().click();
    await page.waitForTimeout(200);
  }
  await reconcileBtn.click();
  await page.waitForTimeout(300);

  // 4) The reconcile panel should now be visible.
  const panel = page.locator('[data-testid="reconcile-statement-mode"]');
  if ((await panel.count()) === 0) { fail("reconcile-statement-mode panel not shown"); process.exit(1); }

  // 5) Read the displayed cleared balance from the DOM (it appears as text).
  //    Compute the target statement balance = cleared + unclearedTxn.amount.Amount.
  //    The uncleared transaction amount is in minor units; we need major units for the input.
  const stmtInput = page.locator('[data-testid="reconcile-statement-input"]');
  if ((await stmtInput.count()) === 0) { fail("reconcile-statement-input not found"); process.exit(1); }

  // Derive the statement balance directly from stored data:
  // cleared balance minor = openingBalance + sum of cleared txn amounts for this account.
  const acc = accounts.find((a) => a.id === targetAccId);
  const openingMinor = (acc.openingBalance && acc.openingBalance.Amount) || 0;
  const clearedMinor = transactions
    .filter((t) => t.accountId === targetAccId && t.cleared)
    .reduce((s, t) => s + (t.amount && t.amount.Amount ? t.amount.Amount : 0), openingMinor);
  const unclearedMinor = (unclearedTxn.amount && unclearedTxn.amount.Amount) || 0;
  const stmtMinor = clearedMinor + unclearedMinor;

  // Determine decimal places from currency (default 2).
  const decimals = 2;
  const stmtMajor = (stmtMinor / Math.pow(10, decimals)).toFixed(decimals);

  await stmtInput.fill(stmtMajor);
  await page.waitForTimeout(200);

  // 6) Mark the uncleared transaction cleared via its "Mark cleared" button.
  //    The button is inside a [data-id] row.
  const txnRow = page.locator(`[data-testid="reconcile-txn-row"][data-id="${unclearedTxn.id}"]`);
  if ((await txnRow.count()) === 0) { fail("reconcile txn row not found for txn " + unclearedTxn.id); process.exit(1); }
  await txnRow.locator('[data-testid="reconcile-txn-clear-btn"]').click();
  await flush(page);

  // 7) Assert the difference element shows 0 (or "+0" / "0.00").
  const diffEl = page.locator('[data-testid="reconcile-difference"]');
  if ((await diffEl.count()) === 0) { fail("reconcile-difference element not found"); process.exit(1); }
  const diffText = await diffEl.textContent();
  if (!/\b0(\.0+)?\b/.test(diffText)) {
    fail(`expected difference to show 0, got "${diffText}"`);
  }

  // 8) Assert the "Reconciled ✓" confirmation is visible.
  const confirmed = page.locator('[data-testid="reconcile-confirmed"]');
  if ((await confirmed.count()) === 0) { fail("reconcile-confirmed badge not shown after difference reaches 0"); process.exit(1); }

  if (!process.exitCode) {
    console.log(`PASS: reconcile-to-statement — cleared balance matched statement (diff 0), "Reconciled ✓" shown.`);
  }
} finally {
  await browser.close();
}
