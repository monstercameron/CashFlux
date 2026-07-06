// SMART Wave 2 tooltip spread e2e — verifies that key-figure explainer tooltips
// and section quick-actions are woven into the main list pages beyond the Dashboard.
//
//   1. At the default density (Standard), the Budgets page carries a smart tooltip
//      on the "left / safe-to-spend" figure; clicking it reveals the explanation.
//   2. The Budgets page also carries a smart section action button.
//   3. Setting density to Off removes the tooltip and section action from Budgets.
//   4. Restore density to Standard so the run leaves a sane default.
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
    // Boot and load sample data so there are budgets/goals/accounts to render.
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 20000 });
    await page.waitForTimeout(1200);
    await dismissOverlay(page);
    const loadSample = page.locator('[data-testid="hero-load-sample"]');
    if (await loadSample.count() > 0) {
      await loadSample.first().click();
      await page.waitForTimeout(1500);
    }

    // Ensure density is Standard before testing (restore from any previous run).
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "standard");
    await page.waitForTimeout(1500);

    // --- 1. Budgets tooltip: "safe to spend" figure carries smart-tip-budget-safe ---
    await goto(page, "/budgets", "#cf-page-view");
    ok(
      (await page.locator('[data-testid="smart-tip-budget-safe"]').count()) > 0,
      "Budgets: safe-to-spend tooltip present at Standard density"
    );

    // Clicking the tooltip button reveals the explanation popover.
    await page
      .locator('[data-testid="smart-tip-budget-safe"] button')
      .first()
      .click();
    await page.waitForTimeout(400);
    ok(
      (await page.locator('[data-testid="smart-tip-pop"]').count()) > 0,
      "Budgets: clicking the tooltip reveals the explanation popover"
    );

    // --- 2. Budgets section action is present ---
    await goto(page, "/budgets", "#cf-page-view");
    ok(
      (await page.locator('[data-testid="smart-section-action"]').count()) > 0,
      "Budgets: smart section action button present at Standard density"
    );

    // --- 3. Density Off removes both tooltip and section action ---
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "off");
    await page.waitForTimeout(3000); // persist

    await goto(page, "/budgets", "#cf-page-view");
    ok(
      (await page.locator('[data-testid="smart-tip-budget-safe"]').count()) === 0,
      "Budgets: density Off removes the safe-to-spend tooltip"
    );
    ok(
      (await page.locator('[data-testid="smart-section-action"]').count()) === 0,
      "Budgets: density Off removes the section action"
    );

    // --- 4. Restore Standard density ---
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "standard");
    await page.waitForTimeout(1500);

    console.log("\nSMART TOOLTIP SPREAD E2E: PASS");
    await browser.close();
    process.exit(0);
  } catch (err) {
    console.error("\nSMART TOOLTIP SPREAD E2E: FAIL —", err.message);
    if (consoleErrors.length)
      console.error("console errors:", consoleErrors.slice(0, 8));
    await browser.close();
    process.exit(1);
  }
})();
