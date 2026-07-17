// gapfeatures.spec.mjs — e2e regressions for the "competitive gap" feature wave:
// local-first additions (no external paid service) that close gaps vs comps —
// unusual-charge alerts, goal growth projection, cancel/negotiate helper, cash-flow
// forecast + safe-to-spend, tax/investment reports, and assistant voice input.
// Each test drives the seeded dataset (clock pinned to FIXED_NOW by boot) and
// asserts the real result.
import { test, expect, nav, mainText, boot } from "./fixtures.mjs";

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

  test("cash forecast widget: 30/60/90-day projected available cash on the dashboard", async ({ app }) => {
    await nav(app, "/");
    const tile = app.getByTestId("dash-forecast");
    await tile.scrollIntoViewIfNeeded();
    await expect(tile).toBeVisible();
    // Three horizons, each a real money figure, plus the explainer with today's total.
    await expect(tile.getByTestId("forecast-30")).toContainText(/[\d,]+\.\d{2}/);
    await expect(tile.getByTestId("forecast-60")).toContainText(/[\d,]+\.\d{2}/);
    await expect(tile.getByTestId("forecast-90")).toContainText(/[\d,]+\.\d{2}/);
    await expect(tile).toContainText(/available cash accounts/i);
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

  // Uses `page` (not `app`) so a fake Web Speech API can be injected BEFORE boot;
  // Playwright Chromium ships a native SpeechRecognition stub that won't transcribe,
  // so we stub both API names to return a deterministic transcript.
  test("assistant voice input: the mic dictates a question into the composer", async ({ page }) => {
    await page.addInitScript(() => {
      window.SpeechRecognition = window.webkitSpeechRecognition = class {
        start() { const s = this; setTimeout(() => { if (s.onresult) s.onresult({ results: [[{ transcript: "how much did we spend on groceries" }]] }); if (s.onend) s.onend(); }, 30); }
        stop() {}
      };
    });
    await boot(page);
    await nav(page, "/assistant");
    const mic = page.getByTestId("asst-voice-btn");
    await expect(mic).toBeVisible();
    await mic.click();
    await expect(page.locator("#cf-chat-input")).toHaveValue(/how much did we spend on groceries/i, { timeout: 10_000 });
  });

  test("notification deep-link: an unusual-charge alert lands on the ledger pre-searched to that merchant", async ({ app }) => {
    // Clicking a transaction-scoped notification shouldn't just open the ledger —
    // it should land on the exact charge. The unusual Blue Bottle Coffee alert opens
    // /transactions with the search already set to that merchant.
    await nav(app, "/notifications");
    const open = app.locator('[data-testid^="notif-open-default-unusual@"]').first();
    await expect(open).toBeVisible();
    await open.click();
    await expect(app).toHaveURL(/\/transactions/, { timeout: 10_000 });
    const search = app.locator('#main input[type="search"]').first();
    await expect(search).toHaveValue(/blue bottle coffee/i, { timeout: 10_000 });
  });

  test("notification deep-link: an account alert scrolls to and flashes that account's card", async ({ app }) => {
    // A stale/low-balance notification is about one account; clicking it lands on
    // /accounts and pulses that account's own row so the eye finds it immediately.
    await nav(app, "/notifications");
    const acctAlert = app
      .locator('[data-testid^="notif-open-default-stale@"], [data-testid^="notif-open-default-low-balance@"]')
      .first();
    // Only assert the flash when the seed actually produced such an alert.
    if ((await acctAlert.count()) > 0) {
      await acctAlert.click();
      await expect(app).toHaveURL(/\/accounts/, { timeout: 10_000 });
      await expect(app.locator(".acct-row.deeplink-flash").first()).toBeVisible({ timeout: 10_000 });
    }
  });

  test("milestone celebrations: the Notification Center leads with a 'Recent wins' card", async ({ app }) => {
    // The warning-heavy feed gets a positive counterweight: the seeded household has
    // real net worth and kept budgets last month, so at least one win is celebrated.
    await nav(app, "/notifications");
    const wins = app.getByTestId("wins-card");
    await expect(wins).toBeVisible({ timeout: 15_000 });
    await expect(wins).toContainText(/recent wins/i);
    // At least one milestone row is present.
    await expect(app.locator('[data-testid^="wins-row-"]').first()).toBeVisible();
  });

  test("bill → budget-fit chip: an upcoming bill shows whether it fits its budget", async ({ app }) => {
    // The seeded "Streaming & apps" recurring ($38) maps to the Subscriptions budget
    // ($40), and car payments map to Transportation — so at least one bill row shows a
    // fit chip stating whether paying it stays within that budget for the period.
    await nav(app, "/bills");
    const chip = app.locator('[data-testid^="bill-fit-"]').first();
    await expect(chip).toBeVisible({ timeout: 15_000 });
    await expect(chip).toContainText(/fits|over/i);
    // Clicking it deep-links to the budget and flashes that budget's card.
    await chip.click();
    await expect(app).toHaveURL(/\/budgets/, { timeout: 10_000 });
    await expect(app.locator(".budget.deeplink-flash").first()).toBeVisible({ timeout: 10_000 });
  });

  test("budget driver panel: an over budget reveals what's driving it", async ({ app }) => {
    // The seeded Groceries budget is over; its card offers a "What's driving this"
    // disclosure that expands to the largest charges behind the overspend.
    await nav(app, "/budgets");
    const toggle = app.locator('[data-testid^="budget-drivers-toggle-"]').first();
    await expect(toggle).toBeVisible();
    await toggle.scrollIntoViewIfNeeded();
    await toggle.click();
    // At least one driving charge appears (a drill button to that merchant's ledger).
    await expect(app.locator('[data-testid^="budget-driver-"]').first()).toBeVisible({ timeout: 10_000 });
  });
});
