// Gate: budget add form rejects duplicate (category+period+owner) (L40).
// Creates a budget for an expense category+period, then attempts to add a second
// identical one; expects an inline error and no duplicate created.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getBudgets = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").budgets || []);
const getCats = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").categories || []);
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // Load the app first so localStorage has an origin (reading it on about:blank
  // throws a SecurityError).
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);

  // Ensure at least one expense category exists.
  let cats = await getCats(page);
  let expCat = cats.find((c) => c.kind === "expense" || c.kind === "Expense");
  if (!expCat) {
    await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(400);
    const catForm = page.locator('[data-testid="category-add-form"]');
    if (await catForm.count()) {
      await catForm.locator('input[type="text"]').fill("ZZDupGuardCat");
      await catForm.locator('button[type="submit"]').click();
    } else {
      await page.locator('button[title="Add something new"]').click();
      await page.waitForTimeout(200);
      await page.locator('button:has-text("New category")').click();
      await page.waitForTimeout(200);
      await page.locator('[data-testid="category-add-form"] input[type="text"]').fill("ZZDupGuardCat");
      await page.locator('[data-testid="category-add-form"] button[type="submit"]').click();
    }
    await flush(page);
    cats = await getCats(page);
    expCat = cats.find((c) => c.kind === "expense" || c.kind === "Expense");
    if (!expCat) { fail("could not create an expense category for this test"); process.exit(1); }
  }

  // Open /budgets and add the first budget.
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(400);

  const openAddForm = async () => {
    let addForm = page.locator('[data-testid="budget-add-form"]');
    if (!(await addForm.count())) {
      await page.locator('button[title="Add something new"]').click();
      await page.waitForTimeout(200);
      await page.locator('button:has-text("New budget")').click();
      await page.waitForTimeout(300);
      addForm = page.locator('[data-testid="budget-add-form"]');
    }
    return addForm;
  };

  let addForm = await openAddForm();
  if (!(await addForm.count())) { fail("budget add form not found"); process.exit(1); }

  // Select the expense category.
  await addForm.locator('select').first().selectOption({ label: expCat.name }).catch(async () => {
    // Try selecting by value.
    await addForm.locator('select').first().selectOption(expCat.id);
  });
  // Select Monthly period.
  await addForm.locator('select[aria-label="Period"]').selectOption("monthly").catch(() => {});
  await addForm.locator('input[type="number"]').first().fill("200");
  await addForm.locator('button[type="submit"]').click();
  await flush(page);

  const afterFirst = (await getBudgets(page)).length;

  // Try to add the same budget again (same category, period Monthly, same owner).
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(400);
  addForm = await openAddForm();

  if (!(await addForm.count())) { fail("budget add form not found on second attempt"); process.exit(1); }
  await addForm.locator('select').first().selectOption({ label: expCat.name }).catch(async () => {
    await addForm.locator('select').first().selectOption(expCat.id);
  });
  await addForm.locator('select[aria-label="Period"]').selectOption("monthly").catch(() => {});
  await addForm.locator('input[type="number"]').first().fill("300");
  await addForm.locator('button[type="submit"]').click();
  await page.waitForTimeout(300);

  const afterSecond = (await getBudgets(page)).length;
  if (afterSecond > afterFirst) {
    fail(`duplicate budget was created — count went from ${afterFirst} to ${afterSecond}`);
  }

  // Expect an error message containing "already exists" or similar.
  const bodyText = await page.textContent("body");
  if (!bodyText.includes("already") && !bodyText.includes("exists") && !bodyText.includes("duplicate")) {
    fail("no error message shown when adding a duplicate budget");
  }

  if (!process.exitCode) console.log("PASS: budget add form rejects duplicate (category+period+owner).");
} finally {
  await browser.close();
}
