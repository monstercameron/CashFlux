// L43 gate — "Top up" an under-limit budget. Navigate to /budgets, find a budget
// that is NOT over its limit, click "Top up…", enter an amount, submit, and assert
// the budget's limit rose by exactly that amount.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const budgets = (page) =>
  page.evaluate(
    () =>
      JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").budgets || []
  );
async function flush(page) {
  await page.evaluate(() =>
    window.dispatchEvent(new Event("visibilitychange"))
  );
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1) Open /budgets and wait for the list to render.
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".budget", { timeout: 60000 });
  await page.waitForTimeout(400);

  // 2) Find a budget row that has a "Top up…" button (i.e. not over-limit).
  const topupBtns = page.locator('button:has-text("Top up…")');
  const count = await topupBtns.count();
  if (count === 0) {
    fail("No 'Top up…' button found on /budgets — are all budgets over limit?");
    process.exit(1);
  }

  // 3) Identify the budget for the first "Top up…" button by its row NAME, and
  //    record that budget's limit from localStorage (the health-first sort reorders
  //    rows after a top-up, so we track by id/name, not DOM position).
  const rowEl = topupBtns.first().locator("xpath=ancestor::div[contains(@class,'budget')]");
  const rowName = (await rowEl.locator(".budget-name, .row-desc, .budget-head").first().textContent().catch(() => "")).trim();
  await flush(page);
  const before = await budgets(page);
  const target = before.find((b) => b.name && rowName.startsWith(b.name));
  if (!target) { fail(`could not match the top-up row "${rowName}" to a budget in the dataset`); process.exit(1); }
  const limitCentsBefore = (target.limit && target.limit.Amount) || 0;

  // 4) Click "Top up…", enter $50, submit.
  await topupBtns.first().click();
  await page.waitForTimeout(200);
  const topupInput = page.locator('input[aria-label="Amount to add"]');
  if ((await topupInput.count()) === 0) {
    fail("Top-up inline form did not appear after clicking 'Top up…'");
    process.exit(1);
  }
  await topupInput.fill("50");
  await page.locator('button:has-text("Add funds")').first().click();
  await flush(page);
  await page.waitForTimeout(400);

  // 5) Re-read THAT budget by id from localStorage and assert the limit rose $50.
  let after = await budgets(page);
  for (let i = 0; i < 10; i++) {
    const t = after.find((b) => b.id === target.id);
    if (t && (t.limit.Amount || 0) !== limitCentsBefore) break;
    await flush(page); after = await budgets(page);
  }
  const updated = after.find((b) => b.id === target.id);
  const delta = ((updated && updated.limit.Amount) || 0) - limitCentsBefore;
  if (delta !== 5000) {
    fail(`Limit did not rise by $50.00 — before: ${limitCentsBefore}, after: ${(updated && updated.limit.Amount)}, delta cents: ${delta}`);
  }
  const limitAmountAfter = ((updated && updated.limit.Amount) || 0) / 100;

  // 6) Confirm the inline form closed.
  if ((await page.locator('input[aria-label="Amount to add"]').count()) > 0) {
    fail("Top-up inline form did not close after successful submission");
  }

  // 9) Confirm the limit change persists in localStorage.
  const stored = await budgets(page);
  const limitCents = Math.round(limitAmountAfter * 100);
  const persisted = stored.some(
    (b) =>
      b.limit &&
      typeof b.limit.Amount === "number" &&
      Math.round(b.limit.Amount) === limitCents
  );
  if (!persisted) {
    fail(
      `Budget limit ${limitAmountAfter.toFixed(2)} not found in localStorage budgets after top-up`
    );
  }

  if (!process.exitCode) {
    console.log(
      `PASS: Top up $50.00 raised budget limit from ${limitAmountBefore.toFixed(2)} to ${limitAmountAfter.toFixed(2)} and persisted.`
    );
  }
} finally {
  await browser.close();
}
