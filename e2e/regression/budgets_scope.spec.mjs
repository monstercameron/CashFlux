// budgets_scope.spec.mjs — the controls that had to start saying what they
// govern: the household budgeting method vs a budget's own (C590), and the
// plan-level rail that counted "items" without naming them or their
// destinations (C588).
import { test, expect, nav, openVia } from "./fixtures.mjs";

// openSettings opens the Budget settings popover, retrying because the tile
// keeps re-rendering briefly after mount and swallows the first click.
//
// The check has to be VISIBILITY, not presence: the popover's contents stay in
// the DOM and are hidden by a class, so a count-based check passes against a
// closed menu and every later assertion then reads a hidden element.
const SETTINGS_MENU = ".bud-set-menu:not(.hidden-menu)";

async function openSettings(app) {
  await app.getByTestId("budgets-settings").scrollIntoViewIfNeeded();
  await openVia(app, app.getByTestId("budgets-settings"), app.locator(SETTINGS_MENU));
}

test.describe("budgets: which method control governs what", () => {
  test("the household picker names its scope and says what it will not touch", async ({ app }) => {
    await nav(app, "/budgets");
    await openSettings(app);

    // The label carries the scope — it used to read just "Budgeting method",
    // identical to the per-budget control in Add/Edit.
    await expect(app.locator(SETTINGS_MENU)).toContainText(/whole household/i);
    // And a sentence saying how far a change reaches. With no per-budget
    // overrides in the sample household, it reaches everything.
    const scope = app.locator(SETTINGS_MENU).getByTestId("budgets-method-scope");
    await expect(scope).toBeVisible();
    await expect(scope).toContainText(/applies to all \d+ budgets/i);
    // Never a formatting artefact: this line takes a different number of
    // arguments in each of its three states.
    await expect(scope).not.toContainText("%!");
  });

  test("a budget's own method says where it currently gets it from", async ({ app }) => {
    await nav(app, "/budgets");
    await app.getByTestId("budgets-add").click();
    await expect(app.getByTestId("budget-add-form")).toBeVisible();
    await app.getByTestId("budget-add-advanced").click();

    const hint = app.getByTestId("budget-add-method-hint");
    await expect(hint).toBeVisible();
    // Following the household by default, and the sentence says the choice here
    // stops at this budget.
    await expect(hint).toContainText(/following the household method/i);
    await expect(hint).toContainText(/this budget only/i);
  });

  test("changing the household method reports what changed and is undoable", async ({ app }) => {
    await nav(app, "/budgets");
    await openSettings(app);
    await app.locator(SETTINGS_MENU).getByTestId("budgets-method").selectOption("zero-based");
    // The toast names the new method AND how many budgets moved — the two facts
    // the old "Budgeting method saved." withheld.
    await expect(app.locator("body")).toContainText(/household method is now .*zero-based/i);
    await expect(app.locator("body")).toContainText(/\d+ budgets? changed/i);
  });
});

test.describe("budgets: the plan rail names its contents", () => {
  test("the header says what is in it, and each row names the page it opens", async ({ app }) => {
    await nav(app, "/budgets");
    const rail = app.getByTestId("budgets-issues-rail");
    await rail.scrollIntoViewIfNeeded();
    // Not "N items to review" — the actual contents.
    await expect(rail).toContainText(/over-assignment|sinking-fund shortfall|follow-ups/i);
    await expect(rail).not.toContainText(/items to review/i);

    await openVia(app, rail, app.getByTestId("budgets-issues-detail"));
    const detail = app.getByTestId("budgets-issues-detail");
    // Every action names its destination page, so the user knows whether they
    // are about to land on Allocate, To-do or Goals before clicking.
    await expect(detail).toContainText(/Open (Allocate|To-do|Goals)/);
  });

  test("returning from a rail destination keeps the same budget period", async ({ app }) => {
    await nav(app, "/budgets");
    const pill = app.getByTestId("period-pill");
    // Step back one period so the test is not merely observing the default.
    const stepBack = pill.locator("xpath=preceding-sibling::button[1]");
    const start = (await pill.innerText()).trim();
    await expect.poll(async () => {
      await stepBack.click().catch(() => {});
      return (await pill.innerText()).trim();
    }, { timeout: 15_000 }).not.toBe(start);
    const period = (await pill.innerText()).trim();

    const rail = app.getByTestId("budgets-issues-rail");
    await rail.scrollIntoViewIfNeeded();
    await openVia(app, rail, app.getByTestId("budgets-issues-detail"));
    const go = app.getByTestId("budgets-resolve-alloc");
    await expect(go).toBeVisible();
    await go.click();
    await expect(app.locator('#main[data-route="/allocate"]')).toBeVisible();

    // Back to Budgets. Deliberately NOT the nav() helper: it asserts the route
    // mounted within the default expect timeout, and /allocate → /budgets on a
    // loaded worker is slower than that. The point of the test is the period, not
    // how fast the page paints.
    await app.evaluate(() => {
      history.pushState({}, "", "/budgets");
      dispatchEvent(new PopStateEvent("popstate"));
    });
    await app.waitForSelector('#main[data-route="/budgets"]', { timeout: 45_000 });
    expect((await pill.innerText()).trim(), "the budget period must survive the round trip").toBe(period);
  });
});
