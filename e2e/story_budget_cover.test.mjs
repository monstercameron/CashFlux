// L1 E2E story - "cover overspending by moving money between budgets". Creates a
// tiny-limit budget on a category that already has sample spend (so it is over),
// plus a funding budget, then uses the row's "Cover" form to move money from the
// source into the over budget. Asserts both budgets re-balance by the covered
// amount (balanced total), the move persists, and it survives a reload.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const OVER = "ZZCOVER-OVER";
const SRC = "ZZCOVER-SRC";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const budgetByName = (page, name) =>
  page.evaluate((nm) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    let found = null;
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) return o.forEach(walk);
      if (o.name === nm && o.limit && o.period) found = o;
      Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  }, name);

// addBudget fills the Budgets add form for the given category label + limit.
async function addBudget(page, name, categoryLabel, limit) {
  await page.locator("#budget-add").fill(name);
  await page.locator("form.form-grid select").first().selectOption({ label: categoryLabel });
  await page.locator('input[type="number"][aria-required="true"]').fill(limit);
  await page.locator('form.form-grid button[type="submit"]').first().click();
  await page.waitForTimeout(500);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#budget-add", { timeout: 60000 });

  // The over budget: $1 limit on Groceries (which has sample spend) -> over.
  // The source budget: a roomy limit on Shopping.
  await addBudget(page, OVER, "Groceries", "1");
  await addBudget(page, SRC, "Shopping", "500");

  await page.waitForTimeout(2500); // let the dataset autosave flush
  const srcBefore = await budgetByName(page, SRC);
  const overBefore = await budgetByName(page, OVER);
  if (!srcBefore || !overBefore) fail("seed budgets were not persisted");

  // Open the over row's Cover form (the only over budget -> the only toggle).
  const coverToggle = page.locator('button[title^="Move money from another budget"]');
  if ((await coverToggle.count()) === 0) fail('the over budget did not offer a "Cover" action');
  await coverToggle.first().click();
  await page.waitForSelector(".cover-form", { timeout: 8000 });

  // Pick the source budget by name and move $50 (well within its room).
  const srcSelect = page.locator('select[aria-label="Cover from budget"]');
  const opts = srcSelect.locator("option");
  let srcVal = null;
  for (let i = 0; i < (await opts.count()); i++) {
    const t = (await opts.nth(i).textContent()) || "";
    if (t.includes(SRC)) {
      srcVal = await opts.nth(i).getAttribute("value");
      break;
    }
  }
  if (!srcVal) fail(`the cover picker did not list "${SRC}" as a source`);
  await srcSelect.selectOption(srcVal);
  await page.locator('input[aria-label="Amount to move"]').fill("50");
  await page.screenshot({ path: path.join(__dirname, "budget-cover.png") });
  await page.locator('.cover-form button[type="submit"]').click();
  await page.waitForTimeout(700);

  // The over budget gained $50 (limit $1 -> $51) and the source dropped $50
  // ($500 -> $450). Assert via the rendered rows (updates immediately); the
  // balance is implied (51 + 450 == 1 + 500).
  const overRow = page.locator(".budget").filter({ hasText: OVER });
  const srcRow = page.locator(".budget").filter({ hasText: SRC });
  const overText = (await overRow.first().innerText()).replace(/\s+/g, " ");
  const srcText = (await srcRow.first().innerText()).replace(/\s+/g, " ");
  if (!overText.includes("/ $51.00")) fail(`over budget limit did not rise to $51.00: ${overText}`);
  if (!srcText.includes("/ $450.00")) fail(`source budget limit did not drop to $450.00: ${srcText}`);

  // Survives reload (pagehide flushes the dataset to localStorage).
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#budget-add", { timeout: 60000 });
  await page.waitForTimeout(800);
  const srcReload = await budgetByName(page, SRC);
  if (!srcReload || srcReload.limit.Amount !== srcBefore.limit.Amount - 5000)
    fail("the cover did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: covered $50 from "${SRC}" into over budget "${OVER}"; balanced + persists across reload.`);
} finally {
  await browser.close();
}
