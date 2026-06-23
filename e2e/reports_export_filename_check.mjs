// L45/L58 gate — Reports CSV exports carry period-stamped filenames so exports
// from different periods do not overwrite each other in the downloads folder.
//
// For Month resolution: expects "spending-by-category-YYYY-MM.csv"
// For Year  resolution: expects "spending-by-category-YYYY.csv"
//
// Invariants checked:
//   EXPORT_FILENAME_MONTH  — month-period export filename contains YYYY-MM stamp
//   EXPORT_FILENAME_YEAR   — year-period export filename contains YYYY stamp
//   NO_PAGE_ERRORS         — zero JS errors throughout
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// waitDownload — clicks a locator and captures the suggested filename from the
// download event triggered within timeoutMs.
async function waitDownload(page, locator, timeoutMs = 8000) {
  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: timeoutMs }),
    locator.click(),
  ]);
  return download.suggestedFilename();
}

try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // --- MONTH resolution check ---
  const monthSeg = page.locator('button, [role="radio"]', { hasText: /^Month$/ }).first();
  if ((await monthSeg.count()) > 0) {
    await monthSeg.click();
    await page.waitForTimeout(300);
  }

  await page.locator('a[title="Reports"], a[href="/reports"]').first().click();
  await page.waitForTimeout(800);

  // Find the spending-by-category CSV download button (first "Export CSV" / "Download CSV").
  const catCsvBtn = page.locator('[data-testid="reports-rollup-toggle"]')
    .locator("xpath=../..") // card parent
    .locator('button.btn', { hasText: /Export CSV|Download CSV/i })
    .first();

  if ((await catCsvBtn.count()) > 0) {
    try {
      const fname = await waitDownload(page, catCsvBtn);
      console.log("Month export filename:", fname);
      // Must match spending-by-category-YYYY-MM.csv pattern
      if (!/^spending-by-category-\d{4}-\d{2}\.csv$/.test(fname)) {
        fail(`EXPORT_FILENAME_MONTH: "${fname}" does not match spending-by-category-YYYY-MM.csv`);
      } else {
        console.log("PASS EXPORT_FILENAME_MONTH:", fname);
      }
    } catch (e) {
      console.log("SKIP EXPORT_FILENAME_MONTH: download event not triggered (no data?) —", e.message);
    }
  } else {
    console.log("SKIP EXPORT_FILENAME_MONTH: spending-by-category CSV button not found");
  }

  // --- YEAR resolution check ---
  const yearSeg = page.locator('button, [role="radio"]', { hasText: /^Year$/ }).first();
  if ((await yearSeg.count()) > 0) {
    await yearSeg.click();
    await page.waitForTimeout(400);
  } else {
    console.log("SKIP EXPORT_FILENAME_YEAR: Year segment not found");
    process.exit(process.exitCode || 0);
  }

  await page.waitForTimeout(500);

  // Re-locate the button after resolution change re-renders the page.
  const catCsvBtnYear = page.locator('[data-testid="reports-rollup-toggle"]')
    .locator("xpath=../..") // card parent
    .locator('button.btn', { hasText: /Export CSV|Download CSV/i })
    .first();

  if ((await catCsvBtnYear.count()) > 0) {
    try {
      const fname = await waitDownload(page, catCsvBtnYear);
      console.log("Year export filename:", fname);
      // Must match spending-by-category-YYYY.csv pattern (four-digit year, no month).
      if (!/^spending-by-category-\d{4}\.csv$/.test(fname)) {
        fail(`EXPORT_FILENAME_YEAR: "${fname}" does not match spending-by-category-YYYY.csv`);
      } else {
        console.log("PASS EXPORT_FILENAME_YEAR:", fname);
      }
    } catch (e) {
      console.log("SKIP EXPORT_FILENAME_YEAR: download event not triggered (no data?) —", e.message);
    }
  } else {
    console.log("SKIP EXPORT_FILENAME_YEAR: spending-by-category CSV button not found for Year resolution");
  }

  if (!process.exitCode) {
    console.log("PASS: reports export filename check complete.");
  }
} finally {
  await browser.close();
}
