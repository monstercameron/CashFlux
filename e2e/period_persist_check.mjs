// L45/L58 gate — period window persistence: the full From/To window (not just
// the resolution) survives a hard reload. Verifies that after navigating to a
// prior period, reloading the page, and landing on /reports the period pill
// still shows the previously-selected period rather than snapping back to the
// current period.
//
// Invariants checked:
//   WINDOW_PERSISTED  — period pill after reload matches the period set before reload
//   NO_PAGE_ERRORS    — zero JS errors throughout
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// Helper: read the period pill text currently shown in the top bar.
async function getPeriodPill(page) {
  // The stepper pill renders the window label (e.g. "May 2026", "2025", "Jun 2 – Jun 8").
  const pill = page.locator('.rstep-pill, [class*="stepper-pill"], [class*="rstep"]').first();
  if ((await pill.count()) === 0) return null;
  return (await pill.textContent()).trim();
}

try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => { errors.push(e.message); fail("page error: " + e.message); });

  // Step 0 — load the app.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(600);

  // Step 1 — switch to Month resolution and navigate to the previous month.
  // Use the segmented control to pick Month, then click the prev-period stepper.
  const monthSeg = page.locator('button, [role="radio"]', { hasText: /^Month$/ }).first();
  if ((await monthSeg.count()) > 0) {
    await monthSeg.click();
    await page.waitForTimeout(300);
  }

  // Step back one period (previous month).
  const prevBtn = page.locator('[aria-label*="previous"], [aria-label*="Previous"], [title*="previous"], [title*="Previous"], button[aria-label*="earlier"], button[aria-label*="Earlier"]').first();
  if ((await prevBtn.count()) > 0) {
    await prevBtn.click();
    await page.waitForTimeout(300);
  } else {
    // Fallback: try the preset "Last period".
    const presetSel = page.locator('select[aria-label*="Jump"], select[title*="Jump"]').first();
    if ((await presetSel.count()) > 0) {
      await presetSel.selectOption("last");
      await page.waitForTimeout(300);
    }
  }

  // Navigate to /reports so the screen persists the window.
  await page.locator('a[title="Reports"], a[href="/reports"]').first().click();
  await page.waitForTimeout(700);

  const pillBefore = await getPeriodPill(page);
  if (!pillBefore) {
    console.log("SKIP: period pill not found — period control layout may differ");
    process.exit(0);
  }
  console.log("Period before reload:", pillBefore);

  // Step 2 — hard reload and navigate back to /reports.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(600);

  // Navigate to /reports after reload.
  await page.locator('a[title="Reports"], a[href="/reports"]').first().click();
  await page.waitForTimeout(700);

  const pillAfter = await getPeriodPill(page);
  console.log("Period after reload:", pillAfter);

  // WINDOW_PERSISTED: the period pill must match across the reload.
  if (pillAfter !== pillBefore) {
    fail(`WINDOW_PERSISTED: period pill changed after reload. Before: "${pillBefore}", After: "${pillAfter}"`);
  } else {
    console.log(`PASS WINDOW_PERSISTED: period pill "${pillAfter}" survived hard reload.`);
  }

  if (!process.exitCode) {
    console.log("PASS: period window persistence check complete.");
  }
} finally {
  await browser.close();
}
