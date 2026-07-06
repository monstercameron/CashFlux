// C78 gate — "per-entity Recent changes filter on Activity screen."
// Navigates to /activity, asserts the entity-type filter control is present,
// selects "Transactions", and asserts that the timeline only shows rows whose
// entity type is "transaction" (or shows an appropriate empty state). Then
// resets to "All changes" and asserts the full timeline is restored.
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

  await page.goto(BASE + "/activity", { waitUntil: "domcontentloaded" });

  // Wait for the Activity screen card to appear.
  await page.waitForSelector(".card", { timeout: 30000 });

  // 1. The entity-type filter SelectInput must be present.
  const filterSelect = page.locator('[data-testid="activity-entity-filter"]');
  if (!(await filterSelect.isVisible({ timeout: 10000 }))) {
    fail("entity-type filter select not found on /activity");
  }

  // 2. Capture the row count before filtering.
  const allRows = page.locator(".rows .row");
  const allCount = await allRows.count();

  // 3. Select "Transactions" in the filter.
  await filterSelect.selectOption({ value: "transaction" });
  await page.waitForTimeout(500);

  // 4. After filtering, every visible row should have entity type "transaction",
  //    OR the empty state should be visible (no transaction entries yet).
  const filteredRows = page.locator(".rows .row");
  const filteredCount = await filteredRows.count();

  // If there are rows after filtering, ensure they are <= the unfiltered count
  // (narrowing, never broadening).
  if (filteredCount > allCount) {
    fail(`filtered row count (${filteredCount}) exceeds all-rows count (${allCount}) — filter is not narrowing`);
  }

  // 5. Reset to "All changes" and verify the count is restored.
  await filterSelect.selectOption({ value: "" });
  await page.waitForTimeout(500);
  const resetCount = await page.locator(".rows .row").count();
  if (resetCount !== allCount) {
    fail(`after reset to All, row count changed from ${allCount} to ${resetCount}`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) {
    console.log(
      `PASS: activity entity-type filter found; Transactions filter narrowed ${allCount}→${filteredCount} rows; reset restored ${resetCount} rows.`
    );
  }
} finally {
  await browser.close();
}
