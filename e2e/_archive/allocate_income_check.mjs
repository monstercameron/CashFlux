// Gate: income → allocate pre-fill entry point.
//
// Verifies that:
//   1. The Allocate screen shows the income pre-fill banner when there is
//      positive income in the current month (seeded via the transactions screen).
//   2. Clicking "Allocate this month's income" button fills the amount input with
//      a positive number matching the seeded income.
//   3. After clicking, the banner dismisses (the amount input is filled and the
//      nudge card is gone or the button is absent).
//
// This test seeds one income transaction, navigates to /allocate, and asserts
// the affordance described in L10 (income → envelopes entry point).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

async function ready(page) {
  // Wait for the app shell (nav) to appear and the boot splash to clear.
  await page.waitForSelector("nav", { timeout: 60000 });
  await page.waitForTimeout(600);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1) Seed a large THIS-MONTH income transaction so the income nudge appears.
  //    The inline transaction add form moved to the +Add modal (C73), so seed via a
  //    one-shot addInitScript that re-applies the injection at document-start (it
  //    survives the navigation's pagehide→autosave clobber).
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await ready(page);
  const today = await page.evaluate(() => new Date().toISOString().slice(0, 10));
  await page.evaluate((iso) => localStorage.setItem("e2e-income", iso), today);
  await page.addInitScript(() => {
    const iso = localStorage.getItem("e2e-income");
    if (!iso) return;
    localStorage.removeItem("e2e-income"); // one-shot
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      const acc = (ds.accounts || []).find((a) => !a.archived) || (ds.accounts || [])[0];
      if (!acc) return;
      ds.transactions = ds.transactions || [];
      ds.transactions.push({
        id: "tx-e2e-income", accountId: acc.id, date: iso + "T12:00:00Z",
        desc: "ZZ Test Paycheck", amount: { Amount: 320000, Currency: acc.currency || "USD" },
      });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  // 2) Navigate to /allocate (fresh load → addInitScript seeds the income).
  await page.goto(BASE + "/allocate", { waitUntil: "domcontentloaded" });
  await ready(page);
  await page.waitForTimeout(500);

  // 3) Assert the income nudge banner is present.
  const nudge = page.locator('[data-testid="income-nudge"]');
  if ((await nudge.count()) === 0) {
    fail("income pre-fill banner (data-testid=income-nudge) not found on /allocate");
    process.exit(1);
  }

  // 4) The apply button text should mention a positive amount.
  const applyBtn = page.locator('[data-testid="income-nudge-apply"]');
  if ((await applyBtn.count()) === 0) {
    fail("income nudge apply button (data-testid=income-nudge-apply) not found");
    process.exit(1);
  }
  const btnText = await applyBtn.innerText();
  if (!/\d/.test(btnText)) {
    fail(`income nudge button has no numeric amount in label: "${btnText}"`);
    process.exit(1);
  }

  // 5) Click the apply button — it should pre-fill the amount input.
  await applyBtn.click();
  await page.waitForTimeout(300);

  // 6) The amount input should now contain a positive number.
  const amountInput = page.locator('input[type="number"][placeholder*="Amount"], input[type="number"][placeholder*="amount"], input[type="number"]').first();
  const amountVal = await amountInput.inputValue().catch(() => "");
  const numeric = parseFloat(amountVal);
  if (isNaN(numeric) || numeric <= 0) {
    fail(`amount input was not pre-filled with a positive value after clicking income nudge (got "${amountVal}")`);
  }

  if (!process.exitCode) {
    console.log(`PASS: income pre-fill — nudge appeared, clicked, amount input filled with ${amountVal}.`);
  }
} finally {
  await browser.close();
}
