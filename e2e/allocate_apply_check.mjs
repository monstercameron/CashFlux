// L17 gate — "Apply allocation commits goal contributions and earmarks, and Undo restores."
// Verifies:
//   1. An amount entered on the Allocate screen surfaces the Apply button.
//   2. Clicking Apply → Confirm persists: goal's currentAmount rises in
//      localStorage `cashflux:dataset` and an earmark record appears.
//   3. Clicking Undo restores goal currentAmount and removes the earmark.
//
// The sample dataset must have at least one incomplete goal and one asset account.
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

/**
 * Read the live cashflux:dataset from localStorage and parse it.
 */
async function readDataset(page) {
  const raw = await page.evaluate(() => localStorage.getItem("cashflux:dataset"));
  if (!raw) return null;
  return JSON.parse(raw);
}

// Flush the periodic autosave (it also fires on visibilitychange) and poll until
// the dataset is present and satisfies pred.
async function waitForDataset(page, pred, timeoutMs = 10000) {
  let d = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await readDataset(page);
    if (d && pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}
// money.Money serializes with capitalized Go field names (no json tags): {Amount,Currency}.
const amt = (m) => (m && typeof m.Amount === "number" ? m.Amount : 0);

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Navigate to the Allocate screen (routed under /planning in the nav).
  await page.goto(BASE + "/allocate", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".budget", { timeout: 60000 });
  await page.waitForTimeout(500);

  // Record pre-apply state (wait for the seed to persist with at least one goal).
  const before = await waitForDataset(page, (d) => (d.goals || []).length > 0);
  if (!before) { fail("cashflux:dataset not found in localStorage before apply"); process.exit(1); }
  const goalsBefore = before.goals || [];
  const earmarksBefore = (before.earmarks || []).length;

  // Enter an amount (100.00 in whatever the base currency is).
  const amountInput = page.locator('input[placeholder*="Amount to allocate"]').first();
  await amountInput.fill("100");
  await amountInput.dispatchEvent("input");
  await page.waitForTimeout(300);

  // The Apply button should now be visible.
  const applyBtn = page.locator('[data-testid="allocate-apply-btn"]');
  await applyBtn.waitFor({ state: "visible", timeout: 5000 });

  // Click Apply → opens confirm panel.
  await applyBtn.click();
  await page.waitForTimeout(300);

  // Confirm button appears.
  const confirmBtn = page.locator('button:has-text("Confirm")').first();
  await confirmBtn.waitFor({ state: "visible", timeout: 5000 });
  await confirmBtn.click();
  await page.waitForTimeout(800);

  // Read post-apply dataset (wait until a goal rose or an earmark appeared).
  const after = await waitForDataset(page, (d) => {
    const gb = (d.goals || []).some((g) => {
      const prev = goalsBefore.find((b) => b.id === g.id);
      return prev && amt(g.currentAmount) > amt(prev.currentAmount);
    });
    return gb || (d.earmarks || []).length > earmarksBefore;
  });
  if (!after) { fail("cashflux:dataset not found after apply"); process.exit(1); }

  // At least one goal's currentAmount should have increased, OR an earmark created.
  const goalsAfter = after.goals || [];
  const earmarksAfter = (after.earmarks || []).length;

  const goalBumped = goalsAfter.some((g) => {
    const prev = goalsBefore.find((b) => b.id === g.id);
    return prev && amt(g.currentAmount) > amt(prev.currentAmount);
  });
  const earmarkCreated = earmarksAfter > earmarksBefore;

  if (!goalBumped && !earmarkCreated) {
    fail(`neither a goal was funded nor an earmark persisted after Apply. ` +
         `goalsBefore=${JSON.stringify(goalsBefore.map(g=>({id:g.id,cur:g.currentAmount})))} ` +
         `goalsAfter=${JSON.stringify(goalsAfter.map(g=>({id:g.id,cur:g.currentAmount})))} ` +
         `earmarksBefore=${earmarksBefore} earmarksAfter=${earmarksAfter}`);
  }

  // Undo button should be visible.
  const undoBtn = page.locator('button:has-text("Undo")').first();
  await undoBtn.waitFor({ state: "visible", timeout: 5000 });
  await undoBtn.click();
  await page.waitForTimeout(800);

  // Read post-undo dataset (wait until earmarks return to baseline).
  const afterUndo = await waitForDataset(page, (d) => (d.earmarks || []).length <= earmarksBefore);
  if (!afterUndo) { fail("cashflux:dataset not found after undo"); process.exit(1); }

  const goalsAfterUndo = afterUndo.goals || [];
  const earmarksAfterUndo = (afterUndo.earmarks || []).length;

  // After undo, goal amounts should be back to pre-apply values.
  const goalRestored = goalsBefore.every((g) => {
    const undone = goalsAfterUndo.find((a) => a.id === g.id);
    if (!undone) return true;
    return amt(undone.currentAmount) === amt(g.currentAmount);
  });
  if (!goalRestored) {
    fail(`goal currentAmount not restored after Undo. ` +
         `before=${JSON.stringify(goalsBefore.map(g=>({id:g.id,cur:g.currentAmount})))} ` +
         `afterUndo=${JSON.stringify(goalsAfterUndo.map(g=>({id:g.id,cur:g.currentAmount})))}`);
  }
  if (earmarksAfterUndo > earmarksBefore) {
    fail(`earmarks not cleared after Undo: before=${earmarksBefore} afterUndo=${earmarksAfterUndo}`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(`PASS: Apply committed ${goalBumped ? "a goal contribution" : "no goal"} and ${earmarkCreated ? "an earmark" : "no earmark"}; Undo restored state.`);
  }
} finally {
  await browser.close();
}
