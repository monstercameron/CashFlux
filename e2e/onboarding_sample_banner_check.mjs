// L6 E2E — "The First Night" sample-data banner checks.
//
// Scenarios:
//   1. Fresh first run (seeded sample) → banner is visible.
//   2. Click "Dismiss" → banner goes away; data stays intact.
//   3. Fresh first run again → click "Start fresh" → banner gone + dataset empty.
//
// Selectors
//   Banner:       [data-testid="sample-data-banner"]
//   Start fresh:  [data-testid="sample-start-fresh"]
//   Dismiss:      [data-testid="sample-dismiss"]
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Helper: read the full dataset from localStorage.
const dataset = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

// Helper: wait until a predicate on the dataset is true (up to timeoutMs).
async function waitForDataset(page, pred, timeoutMs = 9000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    const d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return dataset(page);
}

const nAccounts = (d) => (d.accounts || []).length;

try {
  // ── Scenario 1: fresh first run shows the banner ──────────────────────────
  const ctx1 = await browser.newContext();
  const page1 = await ctx1.newPage();
  const errors1 = [];
  page1.on("pageerror", (e) => errors1.push(String(e)));

  await page1.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page1.waitForSelector("#app *", { timeout: 60000 });

  // Wait for the sample to seed (accounts appear in the dataset).
  await waitForDataset(page1, (d) => nAccounts(d) > 0);
  await page1.waitForTimeout(500); // allow one render cycle

  const banner1 = page1.locator('[data-testid="sample-data-banner"]');
  if (!(await banner1.isVisible())) {
    fail("scenario 1: banner should be visible on first run with sample data");
  } else {
    console.log("PASS scenario 1: banner visible on first run.");
  }

  // ── Scenario 2: "Dismiss" hides banner, data unchanged ────────────────────
  const ctx2 = await browser.newContext();
  const page2 = await ctx2.newPage();
  const errors2 = [];
  page2.on("pageerror", (e) => errors2.push(String(e)));

  await page2.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page2.waitForSelector("#app *", { timeout: 60000 });
  await waitForDataset(page2, (d) => nAccounts(d) > 0);
  await page2.waitForTimeout(500);

  const dismiss = page2.locator('[data-testid="sample-dismiss"]');
  if (!(await dismiss.isVisible())) {
    fail("scenario 2: dismiss button not found in banner");
  } else {
    await dismiss.click();
    await page2.waitForTimeout(400);

    const banner2 = page2.locator('[data-testid="sample-data-banner"]');
    if (await banner2.isVisible()) {
      fail("scenario 2: banner should be gone after dismiss");
    } else {
      console.log("PASS scenario 2: banner gone after dismiss.");
    }

    // Data should still be present (dismiss doesn't wipe).
    const d2 = await dataset(page2);
    if (nAccounts(d2) === 0) {
      fail("scenario 2: dismiss should not delete accounts");
    } else {
      console.log("PASS scenario 2: accounts intact after dismiss.");
    }
  }

  // ── Scenario 3: "Start fresh" hides banner + empties dataset ─────────────
  const ctx3 = await browser.newContext();
  const page3 = await ctx3.newPage();
  const errors3 = [];
  page3.on("pageerror", (e) => errors3.push(String(e)));

  await page3.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page3.waitForSelector("#app *", { timeout: 60000 });
  await waitForDataset(page3, (d) => nAccounts(d) > 0);
  await page3.waitForTimeout(500);

  const startFresh = page3.locator('[data-testid="sample-start-fresh"]');
  if (!(await startFresh.isVisible())) {
    fail("scenario 3: start-fresh button not found in banner");
  } else {
    await startFresh.click();
    await page3.waitForTimeout(600);

    const banner3 = page3.locator('[data-testid="sample-data-banner"]');
    if (await banner3.isVisible()) {
      fail("scenario 3: banner should be gone after Start fresh");
    } else {
      console.log("PASS scenario 3: banner gone after Start fresh.");
    }

    // After Start fresh the in-memory dataset is empty.
    const d3 = await waitForDataset(page3, (d) => nAccounts(d) === 0, 5000);
    if (nAccounts(d3) !== 0) {
      fail(`scenario 3: dataset should be empty after Start fresh (got ${nAccounts(d3)} accounts)`);
    } else {
      console.log("PASS scenario 3: dataset empty after Start fresh.");
    }

    // Flush autosave, reload, confirm still empty (seeded flag stays set so no re-seed).
    await page3.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    await page3.waitForTimeout(400);
    await page3.reload({ waitUntil: "domcontentloaded" });
    await page3.waitForSelector("#app *", { timeout: 60000 });
    await page3.waitForTimeout(2500);
    const d3r = await dataset(page3);
    if (nAccounts(d3r) !== 0) {
      fail(`scenario 3: after reload, store re-seeded (${nAccounts(d3r)} accounts) — seeded flag must be kept`);
    } else {
      console.log("PASS scenario 3: empty slate survives reload.");
    }
  }

  // ── Aggregate page-error check ────────────────────────────────────────────
  const allErrors = [...errors1, ...errors2, ...errors3];
  if (allErrors.length) fail("page errors: " + allErrors.join(" | "));

  await ctx1.close();
  await ctx2.close();
  await ctx3.close();

  if (!process.exitCode) {
    console.log("PASS: all onboarding sample-banner checks passed.");
  }
} finally {
  await browser.close();
}
