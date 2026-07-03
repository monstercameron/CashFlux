// L27 gate — side-by-side plan comparison on the forecast chart.
//
// The planning screen was enhanced (L27) with:
//   • A "Compare with plan" select inside the forecast card — overlays a saved
//     plan's monthly-change projection on the 12-month forecast chart.
//   • A "Prefill start from account" select in the plan add-form — pre-fills
//     the starting balance from a chosen account's current ledger balance.
//
// Flow:
//   1. Navigate to /planning; assert the forecast card loads.
//   2. Add two plans with different monthly changes (+500 and -200/mo, 6-month
//      horizon from a fixed start of $10,000).
//   3. After both plans exist, reload /planning and assert:
//      a. The "Compare with plan" select is present in the forecast card.
//      b. The "Prefill start from account" select is present in the plan form.
//   4. Select the +500 plan in the compare-with picker.
//   5. Assert the compare note ([data-testid="plan-compare-note"]) appears
//      and names the selected plan.
//   6. Assert the compare note contains a dollar figure (the projected end
//      value for the compared plan).
//
// Invariants:
//   I1: compare-with select appears only when ≥1 saved plan exists.
//   I2: selecting a plan makes the compare-note appear with that plan's name.
//   I3: compare-note includes projected-end figures for both curves.
//   I4: prefill-account select present in the plan add-form.
//
// Exits non-zero on any assertion failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TAG = "ZZcmp" + Date.now();
const PLAN_A = TAG + " Raise (+500)";
const PLAN_B = TAG + " Expense (-200)";

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

async function addPlan(page, name, horizon, start, monthly) {
  const planForm = page.locator('form', {
    has: page.locator('button[type="submit"]:has-text("Add plan")'),
  });
  await planForm.locator('input[type="text"]').first().fill(name);
  await planForm.locator('input[type="number"][min="1"]').first().fill(String(horizon));
  const decInps = planForm.locator('input[type="number"][step="0.01"]');
  await decInps.nth(0).fill(String(start));
  await decInps.nth(1).fill(String(monthly));
  await planForm.locator('button[type="submit"]:has-text("Add plan")').click();
  await page.waitForTimeout(800);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1. Navigate to /planning.
  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[placeholder*="Plan name"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // 2. Add two plans.
  await addPlan(page, PLAN_A, 6, 10000, 500);
  await addPlan(page, PLAN_B, 6, 10000, -200);

  // Confirm both appear.
  const bodyAfterAdd = await page.evaluate(() => document.body.innerText);
  if (!bodyAfterAdd.includes(PLAN_A)) fail(`Plan A ("${PLAN_A}") not found after add`);
  else console.log(`  PASS: plan A "${PLAN_A}" added`);
  if (!bodyAfterAdd.includes(PLAN_B)) fail(`Plan B ("${PLAN_B}") not found after add`);
  else console.log(`  PASS: plan B "${PLAN_B}" added`);

  // 3. Reload /planning and verify new UI elements.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('.bento-planning', { timeout: 60000 });
  await page.waitForTimeout(600);

  // I4: prefill-account select must be present in the plan form.
  const prefillSel = page.locator('[data-testid="plan-prefill-account"]');
  if ((await prefillSel.count()) === 0) {
    fail("I4: 'Prefill start from account' select ([data-testid=\"plan-prefill-account\"]) not found in plan add-form");
  } else {
    console.log("  PASS I4: plan-prefill-account select present");
  }

  // I1: compare-with select must be present now that plans exist.
  const compareSel = page.locator('[data-testid="plan-compare-select"]');
  if ((await compareSel.count()) === 0) {
    fail("I1: 'Compare with plan' select ([data-testid=\"plan-compare-select\"]) not found in forecast card");
    process.exit(1);
  }
  console.log("  PASS I1: plan-compare-select present");

  // 4. Select plan A in the compare picker.
  // The option text matches PLAN_A; select by label text.
  await compareSel.selectOption({ label: PLAN_A });
  await page.waitForTimeout(600);

  // 5. Assert the compare note appears.
  const compareNote = page.locator('[data-testid="plan-compare-note"]');
  if ((await compareNote.count()) === 0) {
    fail("I2: compare note ([data-testid=\"plan-compare-note\"]) did not appear after selecting a plan");
  } else {
    const noteText = await compareNote.first().innerText();
    // I2: note must name the selected plan.
    if (!noteText.includes(PLAN_A)) {
      fail(`I2: compare note does not name selected plan "${PLAN_A}". Got: "${noteText}"`);
    } else {
      console.log(`  PASS I2: compare note names plan A: "${noteText.slice(0, 100)}"`);
    }
    // I3: note must include dollar figures.
    if (!(/\$[\d,]/.test(noteText))) {
      fail(`I3: compare note does not include dollar figures. Got: "${noteText}"`);
    } else {
      console.log("  PASS I3: compare note contains dollar figures");
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: planning side-by-side plan comparison works.");
} finally {
  await browser.close();
}
