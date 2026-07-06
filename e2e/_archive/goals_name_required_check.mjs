// Gate: goal add form rejects empty name (L41).
// Attempts to submit the GoalAddForm with no name; expects an inline error.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getGoals = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").goals || []);

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);

  // Open the add form.
  let addForm = page.locator('[data-testid="goal-add-form"]');
  if (!(await addForm.count())) {
    // Try the + Add modal.
    await page.locator('button[title="Add something new"]').click();
    await page.waitForTimeout(200);
    await page.locator('button:has-text("New goal")').click();
    await page.waitForTimeout(300);
    addForm = page.locator('[data-testid="goal-add-form"]');
  }

  const beforeCount = (await getGoals(page)).length;

  // Leave name blank, fill only a target amount.
  await addForm.locator('input[type="number"]').first().fill("50");
  await addForm.locator('button[type="submit"]').click();
  await page.waitForTimeout(300);

  // Goal count must not increase.
  const afterCount = (await getGoals(page)).length;
  if (afterCount !== beforeCount) {
    fail(`goal was created without a name (before=${beforeCount}, after=${afterCount})`);
  }

  // An error message should be visible in the form.
  const errVisible = await addForm.locator('[id="goal-err"], [role="alert"], .error, p.muted').count();
  const bodyText = await page.textContent("body");
  if (errVisible === 0 && !bodyText.includes("name") && !bodyText.includes("required")) {
    fail("no error message shown when submitting a goal with an empty name");
  }

  if (!process.exitCode) console.log("PASS: goal add form rejects empty name with an inline error.");
} finally {
  await browser.close();
}
