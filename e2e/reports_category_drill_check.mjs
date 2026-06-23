// L58 gate — Reports → Transactions category drill-through. Seeds one account,
// one expense category, and one transaction; navigates to /reports; clicks the
// first [data-testid="reports-cat-drill"] button; asserts navigation to
// /transactions with the category filter persisted in the TxFilter atom.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
async function flush(page) { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(400); }

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // The seeded sample already has category spending (dining, groceries, …), so the
  // by-category Reports table renders drill buttons without any extra seeding.
  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid="reports-cat-drill"]', { timeout: 60000 });

  // 4) Click the first category drill button and assert navigation to /transactions.
  const drillBtn = page.locator('[data-testid="reports-cat-drill"]').first();
  const catId = await drillBtn.evaluate((el) => {
    const row = el.closest('[data-testid="reports-cat-row"]');
    return row ? row.getAttribute("data-category-id") : null;
  });
  if (!catId) { fail("could not read data-category-id from cat row"); process.exit(1); }

  await drillBtn.click();
  await page.waitForURL((url) => url.pathname === new URL(BASE + "/transactions").pathname || url.href.includes("/transactions"), { timeout: 10000 });

  const finalUrl = page.url();
  if (!finalUrl.includes("/transactions")) { fail(`expected /transactions after drill, got ${finalUrl}`); }

  // 5) Assert that the TxFilter in localStorage has the category set.
  await page.waitForTimeout(300);
  const filterRaw = await page.evaluate(() => localStorage.getItem("cashflux:tx-filter"));
  if (!filterRaw) { fail("tx-filter not persisted to localStorage after drill"); process.exit(1); }
  const filter = JSON.parse(filterRaw);
  if (filter.category !== catId) {
    fail(`tx-filter.category = "${filter.category}", want "${catId}"`);
  }

  if (!process.exitCode) {
    console.log(`PASS: /reports category drill → /transactions with category filter "${catId}" persisted.`);
  }
} finally {
  await browser.close();
}
