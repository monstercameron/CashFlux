// goals.spec.mjs — regressions for the goals redesign: the decluttered toolbar (sort
// picker, no smart/metrics, single-glyph Add), Edit moved into the ⋯ menu, the top-3
// steps cap + equal card heights, the review-reminder cadence, multi-link accounts/
// budgets, and virtual allocation (earmarking account balances with no transaction).
import { test, expect, nav } from "./fixtures.mjs";

// firstGoalId returns the id of the first rendered goal card (any kind).
async function firstGoalId(app) {
  const card = app.locator('[data-testid^="goal-row-"]').first();
  await card.scrollIntoViewIfNeeded();
  return (await card.getAttribute("data-testid")).replace("goal-row-", "");
}

// firstFinancialGoalId returns the id of the first financial goal card, or null.
async function firstFinancialGoalId(app) {
  const card = app.locator('[data-testid^="goal-row-"][data-kind="financial"]').first();
  if ((await card.count()) === 0) return null;
  await card.scrollIntoViewIfNeeded();
  return (await card.getAttribute("data-testid")).replace("goal-row-", "");
}

test.describe("goals: decluttered toolbar", () => {
  test("has a Sort picker, no smart/metrics controls, and a single-glyph Add button", async ({ app }) => {
    await nav(app, "/goals");
    await expect(app.getByTestId("goals-sort")).toBeVisible();
    // The old smart-hub link and the "Goal metrics" formula toggle are gone from /goals.
    await expect(app.locator('.bento-goals [data-testid="smart-section-action"]')).toHaveCount(0);
    await expect(app.locator('[data-testid="goals-toggle-formulas"]')).toHaveCount(0);
    // The Add-goal button carries exactly ONE glyph (no plus-circle icon AND a "+" text).
    const add = app.getByTestId("goals-add");
    await expect(add).toBeVisible();
    expect(await add.locator("svg").count()).toBe(1);
    await expect(add).not.toContainText("+");
  });

  test("sorting by name orders the active goal cards A→Z", async ({ app }) => {
    await nav(app, "/goals");
    await app.getByTestId("goals-sort").selectOption("name");
    await app.waitForTimeout(300);
    const titles = await app.locator(".goal-list .goal-card .goal-card-title").allInnerTexts();
    expect(titles.length).toBeGreaterThan(1);
    const sorted = [...titles].sort((a, b) => a.localeCompare(b));
    expect(titles).toEqual(sorted);
  });
});

test.describe("goals: card actions moved into the ⋯ menu", () => {
  test("Edit lives in the kebab (not inline) and a financial goal offers Allocate", async ({ app }) => {
    await nav(app, "/goals");
    const gid = await firstGoalId(app);
    // No inline Edit button directly in the card footer.
    await expect(app.locator(`.goal-card-actions > [data-testid="goal-edit-btn-${gid}"]`)).toHaveCount(0);
    // Open ONE menu (prefer a financial goal so we can check Allocate too) — Edit is a menu item.
    const fid = await firstFinancialGoalId(app);
    const target = fid || gid;
    await app.getByTestId(`goal-menu-btn-${target}`).click();
    await expect(app.locator(`.add-menu [data-testid="goal-edit-btn-${target}"]`)).toBeVisible();
    if (fid) {
      await expect(app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`)).toBeVisible();
    }
  });
});

test.describe("goals: steps capped at top 3", () => {
  test("no card shows more than 3 step rows", async ({ app }) => {
    await nav(app, "/goals");
    const lists = app.locator(".goal-todos-list");
    const n = await lists.count();
    for (let i = 0; i < n; i++) {
      expect(await lists.nth(i).locator(".goal-todo").count(), `card ${i} shows >3 steps`).toBeLessThanOrEqual(3);
    }
  });
});

test.describe("goals: edit form — review cadence + multi-link", () => {
  test("the editor exposes a review-reminder picker and account/budget checklists", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.getByTestId(`goal-menu-btn-${fid}`).click();
    await app.locator(`.add-menu [data-testid="goal-edit-btn-${fid}"]`).click();
    await app.waitForTimeout(650); // flip
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("goal-edit-review")).toBeVisible();
    await expect(dialog.getByTestId("goal-link-accts")).toBeVisible();
    await expect(dialog.getByTestId("goal-link-budgets")).toBeVisible();
    // At least one account checkbox exists to link.
    await expect(dialog.locator('[data-testid^="goal-link-acct-"]').first()).toBeVisible();
  });
});

test.describe("goals: virtual allocation", () => {
  test("a master toggle + selectable accounts earmark balances, showing coverage + a status badge", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");

    await app.getByTestId(`goal-menu-btn-${fid}`).click();
    await app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`).click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    // The master toggle controls the account list: off → no rows, on → the picker appears.
    // (The goal may already have seed earmarks, so normalize to off first.)
    const toggle = dialog.getByTestId("goal-alloc-toggle");
    await expect(toggle).toBeVisible();
    await toggle.uncheck();
    await app.waitForTimeout(200);
    await expect(dialog.locator('[data-testid^="goal-alloc-pick-"]')).toHaveCount(0);
    await toggle.check();
    await app.waitForTimeout(300);
    // Now a selectable list of accounts appears; select the first + set an amount.
    await expect(dialog.locator('[data-testid^="goal-alloc-pick-"]').first()).toBeVisible();
    await dialog.locator('[data-testid^="goal-alloc-pick-"]').first().check();
    await app.waitForTimeout(200);
    await dialog.locator('input[data-testid^="goal-alloc-acct"]').first().fill("500");
    await expect(dialog.getByTestId("goal-alloc-summary")).toContainText(/covered/i);
    await dialog.getByTestId("goal-alloc-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    // The card now shows the earmarked/coverage line AND a "partly/fully earmarked" badge.
    await expect(app.locator(`[data-testid="goal-earmarked-${fid}"]`)).toContainText(/earmarked/i);
    await expect(app.locator(`[data-testid="goal-earmark-status-${fid}"]`)).toContainText(/earmarked/i);
  });

  test("smart split fills per-account amounts that sum to the entered total", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.getByTestId(`goal-menu-btn-${fid}`).click();
    await app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`).click();
    await app.waitForTimeout(650);
    const d = app.locator('[role="dialog"]');
    await d.getByTestId("goal-alloc-toggle").check();
    await app.waitForTimeout(200);
    await d.getByTestId("goal-alloc-total").fill("1000");
    await d.getByTestId("goal-alloc-split-prop").click();
    await app.waitForTimeout(250);
    const vals = await d.locator(".goal-alloc-input").evaluateAll((els) => els.map((e) => parseFloat(e.value) || 0));
    const total = vals.reduce((a, b) => a + b, 0);
    // The seed's liquid accounts hold far more than $1,000, so the split sums to exactly it.
    expect(Math.abs(total - 1000)).toBeLessThan(0.05);
  });

  test("the Earmarks tab lists exposure + per-goal earmarks and deletes a row", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    // Create earmarks: split $500 evenly across the liquid accounts (several rows).
    await app.getByTestId(`goal-menu-btn-${fid}`).click();
    await app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`).click();
    await app.waitForTimeout(650);
    const d = app.locator('[role="dialog"]');
    await d.getByTestId("goal-alloc-toggle").check();
    await app.waitForTimeout(200);
    await d.getByTestId("goal-alloc-total").fill("500");
    await d.getByTestId("goal-alloc-split-even").click();
    await app.waitForTimeout(200);
    await d.getByTestId("goal-alloc-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    // Switch to the Earmarks tab.
    await app.getByTestId("goals-tab-earmarks").click();
    await app.waitForTimeout(400);
    await expect(app.getByTestId("goals-tab-earmarks")).toHaveClass(/is-active/);
    await expect(app.locator(".ea-exp-list")).toBeVisible();
    await expect(app.locator(`[data-testid="ea-goal-${fid}"]`)).toBeVisible();

    // Delete one earmark row → that goal shows one fewer row.
    const before = await app.locator(`[data-testid="ea-goal-${fid}"] .ea-row`).count();
    expect(before).toBeGreaterThan(1);
    await app.locator(`[data-testid^="ea-del-${fid}-"]`).first().click();
    await app.waitForTimeout(400);
    await expect(app.locator(`[data-testid="ea-goal-${fid}"] .ea-row`)).toHaveCount(before - 1);
  });

  test("the allocate picker lists only liquid cash accounts (no debts, 401k, property, brokerage)", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.getByTestId(`goal-menu-btn-${fid}`).click();
    await app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`).click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    await dialog.getByTestId("goal-alloc-toggle").check();
    await app.waitForTimeout(300);
    const names = await dialog.locator(".goal-alloc-acct").allInnerTexts();
    expect(names.length).toBeGreaterThan(0);
    // You can only earmark spendable cash — not liabilities, retirement, property, or brokerage.
    for (const n of names) {
      expect(n.toLowerCase()).not.toMatch(/credit card|car loan|loan|mortgage|401|retirement|condo|property|stonks|brokerage|investment/);
    }
  });
});
