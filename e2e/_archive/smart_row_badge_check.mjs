// SMART Wave 1 — row-level badge e2e
//
// Verifies that enabled, account-targeting smart features render a quiet severity
// dot (✦) on individual account rows, and that the global density dial removes
// those badges when set to Off.
//
//   1. Load sample data, enable SMART-A7 (recurring-charge detection, fires on
//      the Accounts page with account IDs as RelatedID), navigate to /accounts
//      and assert a smart badge appears on at least one row.
//   2. Set density to Off; reload /accounts and assert no smart badges remain.
//   3. Restore density to Standard and re-enable state for a clean exit.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const consoleErrors = [];
function ok(c, m) {
  if (!c) throw new Error("ASSERT FAILED: " + m);
  console.log("  ok — " + m);
}
async function dismissOverlay(page) {
  await page.evaluate(() => {
    const o =
      document.getElementById("gwc-error-overlay") ||
      document.querySelector(".gwc-error-overlay");
    if (o) o.remove();
  });
}
async function goto(page, route, sel) {
  await page.goto(BASE + route, { waitUntil: "domcontentloaded" });
  await page.waitForSelector(sel, { timeout: 20000 });
  await dismissOverlay(page);
  await page.waitForTimeout(700);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on("console", (m) => {
    if (m.type() === "error") consoleErrors.push(m.text());
  });

  try {
    // ---- seed: load sample data if the accounts page is empty ----
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 20000 });
    await page.waitForTimeout(1200);
    await dismissOverlay(page);
    const loadSample = page.locator('[data-testid="hero-load-sample"]');
    if (await loadSample.count() > 0) {
      await loadSample.first().click();
      await page.waitForTimeout(2000);
    }

    // ---- step 1: enable SMART-A7 (recurring-charge detection) ----
    // SMART-A7 is a Free rule engine on PageAccounts that sets RelatedID = account.ID;
    // it fires whenever an account has 2+ recurring charge patterns (very common with
    // the Hartley sample dataset which has 48 months of recurring debits).
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    // Open all catalog accordions so feature rows are reachable on the
    // flattened surface.
    for (const g of await page.$$('[data-testid^="smart-group-"]')) { await g.click(); }
    await page.waitForTimeout(400);

    // Ensure density is Standard (badges need ≥ Standard)
    await page.selectOption('[data-testid="smart-density"]', "standard");
    await page.waitForTimeout(1000);

    // Enable SMART-A7 (the toggle is a div[role="switch"] not an input)
    const a7feature = page.locator('[data-testid="smart-feature-SMART-A7"]');
    ok(await a7feature.count() > 0, "SMART-A7 feature row exists on the smart hub");
    // The switch div carries aria-checked; only click if it's off
    const a7switch = a7feature.locator('[role="switch"]');
    const ariaChecked = await a7switch.getAttribute("aria-checked");
    if (ariaChecked !== "true") {
      await a7switch.click();
      await page.waitForTimeout(1500);
    }

    // Settings autosave — wait for persistence before navigating
    await page.waitForTimeout(3000);

    // ---- step 2: navigate to /accounts and assert badge appears ----
    await goto(page, "/accounts", "#cf-page-view");
    await page.waitForTimeout(1500);

    const badges = page.locator('[data-testid^="smart-badge-"]');
    const badgeCount = await badges.count();
    ok(
      badgeCount > 0,
      `at least one smart-badge appears on an account row (found ${badgeCount})`
    );

    // ---- step 3: density Off → badges disappear ----
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "off");
    await page.waitForTimeout(3000); // let autosave flush

    await goto(page, "/accounts", "#cf-page-view");
    await page.waitForTimeout(1000);

    const badgesOff = page.locator('[data-testid^="smart-badge-"]');
    ok(
      (await badgesOff.count()) === 0,
      "density Off hides all smart row badges"
    );

    // ---- restore: set density back to Standard and leave clean ----
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "standard");
    await page.waitForTimeout(1500);

    console.log("\nSMART ROW BADGE E2E: PASS");
    await browser.close();
    process.exit(0);
  } catch (err) {
    console.error("\nSMART ROW BADGE E2E: FAIL —", err.message);
    if (consoleErrors.length)
      console.error("console errors:", consoleErrors.slice(0, 8));
    await browser.close();
    process.exit(1);
  }
})();
