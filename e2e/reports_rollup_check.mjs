// L28 gate — "Reports can roll sub-categories up into their parent." On the
// Spending-by-category card, toggling "Roll up sub-categories" combines children
// (e.g. Electricity + Internet → Utilities), so the number of distinct category
// rows drops and a parent name appears. Uses the Year period so the seeded
// sub-category spend is in range.
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
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  // The by-category breakdown (and its rollup toggle) lives on the Categories tab
  // of the redesigned bento surface.
  await page.waitForSelector(".bento-reports", { timeout: 60000 });
  await page.locator('.bento-reports button', { hasText: "Categories" }).first().click({ force: true });
  await page.waitForSelector('[data-testid="reports-rollup-toggle"]', { timeout: 60000 });

  // Use the Year period so the full sample (incl. sub-category spend) is in range.
  const year = page.locator(".reso-control").getByText("Year", { exact: true });
  if (await year.count()) { await year.first().click(); await page.waitForTimeout(500); }

  // The by-category section is the one holding the rollup toggle.
  const card = page.locator("#sec-categories");
  // The section holds two .rows lists (active + the zeroed-categories disclosure);
  // the rollup behaviour is about the ACTIVE list, so scope to the first.
  const rowCount = () => card.locator(".rows").first().locator(".row").count();
  const rowText = async () => (await card.locator(".rows").first().innerText()).replace(/\s+/g, " ");

  const before = await rowCount();
  const beforeText = await rowText();
  if (before < 2) { fail(`too few category rows to test rollup (${before})`); process.exit(1); }

  // Toggle roll-up.
  await page.locator('[data-testid="reports-rollup-toggle"]').click();
  await page.waitForTimeout(500);
  const after = await rowCount();
  const afterText = await rowText();

  if (after >= before) {
    fail(`rolling up did not reduce the category row count (${before} -> ${after})`);
  }
  // A seeded child (Electricity/Internet/Gas/Transit) should no longer be a row,
  // while its parent (Utilities/Transport) should be present — best-effort signal.
  if (/Utilities|Transport/.test(beforeText) || /Utilities|Transport/.test(afterText)) {
    if (!/Utilities|Transport/.test(afterText)) {
      fail("expected a parent category (Utilities/Transport) in the rolled-up view");
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: roll-up combined sub-categories into parents (${before} -> ${after} rows).`);
} finally {
  await browser.close();
}
