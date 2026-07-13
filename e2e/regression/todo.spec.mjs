// todo.spec.mjs — regressions for the to-do page refresh: the search box, the "linked
// to" feature filter, the single-glyph Add button, the full compose modal for sub-tasks,
// and drag-to-reorder in Custom order.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("todo: toolbar", () => {
  test("has a search box, a linked-feature filter, Custom order, and a single-glyph Add", async ({ app }) => {
    await nav(app, "/todo");
    await expect(app.getByTestId("todo-search")).toBeVisible();
    await expect(app.getByTestId("todo-filter-link")).toBeVisible();
    // Custom order is a real sort option.
    await expect(app.locator('[data-testid="todo-sort"] option[value="manual"]')).toHaveCount(1);
    // The Add button carries exactly one glyph and no literal "+" in its text.
    const add = app.getByTestId("todo-add");
    expect(await add.locator("svg").count()).toBe(1);
    await expect(add).not.toContainText("+");
  });

  test("search narrows the list; clearing restores it", async ({ app }) => {
    await nav(app, "/todo");
    const before = await app.locator('[data-testid="task-card"]').count();
    expect(before).toBeGreaterThan(3);
    await app.getByTestId("todo-search").fill("nursery");
    await app.waitForTimeout(300);
    const after = await app.locator('[data-testid="task-card"]').count();
    expect(after).toBeLessThan(before);
    expect(after).toBeGreaterThan(0);
    await app.getByTestId("todo-search-clear").click();
    await app.waitForTimeout(300);
    expect(await app.locator('[data-testid="task-card"]').count()).toBe(before);
  });

  test("the linked-feature filter shows only tasks tied to that feature", async ({ app }) => {
    await nav(app, "/todo");
    await app.getByTestId("todo-filter-link").selectOption("goal");
    await app.waitForTimeout(300);
    const cards = app.locator('[data-testid="task-card"]');
    const n = await cards.count();
    expect(n).toBeGreaterThan(0);
    // Every visible task shows a goal link chip (data-testid task-link-*, class is-goal).
    for (let i = 0; i < n; i++) {
      await expect(cards.nth(i).locator(".todo-link.is-goal")).toHaveCount(1);
    }
  });
});

test.describe("todo: sub-task modal", () => {
  test("Add sub-task opens the full compose form (not a bare prompt)", async ({ app }) => {
    await nav(app, "/todo");
    const tid = await app.locator('[data-testid="task-card"]').first().getAttribute("id");
    await app.getByTestId(`task-menu-btn-${tid}`).click();
    await app.getByTestId(`task-addsub-${tid}`).click();
    await app.waitForTimeout(750);
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog).toContainText(/new sub-task/i);
    // The full composer: title field + priority segments + due + the submit.
    await expect(dialog.getByTestId("task-add-form")).toBeVisible();
    await expect(dialog.getByTestId("task-prio-high")).toBeVisible();
    await expect(dialog.getByTestId("task-add-submit")).toBeVisible();
  });
});

test.describe("todo: standard pager", () => {
  test("mirrors top+bottom with rows-per-page and jump-to-page", async ({ app }) => {
    await nav(app, "/todo");
    // Two pagers (top + bottom) when the list spans multiple pages.
    await expect(app.locator(".std-pager")).toHaveCount(2);
    const topPager = app.locator(".std-pager").first();
    // Rows-per-page: switch to 10 → the range + page count update.
    await topPager.locator(".pager-size", { hasText: /^10$/ }).click();
    await app.waitForTimeout(300);
    await expect(app.getByTestId("todo-range").first()).toContainText(/1.10 of \d+/);
    // Jump to page 2 → the window advances to items 11–20.
    const jump = app.getByTestId("todo-jump").first();
    await jump.fill("2");
    await jump.press("Enter");
    await app.waitForTimeout(300);
    await expect(app.getByTestId("todo-range").first()).toContainText(/11.20 of \d+/);
  });

  test("the rows-per-page picker stays put when a bigger size collapses to one page", async ({ app }) => {
    await nav(app, "/todo");
    const top = app.locator(".std-pager-top");
    await expect(top).toBeVisible();
    // Pick "All" (one page) — the top pager (and its size buttons) must NOT disappear under
    // the cursor. Regression: it used to be guarded on page-count, so it vanished on click.
    await top.locator(".pager-size", { hasText: /^All$/ }).click();
    await app.waitForTimeout(300);
    await expect(app.locator(".std-pager-top")).toHaveCount(1);
    await expect(app.locator('.std-pager-top .pager-size', { hasText: /^All$/ })).toHaveAttribute("aria-pressed", "true");
  });
});

test.describe("todo: drag-to-reorder", () => {
  // Native HTML5 drag-and-drop can't be reliably simulated in Playwright (verified: dragTo,
  // synthetic DragEvents, and stepped real-mouse drags all fail to drive the browser's native
  // drag here), so this covers the observable wiring: Custom order arms draggable grip handles
  // on every row, and other sort modes don't. The reorder ITSELF is unit-tested exhaustively in
  // internal/tasksort (Reorder) and mirrors the shipped custom-page widget reorder.
  test("Custom order arms draggable grips on every row; other sorts don't", async ({ app }) => {
    await nav(app, "/todo");
    // Smart order: no grips.
    await app.getByTestId("todo-sort").selectOption("smart");
    await app.waitForTimeout(300);
    await expect(app.locator('[data-testid^="task-grip-"]')).toHaveCount(0);

    // Custom order: a draggable grip on every visible row.
    await app.getByTestId("todo-sort").selectOption("manual");
    await app.waitForTimeout(300);
    const grips = app.locator('[data-testid^="task-grip-"]');
    const rows = app.locator('[data-testid="task-card"]');
    const nGrips = await grips.count();
    expect(nGrips).toBeGreaterThan(2);
    expect(nGrips).toBe(await rows.count());
    await expect(grips.first()).toHaveAttribute("draggable", "true");
  });
});
