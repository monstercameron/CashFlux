// L27 gate — "runway indicator on what-if plans".
// Adds a sabbatical plan (start $22,500, -$4,000/mo, 12-month horizon) and
// asserts that the resulting plan card shows:
//   • a "Money lasts ~5.6 months" (or similar) runway readout in the danger tone
//   • a danger badge element (.plan-runway--danger)
//
// Form selectors used (no id/name attrs on these inputs — they are identified
// by placeholder text or by their order within the plan add-form):
//   Plan name:       input[type="text"][placeholder*="Plan name"]
//   Horizon (months): first input[type="number"][min="1"] in the form
//   Starting balance: input[type="number"][step="0.01"] nth(0) in the form
//   Monthly change:   input[type="number"][step="0.01"] nth(1) in the form
//   Submit:           button[type="submit"] with text "Add plan"
//
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PLAN_NAME = "Sabbatical runway e2e " + Date.now();

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  // Wait for the plan add-form to be present.
  await page.waitForSelector('input[placeholder*="Plan name"]', { timeout: 60000 });
  await page.waitForTimeout(400);

  // Scope ALL inputs to the plan add-form (the one with the "Add plan" submit).
  // The page also has a "Can I afford it?" section with its own step="0.01"
  // inputs earlier in the DOM, so global nth() selectors hit the wrong fields.
  const planForm = page.locator('form', { has: page.locator('button[type="submit"]:has-text("Add plan")') });
  await planForm.locator('input[placeholder*="Plan name"]').fill(PLAN_NAME);
  // Horizon (months): the min="1" number input within the plan form.
  await planForm.locator('input[type="number"][min="1"]').first().fill("12");
  // Within the plan form, the step="0.01" inputs are start balance then monthly.
  const decimalInputs = planForm.locator('input[type="number"][step="0.01"]');
  await decimalInputs.nth(0).fill("22500");
  await decimalInputs.nth(1).fill("-4000");

  await planForm.locator('button[type="submit"]:has-text("Add plan")').click();

  // Poll until a plan card with the plan name appears. The sample seeds its own
  // plans, so scope every assertion to OUR plan's card (the ancestor row of the
  // row-desc carrying the unique name) rather than a global .first().
  const nameSpan = page.locator(`.row-desc`, { hasText: PLAN_NAME });
  await nameSpan.waitFor({ state: "attached", timeout: 15000 });
  // The plan card is the nearest ancestor that also holds the runway readout.
  const card = page.locator('.row', { has: page.locator(`.row-desc`, { hasText: PLAN_NAME }) }).first();
  await card.waitFor({ state: "attached", timeout: 5000 });
  await page.waitForTimeout(300);

  // The danger badge must be present on OUR plan's card.
  const dangerBadge = card.locator('.plan-runway--danger');
  if ((await dangerBadge.count()) === 0) fail("danger badge (.plan-runway--danger) not on the new plan card");

  // The runway text must contain a number close to 5.6 (22500 / 4000 ≈ 5.625).
  const runwayText = (await card.locator('.plan-runway__text').first().textContent().catch(() => "")) || "";
  if (!/~?5[.,]\d/.test(runwayText)) {
    fail(`runway text does not show ~5.x months; got: "${runwayText}"`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(`PASS: runway indicator — badge visible, runway text: "${runwayText}"`);
} finally {
  await browser.close();
}
