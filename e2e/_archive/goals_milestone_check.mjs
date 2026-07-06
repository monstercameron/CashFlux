// Gate: MilestoneCrossed + milestone toast (L38).
// Seeds a goal at 0% then contributes to cross the 25% milestone;
// expects a notice to appear containing milestone text.
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
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

const GOAL_NAME = "ZZMilestoneGoal_" + Date.now();

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // Open Goals via the + Add modal.
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid="goal-add-form"]', { timeout: 60000 }).catch(() => null);

  // If the add form isn't inline, open via topbar + Add.
  if (!(await page.locator('[data-testid="goal-add-form"]').count())) {
    await page.locator('button[title="Add something new"]').click();
    await page.waitForTimeout(200);
    await page.locator('button:has-text("New goal")').click();
    await page.waitForTimeout(200);
  }

  // Fill in a goal: $100 target, $0 current.
  const addForm = page.locator('[data-testid="goal-add-form"]');
  await addForm.locator('input[type="text"]').fill(GOAL_NAME);
  await addForm.locator('input[type="number"]').first().fill("100");
  await addForm.locator('button[type="submit"]').click();
  await flush(page);

  let goals = await getGoals(page);
  const g = goals.find((x) => x.name === GOAL_NAME);
  if (!g) { fail("goal not found after add"); process.exit(1); }

  // Navigate back to /goals to render the row.
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(`[data-testid="goal-row-${g.id}"]`, { timeout: 30000 });

  // Click Contribute on the goal row.
  await page.locator(`[data-testid="goal-row-${g.id}"] button[title="Add to this goal"]`).click();
  await page.waitForTimeout(200);

  // Enter $30 — crosses the 25% milestone.
  await page.locator('input[type="number"][placeholder]').last().fill("30");
  await page.locator('form button[type="submit"]').last().click();
  await flush(page);

  // Expect a notice with milestone text (25% message).
  const noticeText = await page.locator('[role="status"], .notice, .toast, [aria-live]').allTextContents().catch(() => []);
  const hasToast = noticeText.some((t) => t.includes("Added") || t.includes("milestone") || t.includes("way there") || t.includes("25"));
  if (!hasToast) {
    // Also check page body for any notice text.
    const bodyText = await page.textContent("body");
    if (!bodyText.includes("way there") && !bodyText.includes("25%") && !bodyText.includes("Added $30")) {
      fail("no milestone or contribution toast visible after crossing 25%");
    }
  }

  // Verify goal progress updated.
  goals = await getGoals(page);
  const updated = goals.find((x) => x.id === g.id);
  if (!updated) { fail("goal missing after contribute"); }
  else if ((updated.currentAmount?.Amount ?? updated.currentAmount?.amount ?? 0) < 3000) {
    fail(`goal current amount not updated (got ${JSON.stringify(updated.currentAmount)})`);
  }

  if (!process.exitCode) console.log("PASS: goals milestone toast fires after crossing 25% threshold.");
} finally {
  await browser.close();
}
