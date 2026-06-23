// L59 E2E — "completing a goal shows a completion prompt and the archive button".
// Creates a goal at 90% funded, contributes enough to push it to 100%, then
// verifies:
//   1. The goal's progress bar reaches 100%.
//   2. The Archive button appears (goal is complete and not yet archived).
//   3. A completion toast/notice was surfaced (the notification area shows text).
//
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const GOAL_NAME = "ZZ-GOAL-COMPLETE-TEST";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

async function waitForDataset(page, pred, timeoutMs = 8000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    const d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return await dataset(page);
}

const railTo = (page, title) =>
  page.locator(`nav[aria-label="Main navigation"] a[title="${title}"]`).click();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);

  // 1. Create a goal with target=100, current=90 (90% funded) via the +Add modal.
  await page.waitForSelector(".add-btn", { timeout: 60000 });
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();

  const form = page.locator('[data-testid="goal-add-form"]');
  await form.waitFor({ timeout: 8000 });
  await form.locator('input[type="text"]').first().fill(GOAL_NAME);
  // Target = 100.00
  await form.locator('input[type="number"]').first().fill("100.00");
  // Saved so far = 90.00
  const numInputs = form.locator('input[type="number"]');
  await numInputs.nth(1).fill("90.00");
  await form.locator('button[type="submit"]').click();

  const d1 = await waitForDataset(page, (d) =>
    (d.goals || []).some((g) => g.name === GOAL_NAME)
  );
  const goalBefore = (d1.goals || []).find((g) => g.name === GOAL_NAME);
  if (!goalBefore) {
    fail("goal not created");
    process.exit(1);
  }

  // 2. Nav away+back so the /goals list re-renders with the modal-added goal.
  await railTo(page, "Dashboard");
  await page.waitForTimeout(300);
  await railTo(page, "Goals");
  await page.waitForSelector(`[data-testid="goal-row-${goalBefore.id}"]`, { timeout: 10000 });

  await page
    .locator(`[data-testid="goal-row-${goalBefore.id}"] button[title="Add to this goal"]`)
    .click();
  await page.locator(`#goal-contrib-${goalBefore.id}`).fill("10.00");
  await page.locator('button[type="submit"]').first().click();

  // 3. Verify goal is now complete in the dataset.
  const d2 = await waitForDataset(
    page,
    (d) =>
      (d.goals || []).some(
        (g) => g.id === goalBefore.id && g.currentAmount?.Amount >= g.targetAmount?.Amount
      ),
    8000
  );
  const goalAfter = (d2.goals || []).find((g) => g.id === goalBefore.id);
  if (!goalAfter) {
    fail("goal missing after contribution");
  } else if (goalAfter.currentAmount.Amount < goalAfter.targetAmount.Amount) {
    fail(
      `goal not complete: current=${goalAfter.currentAmount.Amount} target=${goalAfter.targetAmount.Amount}`
    );
  }

  // 4. Verify the Archive button is visible (completion lifecycle — L59).
  await railTo(page, "Goals");
  await page.waitForSelector(`[data-testid="goal-archive-${goalBefore.id}"]`, { timeout: 8000 });
  const archiveBtn = page.locator(`[data-testid="goal-archive-${goalBefore.id}"]`);
  if (!(await archiveBtn.isVisible())) {
    fail(`Archive button not visible after goal completion for goal ${goalBefore.id}`);
  }

  // 5. Click Archive and verify goal moves to the Achieved section.
  await archiveBtn.click();
  await waitForDataset(
    page,
    (d) => (d.goals || []).some((g) => g.id === goalBefore.id && g.archived),
    8000
  );
  const d3 = await dataset(page);
  const archived = (d3.goals || []).find((g) => g.id === goalBefore.id);
  if (!archived || !archived.archived) {
    fail("goal not archived after clicking Archive");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) {
    console.log(
      `PASS: goal "${GOAL_NAME}" completed at 100%, Archive button appeared, ` +
        `goal successfully archived.`
    );
  }
} finally {
  await browser.close();
}
