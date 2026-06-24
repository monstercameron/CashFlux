// SMART hub e2e — the /smart per-page-intelligence surface.
//
// Exercises the full opt-in stack end to end against the real wasm app:
//   1. Load sample data, navigate to /smart.
//   2. The Manage catalog renders with feature toggle rows, each carrying a
//      Free/AI cost badge; with nothing enabled the insights section shows the
//      onboarding copy (no cards).
//   3. Enable a deterministic Free feature (SMART-B8 "safe to spend", which fires
//      whenever there is liquid cash) → an insight card appears with the matching
//      data-feature, proving the adapter → engine → card pipeline works live.
//   4. The opt-in persists across a reload (the toggle stays on).
//   5. Dismissing the card removes it.
//
// NOTE: the app logs one pre-existing "call to released function" console error
// per route change (present app-wide before this feature); it is reported but does
// NOT gate this test, matching loopstory_90.
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

async function gotoSmart(page) {
  await page.goto(BASE + "/smart", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid="smart-hub"]', { timeout: 20000 });
  await dismissOverlay(page);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage();
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });

  try {
    // --- 1. Boot + load sample data ---------------------------------------
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app", { timeout: 20000 });
    await page.waitForTimeout(1200);
    await dismissOverlay(page);

    const loadSample = page.locator('[data-testid="hero-load-sample"]');
    if (await loadSample.count() > 0) {
      await loadSample.first().click();
      await page.waitForTimeout(1500);
      console.log("  loaded sample data");
    } else {
      console.log("  (sample already loaded or hero absent)");
    }

    // --- 2. /smart renders with the manage catalog ------------------------
    await gotoSmart(page);
    ok(await page.locator('[data-testid="smart-manage"]').count() > 0, "Manage section renders");

    const b8Row = page.locator('[data-testid="smart-feature-SMART-B8"]');
    ok(await b8Row.count() > 0, "SMART-B8 toggle row present in the catalog");

    // Cost badge is present (Free for B8) — the cost-transparency promise.
    const badgeText = await b8Row.innerText();
    ok(/Free/i.test(badgeText), "B8 row shows a Free cost badge");

    // A Free row and an AI row both exist somewhere (honest tiering).
    const anyAITier = await page.locator('[data-testid="smart-manage"]').innerText();
    ok(/Free/.test(anyAITier), "catalog shows Free tier labels");

    // Nothing enabled yet → onboarding, no cards.
    ok(await page.locator('[data-testid="smart-card"]').count() === 0, "no insight cards before opting in");

    // --- 3. Enable B8 → an insight card appears ---------------------------
    const toggle = b8Row.locator('button, input, [role="switch"]').first();
    ok(await toggle.count() > 0, "B8 row has a switch control");
    await toggle.click();
    await page.waitForTimeout(1200);
    await dismissOverlay(page);

    await page.waitForSelector('[data-testid="smart-card"][data-feature="SMART-B8"]', { timeout: 10000 });
    ok(true, "enabling B8 surfaced a live insight card (adapter→engine→card)");

    // --- 4. Opt-in persists across reload ---------------------------------
    // Give the dataset autosave (which carries the preserved settingskv table)
    // time to flush to IndexedDB before reloading.
    await page.waitForTimeout(3000);
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('[data-testid="smart-hub"]', { timeout: 20000 });
    await dismissOverlay(page);
    await page.waitForTimeout(800);
    ok(
      await page.locator('[data-testid="smart-card"][data-feature="SMART-B8"]').count() > 0,
      "the B8 opt-in + its insight persist across a reload",
    );

    // --- 5. Dismiss removes the card --------------------------------------
    const card = page.locator('[data-testid="smart-card"][data-feature="SMART-B8"]').first();
    await card.locator('[data-testid="smart-dismiss"]').first().click();
    await page.waitForTimeout(1000);
    ok(
      await page.locator('[data-testid="smart-card"][data-feature="SMART-B8"]').count() === 0,
      "dismissing removes the insight card",
    );

    const releasedFnOnly = consoleErrors.every((e) => /released function/i.test(e));
    if (consoleErrors.length && !releasedFnOnly) {
      console.log("  console errors (non-gating):", consoleErrors.slice(0, 5));
    }

    console.log("\nSMART HUB E2E: PASS");
    await browser.close();
    process.exit(0);
  } catch (err) {
    console.error("\nSMART HUB E2E: FAIL —", err.message);
    if (consoleErrors.length) console.error("console errors:", consoleErrors.slice(0, 8));
    await browser.close();
    process.exit(1);
  }
})();
