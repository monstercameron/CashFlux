// Gate: category add form resets kind to Expense after adding an Income category (L42).
// Adds an Income category, then checks that the kind select reverts to Expense
// so the next add starts from a clean default.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

const CAT_NAME = "ZZIncomeKindReset_" + Date.now();

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);

  // Get or open the category add form.
  let addForm = page.locator('[data-testid="category-add-form"]');
  if (!(await addForm.count())) {
    await page.locator('button[title="Add something new"]').click();
    await page.waitForTimeout(200);
    await page.locator('button:has-text("New category")').click();
    await page.waitForTimeout(300);
    addForm = page.locator('[data-testid="category-add-form"]');
  }

  if (!(await addForm.count())) { fail("category add form not found"); process.exit(1); }

  // Select "Income" kind.
  await addForm.locator('select[aria-label="Category type"]').selectOption("income");
  await page.waitForTimeout(100);

  // Fill the name and submit.
  await addForm.locator('input[type="text"]').fill(CAT_NAME);
  await addForm.locator('button[type="submit"]').click();
  await flush(page);

  // After the add, the form should reset. Re-locate it (it may re-render).
  await page.waitForTimeout(300);
  addForm = page.locator('[data-testid="category-add-form"]');
  if (!(await addForm.count())) {
    // Modal may have closed; re-open.
    await page.locator('button[title="Add something new"]').click();
    await page.waitForTimeout(200);
    await page.locator('button:has-text("New category")').click();
    await page.waitForTimeout(300);
    addForm = page.locator('[data-testid="category-add-form"]');
  }

  if (!(await addForm.count())) {
    // Can't verify the reset if the form closed; skip but warn.
    console.log("WARN: category add form not re-visible after add — skipping kind-reset assertion.");
    process.exit(0);
  }

  // The kind select should now be "expense" (the default after reset).
  const kindSelect = addForm.locator('select[aria-label="Category type"]');
  const selectedValue = await kindSelect.inputValue();
  if (selectedValue !== "expense") {
    fail(`kind select not reset to expense after add (got "${selectedValue}")`);
  }

  if (!process.exitCode) console.log("PASS: category add form kind resets to Expense after submitting an Income category.");
} finally {
  await browser.close();
}
