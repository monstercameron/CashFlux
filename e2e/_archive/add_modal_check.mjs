// C79 gate — "+Add" menu opens entity add forms in FlipPanel modals. Verifies:
// 1. Clicking the top-bar +Add button opens the popover menu.
// 2. Clicking the "Goal" item opens a [role=dialog] FlipPanel with the goal add form.
// 3. Submitting with empty required fields keeps the dialog open (validation error).
// 4. Filling valid values and submitting closes the dialog and persists the goal.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const GOAL_NAME = "E2E-MODAL-GOAL-" + Date.now();
const TARGET_AMOUNT = "500";

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const goals = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").goals || []);
async function flush(page) { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(400); }

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // Load the app at any route (dashboard is fine).
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  // Wait for the top-bar +Add button.
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // 1. Open the +Add popover menu.
  await page.locator(".add-btn").click();
  await page.waitForTimeout(200);

  // 2. Click the Goal menu item (data-testid or role=menuitem with visible text).
  const goalItem = page.locator('[role="menuitem"]', { hasText: /goal/i });
  if ((await goalItem.count()) === 0) { fail("Goal menu item not found in +Add menu"); process.exit(1); }
  await goalItem.first().click();
  await page.waitForTimeout(400);

  // Assert a [role=dialog] is present (the FlipPanel).
  const dialog = page.locator('[role="dialog"]');
  if ((await dialog.count()) === 0) { fail("FlipPanel dialog did not appear after clicking Goal menu item"); process.exit(1); }

  // The goal add form must be inside the dialog.
  const nameField = dialog.locator('#goal-add');
  if ((await nameField.count()) === 0) { fail('goal name field (#goal-add) not found inside dialog'); process.exit(1); }

  // 3. Submit with empty required fields — dialog must STAY open (validation error).
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(300);
  if ((await page.locator('[role="dialog"]').count()) === 0) { fail("dialog closed after empty submit — should have shown validation error and stayed open"); process.exit(1); }

  // 4. Fill valid values and submit — dialog should CLOSE and goal should persist.
  await nameField.fill(GOAL_NAME);
  // Target amount is aria-required.
  await dialog.locator('input[type="number"][aria-required="true"]').first().fill(TARGET_AMOUNT);
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  // Dialog should be gone.
  if ((await page.locator('[role="dialog"]').count()) !== 0) {
    fail("dialog did not close after a valid goal submission");
  }

  // Goal should exist in localStorage.
  await flush(page);
  let all = await goals(page);
  for (let i = 0; i < 8 && !all.find((g) => g.name === GOAL_NAME); i++) { await flush(page); all = await goals(page); }
  const saved = all.find((g) => g.name === GOAL_NAME);
  if (!saved) fail(`goal "${GOAL_NAME}" not found in localStorage cashflux:dataset after modal add`);

  if (!process.exitCode) console.log(`PASS: +Add → Goal modal opened, invalid submit kept dialog open, valid submit added "${GOAL_NAME}" and closed dialog.`);
} finally {
  await browser.close();
}
