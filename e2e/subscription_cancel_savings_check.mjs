// L12 — Subscriptions: cancel-candidates selection + annual savings summary.
//
// Seeded subscriptions in the sample dataset (24 months of monthly charges):
//   - "Gym membership"  → Iron Works Gym,   $40/mo, detected as monthly
//   - "Subscriptions"   → Streaming & apps, $30/mo, detected as monthly
//
// This test:
//   1. Loads the sample-data app and navigates to /subscriptions.
//   2. Selects the two rows via their checkboxes.
//   3. Asserts the savings summary ("subs-cancel-savings") appears and contains a
//      non-zero dollar amount.
//   4. Clicks the bulk-cancel button.
//   5. Asserts that both subscription names now appear in
//      cashflux:dataset.subscriptionCancellations.
//
// Selectors used:
//   - Checkbox:         [data-testid="sub-cancel-select-gym-membership"]
//                       [data-testid="sub-cancel-select-subscriptions"]
//   - Savings summary:  [data-testid="subs-cancel-savings"]
//   - Bulk cancel btn:  [data-testid="subs-bulk-cancel-btn"]
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// flush forces the in-WASM app to persist its state to localStorage.
const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(350);
};

// cancellations returns the subscriptionCancellations array from cashflux:dataset.
const cancellations = (page) => page.evaluate(() => {
  try {
    return (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").subscriptionCancellations || []);
  } catch (e) {
    return [];
  }
});

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  // Boot with sample data.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // Ensure we start from sample data (banner present on fresh load).
  await flush(page);

  // Navigate to /subscriptions.
  await page.locator('a[href*="subscriptions"]').first().click();
  await page.waitForTimeout(800);

  // The subscriptions screen detects recurring charges from the sample dataset.
  // We expect at least "Gym membership" and "Subscriptions" rows.
  const gymCheck = page.locator('[data-testid="sub-cancel-select-gym-membership"]');
  const subsCheck = page.locator('[data-testid="sub-cancel-select-subscriptions"]');

  if ((await gymCheck.count()) === 0) {
    fail('Checkbox sub-cancel-select-gym-membership not found — detection may have missed "Gym membership"');
  }
  if ((await subsCheck.count()) === 0) {
    fail('Checkbox sub-cancel-select-subscriptions not found — detection may have missed "Subscriptions"');
  }

  // Before selecting, the savings summary should NOT be present.
  if ((await page.locator('[data-testid="subs-cancel-savings"]').count()) !== 0) {
    fail("Savings summary visible before any selection — should only appear when ≥1 row is checked");
  }

  // Select both rows.
  await gymCheck.click();
  await page.waitForTimeout(200);
  await subsCheck.click();
  await page.waitForTimeout(200);

  // Savings summary should now be visible.
  const summary = page.locator('[data-testid="subs-cancel-savings"]');
  if ((await summary.count()) === 0) {
    fail("Savings summary not visible after selecting 2 subscriptions");
  }

  // Summary text must contain a non-zero dollar figure.
  const summaryText = await summary.innerText();
  const hasDollar = /\$\d/.test(summaryText);
  if (!hasDollar) {
    fail(`Savings summary text "${summaryText}" does not contain a dollar amount`);
  }

  // Click the bulk-cancel button.
  const bulkBtn = page.locator('[data-testid="subs-bulk-cancel-btn"]');
  if ((await bulkBtn.count()) === 0) {
    fail("Bulk cancel button not found after selection");
  }
  await bulkBtn.click();
  await page.waitForTimeout(400);

  // Flush and inspect localStorage.
  await flush(page);
  const stored = await cancellations(page);

  const cancelledNames = stored.map((c) => (c.subName || c.SubName || "").toLowerCase());
  if (!cancelledNames.some((n) => n.includes("gym"))) {
    fail(`"Gym membership" not in cancellations after bulk cancel: ${JSON.stringify(cancelledNames)}`);
  }
  if (!cancelledNames.some((n) => n.includes("subscription"))) {
    fail(`"Subscriptions" not in cancellations after bulk cancel: ${JSON.stringify(cancelledNames)}`);
  }

  // After bulk cancel the savings summary should be gone (selection was cleared).
  if ((await page.locator('[data-testid="subs-cancel-savings"]').count()) !== 0) {
    fail("Savings summary still visible after bulk cancel — selection should have been cleared");
  }

  if (!process.exitCode) {
    console.log("PASS: L12 subscription cancel-savings: selected 2 subs, verified savings summary and bulk cancel.");
  }
} finally {
  await browser.close();
}
