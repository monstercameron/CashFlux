// todo.spec.mjs — regressions for the to-do page refresh: the search box, the "linked
// to" feature filter, the single-glyph Add button, the full compose modal for sub-tasks,
// and drag-to-reorder in Custom order.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("todo: condition-triggered suggestions", () => {
  test("the Suggested strip proposes tasks; Add creates one and the row leaves", async ({ app }) => {
    await nav(app, "/todo");
    const strip = app.getByTestId("todo-suggest-strip");
    await expect(strip).toBeVisible();
    // The section now sits below the committed list and starts collapsed — expand it.
    await app.getByTestId("todo-suggest-toggle").click();
    const rows = strip.locator('[data-testid^="todo-suggest-"][data-testid*=":"], [data-testid^="todo-suggest-stale"], [data-testid^="todo-suggest-overspent"], [data-testid="todo-suggest-unreviewed"]');
    const before = await rows.count();
    expect(before).toBeGreaterThan(0);
    // Add the first proposal: it becomes a real task and its row leaves the strip.
    const firstAdd = strip.locator('[data-testid^="todo-suggest-add-"]').first();
    const addId = await firstAdd.getAttribute("data-testid");
    await firstAdd.click();
    await expect(app.locator("body")).toContainText(/Task added:/);
    await expect(strip.locator(`[data-testid="${addId}"]`)).toHaveCount(0);
    // Dismiss the next one (if any): quiet notice, row leaves.
    const nextDismiss = strip.locator('[data-testid^="todo-suggest-dismiss-"]').first();
    if (await nextDismiss.count()) {
      const dId = await nextDismiss.getAttribute("data-testid");
      await nextDismiss.click();
      await expect(app.locator(`[data-testid="${dId}"]`)).toHaveCount(0);
    }
  });
});

test.describe("todo: checklist templates", () => {
  test("Month-end close instantiates a parent task with ordered steps", async ({ app }) => {
    await nav(app, "/todo");
    await app.getByTestId("todo-checklists-btn").click();
    await app.getByTestId("todo-checklist-monthend").click();
    // The undoable toast names the checklist and its step count.
    await expect(app.locator("body")).toContainText(/Month-end close .* added with 5 steps/i);
    // The parent appears in the list; search narrows to it and its steps.
    await app.getByTestId("todo-search").fill("Month-end close");
    await expect(app.locator("#main")).toContainText(/Month-end close — [A-Z][a-z]{2} \d{4}/);
    // A couple of the steps exist (nested under the parent).
    await app.getByTestId("todo-search").fill("Reconcile each cash account");
    await expect(app.locator("#main")).toContainText(/Reconcile each cash account to its statement/);
  });
});

test.describe("todo: toolbar", () => {
  test("has a search box, a linked-feature filter, Custom order, and a single-glyph Add", async ({ app }) => {
    await nav(app, "/todo");
    const search = app.getByTestId("filter-toolbar-search");
    await expect(search).toBeVisible();
    await expect(search).toHaveAttribute("role", "searchbox");
    const filters = app.locator(".filter-toolbar .filters-trigger");
    await filters.click();
    const panel = app.locator("#filter-inline-panel");
    await expect(panel).toBeVisible();
    await expect(panel.getByRole("combobox", { name: "Filter by linked feature" })).toBeVisible();
    // Custom order is a real sort option inside Filters.
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
    await app.getByTestId("filter-toolbar-search").fill("nursery");
    const cards = app.locator('[data-testid="task-card"]');
    await expect.poll(() => cards.count()).toBeLessThan(before);
    const after = await cards.count();
    expect(after).toBeGreaterThan(0);
    await app.getByTestId("filter-clear-search").click();
    await expect.poll(() => cards.count()).toBe(before);
  });

  test("the linked-feature filter shows only tasks tied to that feature", async ({ app }) => {
    await nav(app, "/todo");
    await app.locator(".filter-toolbar .filters-trigger").click();
    const panel = app.locator("#filter-inline-panel");
    await expect(panel).toBeVisible();
    await panel.getByRole("combobox", { name: "Filter by linked feature" }).selectOption("goal");
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

test.describe("todo: shared display modes", () => {
  test("list, board and calendar are projections of the same task set", async ({ app }) => {
    await nav(app, "/todo");
    const list = app.getByTestId("todo-view-list");
    const board = app.getByTestId("todo-view-board");
    const calendar = app.getByTestId("todo-view-calendar");
    const taskCards = app.locator('[data-testid="task-card"]');

    await expect(list).toHaveAttribute("aria-pressed", "true");
    const listCount = await taskCards.count();
    expect(listCount).toBeGreaterThan(0);

    await board.click();
    await expect(board).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("todo-board")).toBeVisible();
    await expect(app.locator('[data-testid^="todo-board-card-"]').first()).toBeVisible();

    await calendar.click();
    await expect(calendar).toHaveAttribute("aria-pressed", "true");
    await expect(app.locator(".uical")).toBeVisible();
    await expect(app.getByTestId("todo-cal-title")).toBeVisible();

    await list.click();
    await expect(list).toHaveAttribute("aria-pressed", "true");
    await expect.poll(() => taskCards.count()).toBe(listCount);
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
    // ONE pager, below the list: the 2026-07-19 command-bar realign deliberately
    // retired the top mirror (the command bar anchors the top of the workspace).
    await expect(app.locator(".std-pager")).toHaveCount(1);
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
    const pager = app.locator(".std-pager");
    await expect(pager).toBeVisible();
    // Pick "All" (one page) — the pager (and its size buttons) must NOT disappear under
    // the cursor. Regression: it used to be guarded on page-count, so it vanished on click.
    await pager.locator(".pager-size", { hasText: /^All$/ }).click();
    await app.waitForTimeout(300);
    await expect(app.locator(".std-pager")).toHaveCount(1);
    await expect(app.locator('.std-pager .pager-size', { hasText: /^All$/ })).toHaveAttribute("aria-pressed", "true");
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
