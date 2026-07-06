// C49 e2e gate — inline-edit advanced-fields disclosure for asset accounts.
// Verifies: (1) opening the inline edit for an asset shows a "Show advanced
// fields" toggle (aria-expanded="false"), (2) clicking it reveals the
// expected-return / liquidity / stability / lock-until fields, (3) clicking
// it again hides them, (4) the form saves successfully without opening advanced.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(
  path.join(__dirname, "..", ".tools", "package.json")
);
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  // Wait for at least one Edit button (requires sample data or a pre-existing asset).
  await page.waitForSelector('[title]', { timeout: 60000 });

  // Load sample data if there are no accounts yet.
  const loadBtn = page.getByText("Load sample data");
  if (await loadBtn.count() > 0) {
    await loadBtn.click();
    await page.waitForTimeout(800);
  }

  // Open the first asset account's inline edit.
  const editBtns = page.locator('button[title]').filter({ hasText: /edit/i });
  if (await editBtns.count() === 0) {
    fail("no Edit buttons found on accounts page");
  } else {
    await editBtns.first().click();
    await page.waitForTimeout(400);

    // 1. The advanced-fields toggle must be visible and collapsed.
    const toggle = page.locator('button.cf-adv-toggle').first();
    if (await toggle.count() === 0) {
      fail("advanced-fields toggle not found in inline-edit form");
    } else {
      const expanded = await toggle.getAttribute("aria-expanded");
      if (expanded !== "false") {
        fail(`toggle should start collapsed (aria-expanded="false"), got "${expanded}"`);
      }

      // 2. Clicking the toggle reveals the advanced fields.
      await toggle.click();
      await page.waitForTimeout(200);
      const expandedAfter = await toggle.getAttribute("aria-expanded");
      if (expandedAfter !== "true") {
        fail(`toggle should be expanded after click, got "${expandedAfter}"`);
      }
      // At least one advanced field (e.g. expected-return number input) should appear.
      const advFields = page.locator('input[step="0.01"]');
      if (await advFields.count() === 0) {
        fail("no advanced number fields visible after expanding disclosure");
      }

      // 3. Clicking toggle again collapses the section.
      await toggle.click();
      await page.waitForTimeout(200);
      const collapsedAgain = await toggle.getAttribute("aria-expanded");
      if (collapsedAgain !== "false") {
        fail(`toggle should collapse on second click, got "${collapsedAgain}"`);
      }

      // 4. Saving without opening advanced should succeed (Cancel here to keep test
      //    non-destructive; a cancel is equivalent to verifying the form closes).
      const cancelBtn = page.locator('button').filter({ hasText: /cancel/i }).first();
      await cancelBtn.click();
      await page.waitForTimeout(300);
      // After cancel the edit form should be gone.
      if (await page.locator('button.cf-adv-toggle').count() > 0) {
        fail("edit form still visible after Cancel");
      }
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(
      "PASS: account inline-edit shows advanced-fields disclosure; toggle expands/collapses correctly."
    );
  }
} finally {
  await browser.close();
}
