// gapfeatures.spec.mjs — e2e regressions for the "competitive gap" feature wave:
// local-first additions (no external paid service) that close gaps vs comps —
// unusual-charge alerts, goal growth projection, cancel/negotiate helper, cash-flow
// forecast + safe-to-spend, tax/investment reports, and assistant voice input.
// Each test drives the seeded dataset (clock pinned to FIXED_NOW by boot) and
// asserts the real result.
import { test, expect, nav, mainText } from "./fixtures.mjs";

test.describe("gap features", () => {
  test("unusual-charge alert: a merchant billing far above its own normal surfaces", async ({ app }) => {
    // The sample seeds a $68 Blue Bottle Coffee charge against a ~$7 baseline; the
    // on-device unusual-charge detector should flag it in the Notification Center.
    await nav(app, "/notifications");
    const text = await mainText(app);
    expect(text).toMatch(/unusual charge at blue bottle coffee/i);
    // The body states the charge vs the payee's typical amount.
    expect(text).toMatch(/\$68\.00/);
    expect(text).toMatch(/usual \$7\.35/i);
  });

  test("goal growth projection: an investment goal shows a growth-adjusted date", async ({ app }) => {
    // The seeded "Trade up to a bigger family home" goal carries a 5% expected
    // annual return, so its card projects a completion date from compounding.
    await nav(app, "/goals");
    const figs = app.getByTestId("goal-figs-goal-house");
    await expect(figs).toBeVisible();
    await expect(figs).toContainText(/projected/i);
    await expect(figs).toContainText(/5% growth/i);
  });

  test("subscription cancel helper: 'remind me' creates a tracked cancellation task", async ({ app }) => {
    await nav(app, "/subscriptions");
    await app.getByRole("button", { name: /remind me/i }).first().click();
    await expect(app.locator("body")).toContainText(/added a reminder to review/i, { timeout: 15_000 });
    // The created task is a guided cancellation checklist on the To-do list.
    await nav(app, "/todo");
    await expect(app.locator("#main")).toContainText(/review subscription/i, { timeout: 15_000 });
  });

  test("bill negotiation helper: 'negotiate' opens a task pre-filled with talking points", async ({ app }) => {
    await nav(app, "/bills");
    await app.locator('[data-testid^="bill-negotiate-"]').first().click();
    const notes = app.locator("textarea.tc-notes").first();
    await expect(notes).toBeVisible();
    await expect(notes).toHaveValue(/competitor/i);
    await expect(notes).toHaveValue(/retention/i);
  });

  test("daily safe-to-spend: the cash runway shows a per-day allowance until next income", async ({ app }) => {
    await nav(app, "/planning");
    const perDay = app.getByTestId("runway-perday");
    await expect(perDay).toBeVisible({ timeout: 15_000 });
    await expect(perDay).toContainText(/\/day until your next income/i);
    await expect(perDay).toContainText(/\(in \d+ days\)/i);
  });

  test("investment performance: the reports appendix shows per-account return", async ({ app }) => {
    await nav(app, "/reports");
    const sec = app.getByTestId("investperf-section");
    await sec.scrollIntoViewIfNeeded();
    await expect(sec).toBeVisible();
    // The three seeded investment accounts (401k, brokerage, Roth) each get a row.
    await expect(app.getByTestId("invperf-row")).toHaveCount(3);
    await expect(app.getByTestId("invperf-total")).toContainText(/across all investments/i);
    await expect(app.getByTestId("invperf-total")).toContainText(/gain/i);
  });
});
