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

// stepPeriodBack clicks the period stepper until the pill actually changes.
//
// A single click is not enough: the page keeps re-rendering briefly after mount,
// and a click landing in that window is discarded — and even when it lands, the
// pill's text has to be re-read AFTER the re-render, not in the same tick. Both
// mistakes look identical (the period "did not move") and both were in the first
// draft of these tests.
async function stepPeriodBack(app, times = 1) {
  const pill = app.getByTestId("period-pill");
  const stepBack = pill.locator("xpath=preceding-sibling::button[1]");
  for (let i = 0; i < times; i++) {
    const before = (await pill.innerText()).trim();
    // Click ONCE, then wait for the change. Clicking on every poll tick queues
    // extra steps, and a late one lands after the value has been read — the
    // period then runs one ahead of everything computed from it, which is a
    // failure that looks exactly like the bug under test.
    let moved = false;
    for (let attempt = 0; attempt < 5 && !moved; attempt++) {
      await stepBack.click().catch(() => {});
      try {
        await expect
          .poll(async () => (await pill.innerText()).trim(), { timeout: 3_000, intervals: [150, 150, 300] })
          .not.toBe(before);
        moved = true;
      } catch {
        // The click was swallowed by a re-render; try again.
      }
    }
    expect(moved, "the period stepper never moved the pill").toBe(true);
  }
  return (await pill.innerText()).trim();
}

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
    const period = await stepPeriodBack(app);

    const rail = app.getByTestId("budgets-issues-rail");
    await rail.scrollIntoViewIfNeeded();
    await openVia(app, rail, app.getByTestId("budgets-issues-detail"));
    const go = app.getByTestId("budgets-resolve-alloc");
    await expect(go).toBeVisible();
    await go.click();
    await expect(app.locator('#main[data-route="/allocate"]')).toBeVisible();

    // Back to Budgets the way a user can: the rail link. Deliberately NOT a
    // synthetic popstate — after an in-app navigation the router ignores one
    // entirely (filed as C610, a browser-Back defect found by this very test),
    // and a spec about the budget period should fail on the period, not on that.
    await app.getByRole("link", { name: "Budgets" }).first().click({ force: true });
    await app.waitForSelector('#main[data-route="/budgets"]', { timeout: 45_000 });
    expect((await pill.innerText()).trim(), "the budget period must survive the round trip").toBe(period);
  });
});

test.describe("budgets: supporting modules follow the selected period", () => {
  // C607: the unbudgeted strip and the tracking editor read time.Now() and
  // labelled the result "this month" whatever period the page was showing, so a
  // closed July view listed August's figures under a caption promising July's.
  test("the unbudgeted strip reports the viewed period and says which", async ({ app }) => {
    await nav(app, "/budgets");
    const pill = app.getByTestId("period-pill");
    const hint = app.getByTestId("budgets-unbudgeted-hint");
    const strip = app.getByTestId("budgets-unbudgeted");
    await strip.scrollIntoViewIfNeeded();

    const current = (await pill.innerText()).trim();
    await expect(hint).toContainText(current);
    await expect(hint).not.toContainText(/this month/i);
    const chipsNow = (await strip.innerText()).replace(/\s+/g, " ");

    // Page back far enough that the household's spending genuinely differs.
    const past = await stepPeriodBack(app, 2);
    expect(past, "the pill should have moved").not.toBe(current);

    await strip.scrollIntoViewIfNeeded();
    await expect(hint).toContainText(past);
    // A closed period is described in the past tense, not as a live invitation.
    await expect(hint).toContainText(/had spending/i);
    expect((await strip.innerText()).replace(/\s+/g, " "), "the figures must change with the period").not.toBe(chipsNow);
  });

  test("the tracking editor's caption names the period its counts cover", async ({ app }) => {
    await nav(app, "/budgets");
    const period = await stepPeriodBack(app);

    const kebab = app.getByTestId("budget-kebab-bud-transport");
    await kebab.scrollIntoViewIfNeeded();
    const cats = app.locator('.add-menu:not(.hidden-menu) [data-testid="edit-budget-cats-btn-bud-transport"]');
    await openVia(app, kebab, cats);
    await cats.click();

    const caption = app.getByTestId("budgetcats-metahint");
    await expect(caption).toBeVisible();
    await expect(caption).toContainText(period);
    await expect(caption).not.toContainText(/this month/i);
  });
});

test.describe("budgets: recurring dates say what they are relative to", () => {
  // C609: every row read "Next <date>" whether the date had passed, fell inside
  // the period on screen, or belonged to the schedule's future — three different
  // facts under one word, on a page whose whole job is period-scoped figures.
  test("each recurring date names its state, and only overdue is toned", async ({ app }) => {
    await nav(app, "/budgets");
    const strip = app.getByTestId("budgets-recurring");
    await strip.scrollIntoViewIfNeeded();

    // No bare "Next <date>" survives: each date carries its relationship.
    await expect(strip).not.toContainText(/·\s*Next \w{3} \d/);
    await expect(strip).toContainText(/(Was due|Due|Next due .*after this period)/);

    // The states are exposed structurally, so a wording change cannot quietly
    // collapse three meanings back into one.
    const states = await app.evaluate(() => [
      ...new Set([...document.querySelectorAll('[data-testid^="brc-date-"]')].map((e) => e.getAttribute("data-testid"))),
    ]);
    expect(states.length, "at least one classified date").toBeGreaterThan(0);
    for (const s of states) {
      expect(s).toMatch(/^brc-date-(overdue|due-in-period|after-period)$/);
    }
    // Only an overdue date is toned — colouring all three hides the one that asks
    // the user for something.
    const toned = await app.locator(".brc-date.is-overdue").count();
    const overdue = await app.locator('[data-testid="brc-date-overdue"]').count();
    expect(toned).toBe(overdue);
  });
});

test.describe("budgets: the Add-budget advanced surface", () => {
  // C594: it was one undifferentiated pile — a formula handle, a whole category
  // tree, tags, owner, method, rollover and custom fields — and it OPENED with
  // the implementation-oriented variable name, before the user had finished
  // establishing the budget.
  test("the common path is short, and the advanced surface is grouped and explained", async ({ app }) => {
    await nav(app, "/budgets");
    await app.getByTestId("budgets-add").click();
    const form = app.getByTestId("budget-add-form");
    await expect(form).toBeVisible();

    // Closed, the form asks for a budget: a name, what it measures, a category, a
    // limit and a period. The formula handle is not part of that.
    await expect(app.getByTestId("budget-add-varname")).toHaveCount(0);
    await expect(app.getByTestId("budget-add-tags")).toHaveCount(0);

    await app.getByTestId("budget-add-advanced").click();
    // Three named groups, each with a sentence in product language.
    for (const g of ["tracking", "behaviour", "formula"]) {
      const group = app.getByTestId(`budget-add-group-${g}`);
      await expect(group).toBeVisible();
      // A heading AND an explanation — a bare heading would just be a divider.
      expect((await group.innerText()).split("\n").filter(Boolean).length).toBeGreaterThan(1);
    }
    // The formula handle comes LAST, after everything a normal budget needs.
    const order = await app.evaluate(() =>
      [...document.querySelectorAll('[data-testid^="budget-add-group-"]')].map((e) =>
        e.getAttribute("data-testid"),
      ),
    );
    expect(order[order.length - 1]).toBe("budget-add-group-formula");
    // …and it is explained rather than presented as a required field.
    await expect(app.getByTestId("budget-add-group-formula")).toContainText(/most people never need it/i);
  });
});
