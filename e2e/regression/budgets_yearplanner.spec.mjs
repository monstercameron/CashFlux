// budgets_yearplanner.spec.mjs — C608: the year planner has to be usable without
// horizontal scrolling, and every cell has to describe itself.
//
// The grid was twelve columns wide with one affordance for the rest: an
// aria-hidden "Scroll sideways for the full year →" pointing at a scrollbar under
// a tall table. Its cells were buttons whose accessible name was two bare amounts
// ("$1,100.00 $1,300.00") — no month, no budget, no way to tell which was which.
import { test, expect, nav, openVia } from "./fixtures.mjs";

async function openPlanner(app) {
  const section = app.locator(".budget-annualgrid").first();
  await section.scrollIntoViewIfNeeded();
  await openVia(app, section.locator("button").first(), app.getByTestId("annualgrid-window"));
}

async function monthColumns(app) {
  return app.locator(".budget-annualgrid-table thead th").evaluateAll((els) =>
    els.map((e) => e.textContent.trim()).filter((t) => /^[A-Z][a-z]{2}$/.test(t)),
  );
}

test.describe("budgets: year planner", () => {
  test("every cell names its budget, its month, and which figure is which", async ({ app }) => {
    await nav(app, "/budgets");
    await openPlanner(app);

    const cell = app.locator('[data-testid^="annualgrid-cell-"]').first();
    const aria = await cell.getAttribute("aria-label");
    expect(aria, "a cell must carry an accessible name").toBeTruthy();
    // Budget, month + year, and the two figures labelled.
    expect(aria).toMatch(/, \w+ \d{4}: /);
    expect(aria).toMatch(/(spent of|projected against) .* planned/);
    // Its column header announces the full month, which the cell's label refers to.
    const header = app.locator(".budget-annualgrid-table thead th").nth(1);
    await expect(header).toHaveAttribute("aria-label", /^\w+ \d{4}$/);
  });

  test("a half-year window removes the horizontal scroll entirely", async ({ app }) => {
    await nav(app, "/budgets");
    await openPlanner(app);

    expect(await monthColumns(app), "the default is the whole year").toHaveLength(12);

    await app.getByTestId("annualgrid-win-h2").click();
    await expect(app.getByTestId("annualgrid-win-h2")).toHaveAttribute("aria-pressed", "true");
    const half = await monthColumns(app);
    expect(half).toHaveLength(6);
    expect(half[0]).toBe("Jul");
    expect(half[5]).toBe("Dec");

    // The point of the control: no sideways scrolling left to do.
    const fits = await app.evaluate(() => {
      const f = document.querySelector(".budget-annualgrid-scroll");
      return f ? f.scrollWidth <= f.clientWidth + 1 : false;
    });
    expect(fits, "a half-year must fit its frame without horizontal scroll").toBe(true);

    // And the other half is one keyboard-reachable click away.
    await app.getByTestId("annualgrid-win-h1").click();
    const first = await monthColumns(app);
    expect(first[0]).toBe("Jan");
    expect(first[5]).toBe("Jun");
  });
});
