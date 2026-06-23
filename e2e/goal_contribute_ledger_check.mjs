// L43/L59 E2E — "contribute with ledger posting debits the linked account".
// Creates an account, creates a goal linked to that account, contributes with
// the "also move money" checkbox ticked, then verifies:
//   1. Goal CurrentAmount increased by the contributed amount.
//   2. A matching debit transaction was posted to the linked account.
//
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const ACCT_NAME = "ZZ-GOAL-LEDGER-ACCT";
const GOAL_NAME = "ZZ-GOAL-LEDGER-TEST";
const CONTRIB_AMT = "50.00";
const CONTRIB_MINOR = 5000; // 50.00 in cents

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

async function waitForDataset(page, pred, timeoutMs = 8000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    const d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return await dataset(page);
}

const railTo = (page, title) =>
  page.locator(`nav[aria-label="Main navigation"] a[title="${title}"]`).click();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1. Create the linked account via the +Add modal (the inline add form was
  //    moved into the top-bar +Add FlipPanel modal in C73/C79).
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  const acctForm = page.locator('[data-testid="account-add-form"]');
  await acctForm.waitFor({ timeout: 10000 });
  await acctForm.locator('input[type="text"]').first().fill(ACCT_NAME);
  await acctForm.locator('button[type="submit"]').click();
  const d0 = await waitForDataset(page, (d) =>
    (d.accounts || []).some((a) => a.name === ACCT_NAME)
  );
  const acct = (d0.accounts || []).find((a) => a.name === ACCT_NAME);
  if (!acct) {
    fail("linked account not created");
    process.exit(1);
  }

  // 2. Create a goal linked to that account via the +Add → Goal modal.
  await railTo(page, "Goals");
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();

  // Fill the add-goal form (name, target, linked account).
  const form = page.locator('[data-testid="goal-add-form"]');
  await form.waitFor({ timeout: 8000 });
  await form.locator('input[type="text"]').first().fill(GOAL_NAME);
  await form.locator('input[type="number"]').first().fill("200.00");
  await form.locator('select[aria-label="Linked account (optional)"]').selectOption({ label: ACCT_NAME });
  await form.locator('button[type="submit"]').click();

  const d1 = await waitForDataset(page, (d) =>
    (d.goals || []).some((g) => g.name === GOAL_NAME)
  );
  const goalBefore = (d1.goals || []).find((g) => g.name === GOAL_NAME);
  if (!goalBefore) {
    fail("goal not created");
    process.exit(1);
  }
  const txnsBefore = (d1.transactions || []).filter((t) => t.accountId === acct.id).length;

  // 3. Contribute with ledger posting. Nav away+back so the /goals list re-renders
  //    with the modal-added goal (the add event doesn't refresh the list in place).
  await railTo(page, "Dashboard");
  await page.waitForTimeout(300);
  await railTo(page, "Goals");
  await page.waitForSelector(`[data-testid="goal-row-${goalBefore.id}"]`, { timeout: 10000 });

  // Open contribute form.
  await page
    .locator(`[data-testid="goal-row-${goalBefore.id}"] button[title="Add to this goal"]`)
    .click();

  // Fill amount.
  await page.locator(`#goal-contrib-${goalBefore.id}`).fill(CONTRIB_AMT);

  // Tick the "also move money" checkbox (only appears when goal has a linked account).
  const cbId = `goal-contrib-ledger-${goalBefore.id}`;
  const cb = page.locator(`#${cbId}`);
  await cb.waitFor({ timeout: 5000 });
  await cb.check();

  // Submit.
  await page.locator('button[type="submit"]').first().click();

  // 4. Verify goal CurrentAmount increased.
  const d2 = await waitForDataset(
    page,
    (d) =>
      (d.goals || []).some(
        (g) => g.id === goalBefore.id && g.currentAmount?.Amount > goalBefore.currentAmount?.Amount
      ),
    8000
  );

  const goalAfter = (d2.goals || []).find((g) => g.id === goalBefore.id);
  if (!goalAfter) {
    fail("goal missing after contribute");
  } else {
    const diff = goalAfter.currentAmount.Amount - goalBefore.currentAmount.Amount;
    if (diff !== CONTRIB_MINOR) {
      fail(`goal currentAmount diff expected ${CONTRIB_MINOR}, got ${diff}`);
    }
  }

  // 5. Verify a debit transaction was posted to the linked account.
  const txnsAfter = (d2.transactions || []).filter((t) => t.accountId === acct.id);
  if (txnsAfter.length <= txnsBefore) {
    fail(`expected a new transaction on account ${ACCT_NAME}, none found`);
  } else {
    const debit = txnsAfter.find((t) => t.amount?.Amount === -CONTRIB_MINOR);
    if (!debit) {
      fail(
        `expected a debit of -${CONTRIB_MINOR} minor units on ${ACCT_NAME}; found amounts: ${txnsAfter.map((t) => t.amount?.Amount).join(", ")}`
      );
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) {
    console.log(
      `PASS: contributed ${CONTRIB_AMT} to goal "${GOAL_NAME}" with ledger posting; ` +
        `CurrentAmount bumped and debit transaction posted to "${ACCT_NAME}".`
    );
  }
} finally {
  await browser.close();
}
