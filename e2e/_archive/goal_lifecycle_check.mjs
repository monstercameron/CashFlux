// L20 gate — goal-completion lifecycle: over-funding note, archive → Achieved,
// unarchive, overall-progress exclusion.
//
// Strategy: create an over-funded goal (target $1, saved $2) and a normal
// in-progress goal ($100, $0), then exercise the full lifecycle. Poll
// localStorage cashflux:dataset to confirm archived:true is persisted.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
// Unique names so the test doesn't collide with existing seeded goals.
const OVER_NAME = "ZZ-OVER-FUNDED-GOAL";
const ACTIVE_NAME = "ZZ-ACTIVE-GOAL";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const poll = async (fn, { timeout = 5000, interval = 200 } = {}) => {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (await fn()) return true;
    await new Promise((r) => setTimeout(r, interval));
  }
  return false;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // ── Add the over-funded goal (target $1.00, saved $2.00) ──────────────────
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator("#goal-add").fill(OVER_NAME);
  const nums = dialog.locator('.field[type="number"]');
  await dialog.locator('input[type="number"]').nth(0).fill("1");   // target
  const advL = dialog.locator('.cf-adv-toggle'); // saved-so-far is behind Advanced (L38)
  if (await advL.count()) { await advL.first().click(); await page.waitForTimeout(150); }
  await dialog.locator('input[type="number"]').nth(1).fill("2");   // saved so far (over-funded by $1)
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);

  // ── Add a normal in-progress goal ($100 target, $0 saved) ────────────────
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog2 = page.locator('[role="dialog"]');
  await dialog2.locator("#goal-add").fill(ACTIVE_NAME);
  await dialog2.locator('input[type="number"]').nth(0).fill("100");
  const advL2 = dialog2.locator('.cf-adv-toggle'); // saved-so-far behind Advanced (L38)
  if (await advL2.count()) { await advL2.first().click(); await page.waitForTimeout(150); }
  await dialog2.locator('input[type="number"]').nth(1).fill("0");
  await dialog2.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);
  // Soft-nav cycle to force goals list re-render after modal adds.
  await page.evaluate(() => { window.history.pushState({}, '', '/'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.history.pushState({}, '', '/goals'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(800);

  // ── Assert: over-funding note is visible on the over-funded row ──────────
  // We use the data-testid the screen attaches: data-testid="goal-overfund-<id>"
  const overRow = page.locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME });
  if ((await overRow.count()) === 0) {
    fail(`over-funded goal "${OVER_NAME}" row not found`);
  } else {
    const overfundNote = overRow.locator('[data-testid^="goal-overfund-"]');
    if ((await overfundNote.count()) === 0) {
      fail(`over-funding note not shown on "${OVER_NAME}"`);
    } else {
      const noteText = await overfundNote.first().textContent();
      if (!/over target/i.test(noteText)) {
        fail(`over-funding note text unexpected: "${noteText}"`);
      }
    }
  }

  // ── Capture overall % before archive ─────────────────────────────────────
  const pctBefore = await page.locator(".stat-grid").textContent();

  // ── Click Archive on the over-funded goal ────────────────────────────────
  const archiveBtn = overRow.locator('[data-testid^="goal-archive-"]').first();
  if ((await archiveBtn.count()) === 0) {
    fail(`Archive button not found on completed goal "${OVER_NAME}"`);
  } else {
    await archiveBtn.click();
    await page.waitForTimeout(700);
  }

  // ── Assert: goal left the active list ────────────────────────────────────
  const activeSection = page.locator('section.card', { hasNotText: "Achieved" }).first();
  const stillActive = await activeSection.locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME }).count();
  if (stillActive > 0) {
    fail(`"${OVER_NAME}" still appears in the active list after archive`);
  }

  // ── Assert: Achieved section is visible and contains the goal ────────────
  const achievedSection = page.locator('[aria-label="Achieved"]');
  if ((await achievedSection.count()) === 0) {
    fail(`"Achieved" section not rendered after archiving a goal`);
  } else {
    const inAchieved = await achievedSection.locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME }).count();
    if (inAchieved === 0) {
      fail(`"${OVER_NAME}" not found inside the Achieved section`);
    }
  }

  // ── Assert: overall % changed (archived goal excluded from numerator) ─────
  const pctAfter = await page.locator(".stat-grid").textContent();
  if (pctBefore === pctAfter) {
    // Overall % should drop because the over-funded goal contributed more to
    // savedTotal than targetTotal relative to the active set.
    // If it didn't change the text at all something is wrong.
    fail(`overall progress stat did not change after archiving the over-funded goal (before="${pctBefore}", after="${pctAfter}")`);
  }

  // ── Assert: localStorage has archived:true on this goal ──────────────────
  const persisted = await poll(async () => {
    const raw = await page.evaluate(() => localStorage.getItem("cashflux:dataset"));
    if (!raw) return false;
    const dataset = JSON.parse(raw);
    const goals = dataset.goals || [];
    const found = goals.find((g) => g.name === OVER_NAME);
    return found && found.archived === true;
  });
  if (!persisted) {
    fail(`localStorage cashflux:dataset does not have archived:true on "${OVER_NAME}"`);
  }

  // ── Unarchive: click Unarchive in Achieved section ────────────────────────
  const unarchiveBtn = achievedSection
    .locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME })
    .locator('[data-testid^="goal-unarchive-"]')
    .first();
  if ((await unarchiveBtn.count()) === 0) {
    fail(`Unarchive button not found for "${OVER_NAME}" in Achieved section`);
  } else {
    await unarchiveBtn.click();
    await page.waitForTimeout(700);

    // Should be back in active list.
    const backActive = await page.locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME }).count();
    if (backActive === 0) {
      fail(`"${OVER_NAME}" did not return to the active list after unarchive`);
    }
    // Achieved section should be gone (or not contain the goal).
    const achCount = await achievedSection.count();
    if (achCount > 0) {
      const stillInAchieved = await achievedSection
        .locator('[data-testid^="goal-row-"]', { hasText: OVER_NAME })
        .count();
      if (stillInAchieved > 0) {
        fail(`"${OVER_NAME}" still appears in Achieved after unarchive`);
      }
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(
      "PASS: over-funding note shown; Archive moves goal to Achieved; overall % excludes archived; localStorage persists archived:true; Unarchive restores to active."
    );
  }
} finally {
  await browser.close();
}
