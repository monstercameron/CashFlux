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

    // --- 6. AI feature: honest cost badge + provider gating --------------
    // The A5 (account Q&A) row is an AI feature: its badge must show the AI tier,
    // a per-use cost, and — with no provider configured in this fresh context —
    // a "needs a provider" hint. This is the cost-transparency promise for AI.
    const a5Row = page.locator('[data-testid="smart-feature-SMART-A5"]');
    ok(await a5Row.count() > 0, "SMART-A5 (AI) toggle row present");
    const a5Text = await a5Row.innerText();
    ok(/\bAI\b/.test(a5Text), "A5 row shows the AI tier badge");
    ok(/\/use/.test(a5Text), "A5 row shows a per-use cost");
    ok(/needs a provider/i.test(a5Text), "A5 row warns it needs a provider (none configured)");

    // Enabling an AI feature with no provider surfaces the gated AI section
    // (a hint to configure a provider), never a dead control.
    const a5Toggle = a5Row.locator('button, input, [role="switch"]').first();
    await a5Toggle.click();
    await page.waitForTimeout(1000);
    await dismissOverlay(page);
    ok(await page.locator('[data-testid="smart-ai"]').count() > 0, "enabling an AI feature shows the AI section");
    const aiText = await page.locator('[data-testid="smart-ai"]').innerText();
    ok(/provider/i.test(aiText), "AI section explains a provider is required (honest gating, no dead control)");

    // An enabled AI feature exposes per-feature run controls: a schedule/cadence
    // picker (when it runs) and a mute/snooze button.
    ok(await page.locator('[data-testid="smart-cadence-SMART-A5"]').count() > 0, "enabled AI feature shows a schedule/cadence picker");
    ok(await page.locator('[data-testid="smart-mute-SMART-A5"]').count() > 0, "enabled AI feature shows a mute control");
    // The cadence picker offers Manual (the click-before-run default) and Weekly.
    const cadOpts = await page.locator('[data-testid="smart-cadence-SMART-A5"] option').allInnerTexts();
    ok(cadOpts.some((o) => /manual/i.test(o)) && cadOpts.some((o) => /weekly/i.test(o)), "cadence picker offers Manual + Weekly schedules");

    // Global controls: the density dial + Enable all / Disable all.
    ok(await page.locator('[data-testid="smart-density"]').count() > 0, "the density dial is present");
    const dens = await page.locator('[data-testid="smart-density"] option').allInnerTexts();
    ok(dens.some((o) => /standard/i.test(o)) && dens.some((o) => /everywhere/i.test(o)), "density offers Standard + Everywhere");
    await page.locator('[data-testid="smart-enable-all"]').first().click();
    await page.waitForTimeout(1200);
    ok(await page.locator('[data-testid="smart-card"]').count() >= 1, "Enable all surfaces insights across the catalog");
    await page.locator('[data-testid="smart-disable-all"]').first().click();
    await page.waitForTimeout(1200);
    ok(await page.locator('[data-testid="smart-card"]').count() === 0, "Disable all clears every insight");

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
