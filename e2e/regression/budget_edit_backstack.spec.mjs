// budget_edit_backstack.spec.mjs — stepping into "Edit what this budget tracks…"
// and back (C521).
//
// Cam described the routing he expected: main page → edit budget → tracked
// categories → edit budget → main page. What happened instead was main page →
// edit → categories → main page, because opening the categories editor CLOSED
// the edit form first. That destroyed the component and every unsaved edit with
// it, so a name typed before stepping in was gone on the way back.
import { test, expect, nav } from "./fixtures.mjs";

// The row's Edit action lives in the ⋯ menu, which does not currently open on
// /budgets (C541 — the row's state changes but the keyed row never repaints).
// Its handler is live, so the test dispatches the click directly; when C541 is
// fixed this can go back through the menu.
async function openBudgetEdit(app) {
  await nav(app, "/budgets");
  const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
  await kebab.waitFor({ state: "attached" });
  const id = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
  await app.evaluate((bid) => {
    document.querySelector(`[data-testid="edit-budget-btn-${bid}"]`).click();
  }, id);
  await expect(app.getByTestId("budget-edit-open-cats")).toBeVisible();
  return id;
}

function nameField(app) {
  return app.locator('[class*="flip"] input[type="text"]').first();
}

test.describe("editing a budget and its tracked categories", { tag: "@prod" }, () => {
  test("stepping into tracked categories keeps the panel and the unsaved edits", async ({ app }) => {
    await openBudgetEdit(app);

    const name = nameField(app);
    await name.fill("CHANGED IN FLIGHT");
    await expect(name).toHaveValue("CHANGED IN FLIGHT");

    await app.getByTestId("budget-edit-open-cats").click();

    // ONE panel, not two. Layering a second FlipPanel gives each its own
    // document-level Escape handler, so a single press closes both and takes the
    // unsaved edit with it.
    await expect(app.getByTestId("budget-cats-back")).toBeVisible();
    await expect(app.getByTestId("budgetcats-search")).toBeVisible();
    expect(await app.locator('[class*="flip-panel"]').count()).toBeLessThanOrEqual(1);

    // Back returns to the form with the in-flight edit intact — the whole point.
    await app.getByTestId("budget-cats-back").click();
    await expect(app.getByTestId("budget-edit-open-cats")).toBeVisible();
    await expect(nameField(app)).toHaveValue("CHANGED IN FLIGHT");
  });

  test("the back affordance says the changes are safe", async ({ app }) => {
    await openBudgetEdit(app);
    await app.getByTestId("budget-edit-open-cats").click();
    const back = app.getByTestId("budget-cats-back");
    await expect(back).toContainText(/back to budget/i);
    // Telling the user their work is still there is the difference between a
    // confident click and a cancelled edit.
    await expect(app.getByTestId("budget-cats-back").locator("..")).toContainText(/still here/i);
  });

  test("closing from the categories page does not reopen onto it", async ({ app }) => {
    await openBudgetEdit(app);
    await app.getByTestId("budget-edit-open-cats").click();
    await expect(app.getByTestId("budget-cats-back")).toBeVisible();

    // Close the whole editor while on the sub-page.
    await app.keyboard.press("Escape");
    await expect(app.getByTestId("budget-cats-back")).toHaveCount(0);

    // Reopening must land on the FORM, not on the sub-page the user was last on —
    // a modal that reopens somewhere you did not ask for reads as a bug.
    await openBudgetEdit(app);
    await expect(app.getByTestId("budget-edit-open-cats")).toBeVisible();
    await expect(app.getByTestId("budget-cats-back")).toHaveCount(0);
  });
});
