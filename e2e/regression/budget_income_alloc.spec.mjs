// budget_income_alloc.spec.mjs — the /budgets income-allocation read and the
// add-a-budget category opt-in.
//
// These exist because of a specific report: Cam clicked "Budget income", chose
// his income category, saved, and nothing on the page changed. The basis fed the
// hero band's denominator only under the zero-based method, and the default
// method is Simple. The tests below are the guard that the control always
// changes something a person can see.
import { test, expect, nav } from "./fixtures.mjs";

async function openBudgets(app) {
  await nav(app, "/budgets");
  await expect(app.getByTestId("budgets-status-strip")).toBeVisible();
}

test.describe("income allocation read", () => {
  test("the hero answers 'how much of my income is budgeted' on the default method", async ({ app }) => {
    await openBudgets(app);

    // The sample household is on the default (Simple) method — the method whose
    // hero never reflected the income basis at all.
    const alloc = app.getByTestId("budgets-income-alloc");
    await expect(alloc).toBeVisible();

    // A percentage, and the figure it is a percentage OF, adjacent to it. The
    // first draft put the denominator at the far right edge of the caption,
    // which left "162%" reading as "162% of what?".
    await expect(app.getByTestId("budgets-alloc-pct")).toHaveText(/^\d+%$/);
    await expect(app.getByTestId("budgets-alloc-denom")).toContainText(/of your \$[\d,]+\.\d\d/);
    await expect(app.getByTestId("budgets-alloc-relation")).not.toBeEmpty();
  });

  test("the state, the bar and the words agree with each other", async ({ app }) => {
    await openBudgets(app);
    const alloc = app.getByTestId("budgets-income-alloc");
    const state = await alloc.getAttribute("data-state");
    expect(["under", "over", "exact", "empty"]).toContain(state);

    if (state === "over") {
      // Over income: a striped segment past the tick, a tick, and copy that says
      // so. A bar rendered entirely in the healthy accent under an "over" caption
      // is the exact failure this guards — the reader believes the colour.
      await expect(app.getByTestId("budgets-alloc-overflow")).toBeVisible();
      await expect(app.getByTestId("budgets-alloc-marker")).toBeVisible();
      await expect(app.getByTestId("budgets-alloc-relation")).toContainText(/more than (you earn|was earned)/);
      // ...and the plan must genuinely exceed 100%.
      const pct = parseInt((await app.getByTestId("budgets-alloc-pct").innerText()).replace("%", ""), 10);
      expect(pct).toBeGreaterThan(100);
    } else if (state === "under") {
      // Under income: no overflow, and no tick — the end of the track already IS
      // the income, so a tick at 100% would mark a boundary the edge draws.
      await expect(app.getByTestId("budgets-alloc-overflow")).toHaveCount(0);
      await expect(app.getByTestId("budgets-alloc-marker")).toHaveCount(0);
    }
  });

  test("an over-income plan qualifies the hero figure instead of contradicting it", async ({ app }) => {
    await openBudgets(app);
    const state = await app.getByTestId("budgets-income-alloc").getAttribute("data-state");
    test.skip(state !== "over", "sample household is not over-income");

    // The largest, greenest number on the page is money left in a budget that was
    // never affordable. Unqualified it reads as slack, so the caveat has to travel
    // with the figure rather than sit in smaller type further down.
    const caveat = app.getByTestId("budgets-hero-over-income");
    await expect(caveat).toBeVisible();
    await expect(caveat).toContainText(/over income/);
    await expect(app.getByTestId("budgets-status-strip")).toContainText(/Left in budget/i);
  });

  test("changing the income basis is reachable from the figure it changes", async ({ app }) => {
    await openBudgets(app);
    await expect(app.getByTestId("budgets-income-alloc")).toBeVisible();

    // The control lives on the thing it controls — inside the allocation caption
    // rather than adrift in the meta row — and it is the SAME labelled button, not
    // a second quieter one, so there is exactly one way to do this.
    const change = app.getByTestId("budgets-basis-open");
    await expect(change).toBeVisible();
    await expect(change).toContainText(/income/i);
    await expect(app.getByTestId("budgets-income-alloc").getByTestId("budgets-basis-open")).toBeVisible();
    await change.click();
    await expect(app.locator(".zbb-basis-modal")).toBeVisible();
  });

  test("what the basis modal previews is what the page then uses", async ({ app }) => {
    // C531: the modal anchored its running total on today while the page anchored
    // on the viewed window, so the figure approved in the modal was averaged over
    // different months than the figure the page applied. Both now resolve through
    // one helper — this drives the whole journey rather than reading two numbers
    // off an unset basis, which took the page down a different code path entirely.
    await openBudgets(app);
    await app.getByTestId("budgets-basis-open").click();
    const modal = app.locator(".zbb-basis-modal");
    await expect(modal).toBeVisible();

    // Choose the by-source basis so the modal shows its running total, then
    // include every source so the figure is deterministic.
    await modal.getByRole("combobox").first().selectOption("categories");
    const includeAll = modal.getByRole("button", { name: /include all/i });
    if (await includeAll.count()) await includeAll.click();

    const previewed = (await app.getByTestId("budgets-zbb-sources-total").innerText()).trim();
    expect(previewed).toMatch(/^\$[\d,]+\.\d\d$/);

    // Save and read the page's own denominator back.
    const save = app.getByRole("button", { name: /^(save|done|apply)$/i }).last();
    await save.click();
    await expect(modal).toHaveCount(0);

    await expect
      .poll(async () => (await app.getByTestId("budgets-alloc-denom").innerText()).trim())
      .toContain(previewed);
  });
});

test.describe("adding a budget no longer mints a category behind your back", () => {
  async function openAddBudget(app) {
    await nav(app, "/budgets");
    const add = app.getByTestId("budgets-add-btn").or(app.getByRole("button", { name: /add budget/i })).first();
    await add.click();
    await expect(app.getByTestId("budget-create-cat-label")).toBeVisible();
  }

  test("creating a category is opt-in, and the choice is visible without opening Advanced", async ({ app }) => {
    await openAddBudget(app);

    // The whole defect was that "create a new category" was the DEFAULT and it
    // lived behind the Advanced disclosure, so the common path minted a category
    // named after the budget with nothing on screen offering the alternative.
    await expect(app.getByTestId("budget-create-cat")).not.toBeChecked();
    await expect(app.getByTestId("budget-existing-cat")).toBeVisible();
    await expect(app.getByTestId("budget-new-cat-name")).toHaveCount(0);
  });

  test("ticking the opt-in reveals the name field and states the outcome", async ({ app }) => {
    await openAddBudget(app);
    await app.getByTestId("budget-create-cat").check();

    await expect(app.getByTestId("budget-new-cat-name")).toBeVisible();
    await expect(app.getByTestId("budget-existing-cat")).toHaveCount(0);

    // Naming it after a category that already exists REUSES that category rather
    // than minting a twin, and the form has to say so before Add is pressed.
    const existing = await app
      .getByTestId("budget-existing-cat")
      .count()
      .catch(() => 0);
    expect(existing).toBe(0);
    await app.getByTestId("budget-new-cat-name").fill("Groceries");
    await expect(app.getByTestId("budget-cat-fate")).toBeVisible();
  });

  test("a name that already exists is reused, not duplicated", async ({ app }) => {
    await openAddBudget(app);

    // Read a real category name straight out of the picker.
    const firstOption = (await app.getByTestId("budget-existing-cat").locator("option").first().innerText()).trim();
    await app.getByTestId("budget-create-cat").check();

    // Case and spacing must not defeat the match — "groceries" is "Groceries",
    // and "Eating  Out" with a double space is "Eating Out". The match also has
    // to span the whole TREE: parent-scoping alone let a second, top-level copy
    // of a nested category through, which is a duplicate to everyone but the code.
    await app.getByTestId("budget-new-cat-name").fill(firstOption.toLowerCase());
    await expect(app.getByTestId("budget-cat-fate")).toContainText(/existing/i);
    await expect(app.getByTestId("budget-cat-fate")).not.toContainText(/create/i);
  });
});
