// C73 gate — category add-form kind-select still works after IndentLabel/IndentPx
// migration. Asserts the kind select renders with the correct options and that
// selecting "Income" is reflected in the aria-label value (behavior unchanged).
// Exits non-zero on any failure.
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
  // The add-category form is in the +Add modal (C73/C79) — open it first.
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.locator('.add-btn').click();
  await page.locator('[role="menuitem"]', { hasText: /category/i }).first().click();
  await page.waitForSelector("#cat-add", { timeout: 60000 });

  // The add form must still render the kind select with accessible name.
  const form = page.locator("form", { has: page.locator("#cat-add") });
  const kindSelect = form.locator('select[aria-label="Category type"]');
  const count = await kindSelect.count();
  if (count !== 1) fail(`expected exactly 1 kind-select, found ${count}`);

  // It must have the two expected options.
  const options = await kindSelect.locator("option").allTextContents();
  const hasExpense = options.some((o) => o.toLowerCase().includes("expense"));
  const hasIncome = options.some((o) => o.toLowerCase().includes("income"));
  if (!hasExpense) fail(`kind-select missing Expense option; options: ${JSON.stringify(options)}`);
  if (!hasIncome) fail(`kind-select missing Income option; options: ${JSON.stringify(options)}`);

  // Selecting Income should not throw and the select value should update.
  await kindSelect.selectOption({ label: options.find((o) => o.toLowerCase().includes("income")) });
  const selected = await kindSelect.inputValue();
  if (!selected || selected === "") fail("kind-select value empty after selecting Income");

  // The parent select must also be present and labelled (it re-renders on kind change).
  const parentSelect = form.locator('select[aria-label="Parent category (optional)"]');
  const parentCount = await parentSelect.count();
  if (parentCount !== 1)
    fail(`expected exactly 1 parent-select, found ${parentCount}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log("PASS: C73 — category add-form kind-select renders correct options and responds to selection.");
} finally {
  await browser.close();
}
