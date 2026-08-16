// budget_direction.spec.mjs — budgets that track saving as well as spending (C538).
//
// Cam: "budgets need to also track income cats for example savings or
// investments". Every budget until now was a spending CAP — less is better and
// passing the number is the failure — so a savings budget needed the opposite
// reading, not just a different picker.
//
// The polarity itself is covered by unit tests in internal/budgeting
// (direction_test.go), including the C539 rule that a transfer is not spending
// but IS saving. These cover the part only a browser can: that the choice exists,
// that it defaults to the historical meaning, and that it changes which
// categories a budget may track.
import { test, expect, nav } from "./fixtures.mjs";

async function openAddBudget(app) {
  await nav(app, "/budgets");
  await app.getByTestId("budgets-add").click();
  await expect(app.getByTestId("budget-direction")).toBeVisible();
}

test.describe("what a budget measures", () => {
  test("a budget is a spending cap unless you say otherwise", async ({ app }) => {
    await openAddBudget(app);
    // The zero value IS the historical spending cap, so every budget in every
    // existing dataset keeps its meaning with no migration.
    await expect(app.getByTestId("budget-direction")).toHaveValue("");
    await expect(app.getByTestId("budget-direction-hint")).toHaveCount(0);
  });

  test("choosing Saving explains what will count toward it", async ({ app }) => {
    await openAddBudget(app);
    await app.getByTestId("budget-direction").selectOption("save");

    // Transfers between your own accounts are the ordinary way money reaches
    // savings, and they are excluded from spending budgets — so a savings budget
    // has to say that they count here, or it reads as permanently stuck at zero.
    const hint = app.getByTestId("budget-direction-hint");
    await expect(hint).toBeVisible();
    await expect(hint).toContainText(/transfer/i);
  });

  test("a saving budget may track income categories; a spending budget may not", async ({ app }) => {
    await openAddBudget(app);
    const picker = app.getByTestId("budget-existing-cat");
    const spending = await picker.locator("option").count();
    expect(spending).toBeGreaterThan(0);

    await app.getByTestId("budget-direction").selectOption("save");
    await expect
      .poll(async () => await picker.locator("option").count(), { timeout: 15000 })
      .toBeGreaterThan(spending);

    // ...and switching back re-restricts it. A spending budget tracking an income
    // category can never accrue — matchesScope wants an outflow — so offering one
    // would ship a budget broken by construction.
    await app.getByTestId("budget-direction").selectOption("");
    await expect.poll(async () => await picker.locator("option").count(), { timeout: 15000 }).toBe(spending);
  });

  test("the picker stops apologising once income categories are allowed", async ({ app }) => {
    await openAddBudget(app);
    const adv = app.getByTestId("budget-add-advanced");
    await adv.click();
    // A spending budget withholds them and says so...
    await expect(app.getByTestId("budgetcats-kind-note")).toBeVisible();
    // ...a saving budget has nothing to withhold, so the note stands down rather
    // than explaining an absence that is no longer happening.
    await app.getByTestId("budget-direction").selectOption("save");
    await expect(app.getByTestId("budgetcats-kind-note")).toHaveCount(0);
  });
});
