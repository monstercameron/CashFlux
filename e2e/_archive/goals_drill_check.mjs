// C51 gate — "a linked goal drills into its account's transactions". Adds a goal
// linked to an account, clicks the "linked: <account>" affordance, and asserts it
// navigates to Transactions filtered to that account. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZLINK-GOAL";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Add a goal linked to a real account (linked select is the 2nd select; index 1
  // is the first real account after the "none" option).
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator("#goal-add").fill(NAME);
  await dialog.locator('input[type="number"]').nth(0).fill("1000");
  // The Linked-account select is tucked behind "Show advanced fields" (L38).
  const advToggle = dialog.locator('.cf-adv-toggle');
  if (await advToggle.count()) { await advToggle.first().click(); await page.waitForTimeout(150); }
  await dialog.locator('select').nth(1).selectOption({ index: 1 });
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);
  // Soft-nav cycle to force goals list re-render after modal add.
  await page.evaluate(() => { window.history.pushState({}, '', '/'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.history.pushState({}, '', '/goals'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(800);

  const row = page.locator(".budget", { hasText: NAME });
  if ((await row.count()) === 0) fail(`the linked goal "${NAME}" did not appear`);
  const drill = row.locator(".budget-drill");
  if ((await drill.count()) === 0) fail("the linked goal has no clickable linked-account affordance");
  await drill.first().click();

  await page.waitForFunction(() => location.pathname.endsWith("/transactions"), { timeout: 5000 }).catch(() => fail("did not navigate to /transactions"));
  // The inline add form moved to the +Add modal (C73); use the ledger table as the
  // "transactions screen loaded" marker.
  await page.waitForSelector("tr.row[data-id], .txn-table, [data-testid='txn-search']", { timeout: 60000 });

  const acct = await page.evaluate(() => {
    try {
      return (JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}")).account || "";
    } catch {
      return "";
    }
  });
  if (!acct) fail("the tx-filter account was not set by the goal drill-down");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: linked-goal drill-down → /transactions filtered to account "${acct}".`);
} finally {
  await browser.close();
}
