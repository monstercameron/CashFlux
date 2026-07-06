// SMART affordance toolkit e2e — the inline smart surfaces woven into the app
// (here: the explainer tooltip), and the global density dial that gates them.
//
//   1. At the default density (Standard), the Dashboard net-worth figure carries
//      an opt-in smart explainer tooltip; clicking it reveals the explanation.
//   2. Setting density to Off removes the tooltip everywhere — the density dial
//      governs how much smart weaves into the app.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const consoleErrors = [];
function ok(c, m) { if (!c) throw new Error("ASSERT FAILED: " + m); console.log("  ok — " + m); }
async function dismissOverlay(page) {
  await page.evaluate(() => { const o = document.getElementById("gwc-error-overlay") || document.querySelector(".gwc-error-overlay"); if (o) o.remove(); });
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
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
  try {
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 20000 });
    await page.waitForTimeout(1200);
    await dismissOverlay(page);
    const loadSample = page.locator('[data-testid="hero-load-sample"]');
    if (await loadSample.count() > 0) { await loadSample.first().click(); await page.waitForTimeout(1500); }

    // 1. Default density (Standard) → the net-worth explainer tooltip is present.
    await goto(page, "/", "#cf-page-view");
    ok(await page.locator('[data-testid="smart-tip-networth"]').count() > 0, "net-worth explainer tooltip is present at Standard density");
    // Click reveals the explanation popover.
    await page.locator('[data-testid="smart-tip-networth"] button').first().click();
    await page.waitForTimeout(400);
    ok(await page.locator('[data-testid="smart-tip-pop"]').count() > 0, "clicking the tooltip reveals the explanation");

    // 2. Density Off → tooltip gone everywhere.
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "off");
    await page.waitForTimeout(3000); // persist
    await goto(page, "/", "#cf-page-view");
    ok(await page.locator('[data-testid="smart-tip-networth"]').count() === 0, "density Off removes the tooltip (the dial governs weaving)");

    // restore to standard so the run leaves a sane default
    await goto(page, "/smart", '[data-testid="smart-hub"]');
    await page.selectOption('[data-testid="smart-density"]', "standard");
    await page.waitForTimeout(1500);

    console.log("\nSMART AFFORDANCE E2E: PASS");
    await browser.close();
    process.exit(0);
  } catch (err) {
    console.error("\nSMART AFFORDANCE E2E: FAIL —", err.message);
    if (consoleErrors.length) console.error("console errors:", consoleErrors.slice(0, 8));
    await browser.close();
    process.exit(1);
  }
})();
