// budgets.spec.mjs — regressions for the budgets redesign: Transactions/Notes/Formulas
// in the kebab, the enhanced top-up (this-month vs permanent + fund-from-budgets), the
// notes modal, the copyable formulas modal, the sort picker, and the 50/30/20 template
// living inside the Add-budget modal.
import { test, expect, nav } from "./fixtures.mjs";

async function firstKebab(app) {
  const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
  await kebab.scrollIntoViewIfNeeded();
  const bid = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
  await kebab.click();
  return bid;
}

test.describe("budgets: kebab + row actions", () => {
  test("Transactions, Notes, and Formulas live in the ⋯ menu (not inline)", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstKebab(app);
    await expect(app.locator(`[data-testid="budget-view-txns-${bid}"]`)).toBeVisible();
    await expect(app.locator(`[data-testid="budget-notes-btn-${bid}"]`)).toBeVisible();
    await expect(app.locator(`[data-testid="budget-formulas-btn-${bid}"]`)).toBeVisible();
    // No inline Transactions button in the card footer.
    await expect(app.locator(`.budget-actions > [data-testid="budget-view-txns-${bid}"]`)).toHaveCount(0);
  });
});

test.describe("budgets: enhanced top-up", () => {
  test("top-up offers this-month vs permanent + a fund-from-budgets checklist", async ({ app }) => {
    await nav(app, "/budgets");
    const topup = app.locator('[data-testid^="budget-topup-btn-"]').first();
    await topup.scrollIntoViewIfNeeded();
    await topup.click();
    await app.waitForTimeout(650); // flip
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("topup-dur-month")).toBeVisible();
    await expect(dialog.getByTestId("topup-dur-perm")).toBeVisible();
    // Default is "this month" — the hint says so.
    await expect(dialog.getByTestId("topup-dur-hint")).toContainText(/this period only/i);
    // Expand the funding checklist — it lists budgets with room to give.
    await dialog.getByTestId("topup-cover-toggle").click();
    await expect(dialog.locator('[data-testid^="topup-src-"]').first()).toBeVisible();
  });
});

test.describe("budgets: notes modal", () => {
  test("adding a note via the kebab modal shows a readable notes line on the card", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstKebab(app);
    await app.locator(`[data-testid="budget-notes-btn-${bid}"]`).click();
    await app.waitForTimeout(650);
    const note = "Trim this once the baby-gear splurge settles — revisit in Q4.";
    await app.locator('[role="dialog"] textarea').first().fill(note);
    await app.getByTestId("budget-notes-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });
    const line = app.locator(`[data-testid="budget-notes-${bid}"]`);
    await expect(line).toBeVisible();
    await expect(line).toContainText("Trim this once");
    await expect(line).toHaveAttribute("aria-expanded", "false");
    await line.click();
    await expect(line).toHaveAttribute("aria-expanded", "true");
  });
});

test.describe("budgets: formulas modal", () => {
  test("shows the budget's variables with copy buttons", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstKebab(app);
    await app.locator(`[data-testid="budget-formulas-btn-${bid}"]`).click();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("budget-formulas")).toBeVisible();
    // The five per-budget variables, each with a copy button.
    await expect(app.locator('[data-testid^="budget-formula-copy-"]')).toHaveCount(5);
    await expect(app.locator('[data-testid^="budget-formula-name-"]').first()).toContainText(/^budget_.*_limit$/);
  });
});

test.describe("budgets: sort + add template", () => {
  // Reads every rendered budget card and derives its overage ($ over the limit) and its
  // distance from the limit line in percentage points (|% used − 100|), from the values
  // the card actually shows — so we assert the REAL on-screen order, not the intent.
  async function derivedOrder(app) {
    return app.evaluate(() =>
      [...document.querySelectorAll(".budget-grid .budget")].map((c) => {
        const amt = (c.querySelector(".budget-amount")?.textContent || "").trim();
        const pct = parseInt(c.querySelector(".budget-pct")?.textContent || "0", 10) || 0;
        const toC = (s) => Math.round(parseFloat((s || "").replace(/[^0-9.\-]/g, "")) * 100) || 0;
        const [spent, limit] = amt.split("/").map(toC);
        return { name: (c.querySelector(".row-desc")?.textContent || "").trim(), over: Math.max(0, spent - limit), dist: Math.abs(pct - 100) };
      }),
    );
  }

  test("over budget sorts by severity (overage ↓); close-to-limit by |% − 100| (↑)", async ({ app }) => {
    await nav(app, "/budgets");
    // Over budget → overage strictly non-increasing down the list (worst overspend first).
    await app.getByTestId("budgets-sort").selectOption("overage");
    await app.waitForTimeout(400);
    const ov = await derivedOrder(app);
    expect(ov.length).toBeGreaterThan(2);
    for (let i = 1; i < ov.length; i++) {
      expect(ov[i - 1].over, `overage not descending at ${i} (${ov[i - 1].name}→${ov[i].name})`).toBeGreaterThanOrEqual(ov[i].over);
    }
    // Close to the limit → |% used − 100| non-decreasing (a 99%/101% budget beats a 200%
    // one, and 0%-used budgets — 100 points away — rank LAST, not first).
    await app.getByTestId("budgets-sort").selectOption("near");
    await app.waitForTimeout(400);
    const nr = await derivedOrder(app);
    for (let i = 1; i < nr.length; i++) {
      expect(nr[i - 1].dist, `|%-100| not ascending at ${i} (${nr[i - 1].name}→${nr[i].name})`).toBeLessThanOrEqual(nr[i].dist);
    }
    // A 0%-used budget must not be first under "close to the limit".
    expect(nr[0].dist).toBeLessThan(100);
  });

  test("the 50/30/20 template lives inside the Add-budget modal", async ({ app }) => {
    await nav(app, "/budgets");
    await app.getByTestId("budgets-add").click();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("budget-add-tmpl")).toBeVisible();
    await expect(app.getByTestId("budgets-template-503020")).toBeVisible();
  });
});
