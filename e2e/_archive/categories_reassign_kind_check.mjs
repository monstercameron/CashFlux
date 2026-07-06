// C63 gate — "reassign-before-delete only offers same-kind categories". Deleting an
// in-use EXPENSE category must not let you reassign its data to the INCOME category
// (a data-integrity hazard). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
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

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".row", { timeout: 60000 });

  // Delete the in-use expense category "Groceries" → opens the reassign panel.
  const row = page.locator(".row", { hasText: "Groceries" }).first();
  if ((await row.count()) === 0) fail("no Groceries category row found");
  await row.locator(".btn-del").first().click();
  await page.waitForTimeout(400);

  // The reassign form is the one with a "Move…/delete" submit button. Scope to it so
  // the add-form's Kind select (which has an "Income" option) doesn't confuse us.
  const reassignForm = page.locator("form").filter({ has: page.getByRole("button", { name: /move/i }) }).first();
  if ((await reassignForm.count()) === 0) fail("reassign panel did not open (is Groceries in use in the sample?)");
  const opts = (await reassignForm.locator("select option").allInnerTexts()).map((t) => t.trim());

  if (opts.includes("Income")) fail(`reassign offered the income category to an expense deletion: ${opts.join(", ")}`);
  if (!opts.some((o) => /Dining|Housing|Utilities|Shopping|Transportation/.test(o))) {
    fail(`reassign should still offer other expense categories (saw: ${opts.join(", ")})`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: reassign offers only same-kind (expense) targets, not "Income" (${opts.filter(Boolean).join(", ")}).`);
} finally {
  await browser.close();
}
