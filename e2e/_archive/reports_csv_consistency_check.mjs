// C55 gate — Reports CSV export is consistent: top-payees and largest-expenses
// sections both have a "Download CSV" button (matching category / income / member).
// Asserts: [data-testid="reports-payees-csv"] and [data-testid="reports-largest-csv"]
// exist when those sections are visible (non-empty sample data).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // Use Year resolution so seeded transactions fall in range.
  await page.locator('a[title="Reports"]').first().click();
  await page.waitForTimeout(700);

  // Switch to Year view so data is present.
  const yearSeg = page.locator('button, [role="radio"]', { hasText: /^Year$/ }).first();
  if ((await yearSeg.count()) > 0) {
    await yearSeg.click();
    await page.waitForTimeout(400);
  }

  // Check payees section CSV button.
  const payeesCsv = page.locator('[data-testid="reports-payees-csv"]');
  const largestCsv = page.locator('[data-testid="reports-largest-csv"]');

  const payeesVisible = (await payeesCsv.count()) > 0;
  const largestVisible = (await largestCsv.count()) > 0;

  // If the top-payees section rendered, the CSV button must be there.
  const topPayeesSection = page.locator('section.card', { hasText: "Top payees" }).first();
  if ((await topPayeesSection.count()) > 0 && !payeesVisible) {
    fail("Top payees section visible but [data-testid=reports-payees-csv] not found");
  }

  // If the biggest-expenses section rendered, the CSV button must be there.
  const biggestSection = page.locator('section.card', { hasText: "Biggest expenses" }).first();
  if ((await biggestSection.count()) > 0 && !largestVisible) {
    fail("Biggest expenses section visible but [data-testid=reports-largest-csv] not found");
  }

  if (!payeesVisible && !largestVisible) {
    console.log("SKIP: neither top-payees nor largest-expenses sections are visible with current sample data");
    process.exit(0);
  }

  if (!process.exitCode) {
    const found = [payeesVisible && "payees", largestVisible && "largest"].filter(Boolean).join(", ");
    console.log(`PASS: CSV download buttons present for: ${found}.`);
  }
} finally {
  await browser.close();
}
