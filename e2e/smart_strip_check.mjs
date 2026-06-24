// SMART inline strip e2e — the per-page intelligence interspersed across the app.
//
// Proves the SMART layer is woven into each relevant page (not only the /smart
// hub) AND that it is strictly additive:
//   1. Enable a Budgets Free feature (SMART-B8 "safe to spend", which fires
//      whenever there's liquid cash) on the hub.
//   2. /budgets shows an inline Smart strip (data-testid=smart-strip-budgets)
//      with a B8 insight card.
//   3. The Dashboard ("/") shows the cross-page strip (smart-strip-all) with it.
//   4. /accounts shows NO strip (no Accounts feature enabled) — additive: a page
//      is untouched until the user opts into something for it.
//
// NOTE: the app logs one pre-existing "released function" console error per route
// change (app-wide, predates this); it does NOT gate this test.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const consoleErrors = [];

function ok(cond, msg) {
  if (!cond) throw new Error("ASSERT FAILED: " + msg);
  console.log("  ok — " + msg);
}
async function dismissOverlay(page) {
  await page.evaluate(() => {
    const o = document.getElementById("gwc-error-overlay") || document.querySelector(".gwc-error-overlay");
    if (o) o.remove();
  });
}
async function goto(page, route, waitSel) {
  await page.goto(BASE + route, { waitUntil: "domcontentloaded" });
  await page.waitForSelector(waitSel, { timeout: 20000 });
  await dismissOverlay(page);
  await page.waitForTimeout(700);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
  try {
    // Boot + sample data.
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 20000 });
    await page.waitForTimeout(1200);
    await dismissOverlay(page);
    const loadSample = page.locator('[data-testid="hero-load-sample"]');
    if (await loadSample.count() > 0) { await loadSample.first().click(); await page.waitForTimeout(1500); }

    // Enable SMART-B8 on the hub.
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    const b8 = page.locator('[data-testid="smart-feature-SMART-B8"]');
    ok(await b8.count() > 0, "SMART-B8 toggle present on the hub");
    await b8.locator('button, input, [role="switch"]').first().click();
    await page.waitForTimeout(2500); // let the opt-in autosave flush

    // /budgets → inline strip with a card.
    await goto(page, "/budgets", "#cf-page-view");
    await page.waitForSelector('[data-testid="smart-strip-budgets"]', { timeout: 10000 });
    ok(true, "the Budgets page shows an inline Smart strip");
    ok(
      await page.locator('[data-testid="smart-strip-budgets"] [data-testid="smart-card"]').count() > 0,
      "the Budgets strip contains a live insight card",
    );

    // Dashboard cross-page strip.
    await goto(page, "/", "#cf-page-view");
    ok(await page.locator('[data-testid="smart-strip-all"]').count() > 0, "the Dashboard shows the cross-page Smart strip");

    // /accounts → NO strip (additive: nothing enabled for Accounts).
    await goto(page, "/accounts", "#cf-page-view");
    ok(
      await page.locator('[data-testid="smart-strip-accounts"]').count() === 0,
      "the Accounts page shows NO strip (additive — no Accounts feature enabled)",
    );

    // The toggle is a feature flag: turning SMART-B8 OFF removes it from the
    // Budgets page entirely.
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.locator('[data-testid="smart-feature-SMART-B8"] button, [data-testid="smart-feature-SMART-B8"] input, [data-testid="smart-feature-SMART-B8"] [role="switch"]').first().click();
    await page.waitForTimeout(4000); // settings autosave must flush before reload
    await goto(page, "/budgets", "#cf-page-view");
    ok(
      await page.locator('[data-testid="smart-strip-budgets"]').count() === 0,
      "toggling SMART-B8 OFF removes the strip from Budgets (the toggle is a feature flag)",
    );

    const releasedOnly = consoleErrors.every((e) => /released function/i.test(e));
    if (consoleErrors.length && !releasedOnly) console.log("  console errors (non-gating):", consoleErrors.slice(0, 5));

    console.log("\nSMART STRIP E2E: PASS");
    await browser.close();
    process.exit(0);
  } catch (err) {
    console.error("\nSMART STRIP E2E: FAIL —", err.message);
    if (consoleErrors.length) console.error("console errors:", consoleErrors.slice(0, 8));
    await browser.close();
    process.exit(1);
  }
})();
