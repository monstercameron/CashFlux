// txn_audit.spec.mjs — the Transactions-page audit expansion (C560–C572).
//
// Each test names the ticket it locks down and asserts the ticket's OWN
// acceptance criterion, not the implementation that satisfies it today. Where a
// ticket's failure was "a control reported a state it did not apply", the test
// reads BOTH the control and the thing it claims to control, in the same moment
// — that disagreement is the whole bug, and a test that reads only the label
// would have passed against the broken build.
import { test, expect, nav, openVia, settle } from "./fixtures.mjs";

// The suite's pinned clock (fixtures.FIXED_NOW) is 2026-07-01, so "this month"
// is July 2026 and the month before it is June 2026 everywhere below.
const THIS_MONTH = /July 2026/i;
const PREV_MONTH = /June 2026/i;

// dialog helpers — the app's in-page confirm (never window.confirm).
const dialog = (app) => app.locator(".cf-dialog");
const dialogConfirm = (app) => app.locator("#cf-dialog-confirm");
const dialogCancel = (app) => app.locator("#cf-dialog-cancel");

// A CLOSED overflow menu stays in the DOM (it wears `hidden-menu`), so "the item
// exists" is not "the menu is open" — every popover assertion here keys off the
// open menu, never off an item's presence.
const openMenuIn = (scope) => scope.locator(".add-menu:not(.hidden-menu)");

// openMenu clicks a ⋯ trigger until ITS menu is actually open. openVia retries the
// click because a freshly-mounted screen can still be re-rendering, and a click
// landing in that window sets state on a tree about to be replaced.
async function openMenu(app, scope, trigger) {
  await openVia(app, trigger, openMenuIn(scope));
  return openMenuIn(scope);
}

// openRowMenu opens a ledger row's ⋯ menu and returns the row locator. It picks a
// row from the middle of the page (clear of the sticky chrome) and skips transfer
// legs, which deliberately omit the category-bearing entries.
async function openRowMenu(app, { needs = "txn-split-open" } = {}) {
  for (let i = 4; i < 14; i++) {
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(i);
    await row.scrollIntoViewIfNeeded();
    const menu = await openMenu(app, row, row.locator('[data-testid^="txn-kebab-"]'));
    if (await menu.locator(`[data-testid="${needs}"]`).count()) return row;
    await app.keyboard.press("Escape");
    await expect(openMenuIn(row)).toHaveCount(0);
  }
  throw new Error(`no ledger row offered ${needs}`);
}

// clickRowMenuItem opens a row's menu and activates one of its entries.
async function clickRowMenuItem(app, row, testid) {
  const menu = await openMenu(app, row, row.locator('[data-testid^="txn-kebab-"]'));
  await menu.locator(`[data-testid="${testid}"]`).click();
}

// clickToolbarMenuItem does the same for the toolbar's "⋯ More" overflow.
async function clickToolbarMenuItem(app, testid) {
  const wrap = app.locator('.add-wrap:has([data-testid="txn-more-btn"])');
  const menu = await openMenu(app, wrap, app.getByTestId("txn-more-btn"));
  await menu.locator(`[data-testid="${testid}"]`).click();
}

// selectRow ticks a row's bulk checkbox and waits for the bulk bar to mount.
//
// NOT openVia: that helper retries the CLICK until its check appears, which is
// right for an idempotent "open" button and wrong for a toggle — a slow frame
// makes the second click deselect the row, and the bulk bar never arrives. The
// checkbox's own checked state is the idempotency guard here, so the click is
// repeated only while the box is actually clear.
async function selectRow(app, row) {
  const box = row.locator("input[type=checkbox]");
  await expect
    .poll(async () => {
      if (!(await box.isChecked())) await box.click().catch(() => {});
      return box.isChecked();
    }, { timeout: 20_000 })
    .toBe(true);
  await expect(app.getByTestId("bulk-category-select")).toBeVisible();
}

// rowDates reads the visible rows' date cells, which is how a test can tell that
// the ROWS moved and not merely the label above them.
async function rowDates(app) {
  return app.evaluate(() =>
    [...document.querySelectorAll('tr[data-testid^="txn-row-"]')]
      .map((r) => r.querySelector("td:nth-child(2)")?.textContent.trim())
      .filter(Boolean),
  );
}

test.describe("C560 — one period across the label, the rows and the calendar", () => {
  test("the ledger owns its period, and the top bar's reporting pill is gone", async ({ app }) => {
    await nav(app, "/transactions");
    // The pill could not represent a single day or a hand-typed range, so it could
    // never agree with this page. It belongs to the aggregate surfaces only.
    await expect(app.getByTestId("period-pill")).toHaveCount(0);
    await expect(app.getByTestId("txn-scopebar")).toBeVisible();
    // Unbounded by default, and it says so rather than implying a month.
    await expect(app.getByTestId("txn-scope-label")).toHaveText(/all dates/i);
    // The dashboard still has the pill — this is a scoping change, not a removal.
    await nav(app, "/");
    await expect(app.getByTestId("period-pill")).toBeVisible();
  });

  test("stepping the period moves the label, the rows, the chips and the calendar together", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-scope-prev").click();
    // The label names a whole month...
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    // ...and EVERY visible row falls inside it. This is the assertion the broken
    // build failed: the label said July while the rows were August.
    await expect
      .poll(async () => (await rowDates(app)).every((d) => /Jun\b/.test(d)), { timeout: 15_000 })
      .toBe(true);
    // The date filter carries the same month, as removable chips.
    await expect(app.getByTestId("filter-summary")).toContainText("2026-06-01");
    await expect(app.getByTestId("filter-summary")).toContainText("2026-06-30");
    // The calendar is a projection of that same scope, not a third opinion.
    await app.getByTestId("txn-view-calendar").click();
    await expect(app.getByTestId("txn-cal-month")).toHaveText(PREV_MONTH);
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
  });

  test("the period survives leaving the page and coming back", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-scope-prev").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    await nav(app, "/budgets");
    await nav(app, "/transactions");
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    await expect
      .poll(async () => (await rowDates(app)).every((d) => /Jun\b/.test(d)), { timeout: 15_000 })
      .toBe(true);
  });

  test("the period survives a reload", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-scope-prev").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    await app.reload();
    await app.waitForFunction(
      () => document.documentElement.getAttribute("data-app-ready") === "true",
      null,
      { timeout: 45_000 },
    );
    await nav(app, "/transactions");
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
  });

  test("This month and All dates return to their named states", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-scope-prev").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    await app.getByTestId("txn-scope-thismonth").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(THIS_MONTH);
    // Already on this month, so the control that would do nothing is not offered.
    await expect(app.getByTestId("txn-scope-thismonth")).toHaveCount(0);
    await app.getByTestId("txn-scope-alldates").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(/all dates/i);
    // Unbounded, so there is nothing to clear.
    await expect(app.getByTestId("txn-scope-alldates")).toHaveCount(0);
  });

  test("Ledger and Calendar are a pressed pair with a one-click way back", async ({ app }) => {
    await nav(app, "/transactions");
    const ledger = app.getByTestId("txn-view-ledger");
    const calendar = app.getByTestId("txn-view-calendar");
    await expect(ledger).toHaveAttribute("aria-pressed", "true");
    await expect(calendar).toHaveAttribute("aria-pressed", "false");
    await calendar.click();
    await expect(calendar).toHaveAttribute("aria-pressed", "true");
    await expect(ledger).toHaveAttribute("aria-pressed", "false");
    await expect(app.locator(".txn-cal-grid")).toBeVisible();
    // The way back is the visible pair, not a rediscovered overflow entry.
    await ledger.click();
    await expect(ledger).toHaveAttribute("aria-pressed", "true");
    await expect(app.locator('tr[data-testid^="txn-row-"]').first()).toBeVisible();
    // And the calendar no longer hides in "⋯ More".
    const more = app.locator(String.raw`.add-wrap:has([data-testid="txn-more-btn"])`);
    const moreMenu = await openMenu(app, more, app.getByTestId("txn-more-btn"));
    await expect(moreMenu.locator(String.raw`[data-testid="txn-calendar-btn"]`)).toHaveCount(0);
  });

  test("the calendar carries a caption, not a second month stepper", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-view-calendar").click();
    await expect(app.getByTestId("txn-cal-month")).toBeVisible();
    // Two identical month controls stacked is what this removed.
    await expect(app.getByTestId("txn-cal-prev")).toHaveCount(0);
    await expect(app.getByTestId("txn-cal-next")).toHaveCount(0);
    await expect(app.getByTestId("txn-cal-today")).toHaveCount(0);
  });

  test("picking a calendar day scopes the ledger to that day and says so", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-scope-thismonth").click();
    await app.getByTestId("txn-view-calendar").click();
    await expect(app.getByTestId("txn-cal-month")).toHaveText(THIS_MONTH);
    // Any in-month day with activity; the first enabled cell will do.
    const day = app.locator('[data-testid^="txn-cal-day-2026-07-"]:not([disabled])').first();
    const key = await day.getAttribute("data-date");
    await day.click();
    // Back on the ledger, scoped to exactly that day — and the label states the DAY,
    // not the month it sits in, because the rows are one day's worth.
    await expect(app.getByTestId("txn-view-ledger")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("filter-summary")).toContainText(key);
    await expect(app.getByTestId("txn-scope-label")).not.toHaveText(THIS_MONTH);
  });

  test("the upcoming band steps aside outside a period containing today", async ({ app }) => {
    await nav(app, "/transactions");
    // It belongs to the present, so it shows on an unbounded ledger.
    await expect(app.getByTestId("txn-upcoming-strip")).toBeVisible();
    // Paged back a month, "upcoming" is answering a question the page is not asking.
    await app.getByTestId("txn-scope-prev").click();
    await expect(app.getByTestId("txn-scope-label")).toHaveText(PREV_MONTH);
    await expect(app.getByTestId("txn-upcoming-strip")).toHaveCount(0);
    // Returning to the current month brings it back.
    await app.getByTestId("txn-scope-thismonth").click();
    await expect(app.getByTestId("txn-upcoming-strip")).toBeVisible();
  });
});

test.describe("C561 — Bulk Categorize needs a real choice", () => {
  test("Categorize is inert until a category is chosen, and cannot write a blank one", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    await selectRow(app, row);
    const select = app.getByTestId("bulk-category-select");
    await expect(select).toBeVisible();
    // The resting option is a PROMPT, not the destructive "No category" it used to be.
    await expect(select.locator("option").first()).toHaveText(/choose a category/i);
    // ...and the verb beside it cannot fire.
    await expect(app.getByTestId("bulk-apply-category")).toBeDisabled();
    // Clearing a category is a separate, named action rather than the default's job.
    await expect(app.getByTestId("bulk-clear-category")).toBeVisible();

    // The ticket's AC is about the DATA, not the button: "clicking Categorize with
    // no chosen category cannot mutate a transaction". So force the click past the
    // disabled attribute — the belt-and-braces guard in the handler is what has to
    // hold — and read the row's category cell back.
    const cat = row.locator("td.td-cat");
    // textContent on both sides: innerText normalizes whitespace and drops hidden
    // nodes, so comparing one against the other reports a difference where the DOM
    // has none.
    const before = await cat.textContent();
    await app.getByTestId("bulk-apply-category").dispatchEvent("click");
    await expect(app.getByTestId("bulk-category-select")).toBeVisible(); // selection survives
    await expect.poll(() => cat.textContent(), { timeout: 5_000 }).toBe(before);
  });

  test("choosing a category enables Categorize, and the result names count and category", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    await selectRow(app, row);
    const select = app.getByTestId("bulk-category-select");
    await select.selectOption({ index: 1 });
    const chosen = await select.locator("option:checked").innerText();
    const apply = app.getByTestId("bulk-apply-category");
    await expect(apply).toBeEnabled();
    await apply.click();
    // The toast states BOTH figures, so the commit is verifiable without re-reading rows.
    await expect(app.locator(".toast-msg").first()).toContainText(chosen.trim());
    await expect(app.locator(".toast-msg").first()).toContainText(/filed/i);
  });

  test("clearing a category asks first and says what it costs", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    await selectRow(app, row);
    await app.getByTestId("bulk-clear-category").click();
    await expect(dialog(app)).toBeVisible();
    await expect(dialog(app)).toContainText(/uncategorized/i);
    await expect(dialog(app)).toContainText(/budget|report/i);
    // Backing out changes nothing.
    await dialogCancel(app).click();
    await expect(dialog(app)).toHaveCount(0);
    await expect(app.getByTestId("bulk-category-select")).toBeVisible(); // selection intact
  });
});

test.describe("C562 — excluding from reports is a confirmed, reversible change", () => {
  test("exclude asks, states that balances are unaffected, and can be backed out", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-toggle-exclude" });
    await clickRowMenuItem(app, row, "txn-toggle-exclude");
    await expect(dialog(app)).toBeVisible();
    // The boundary is the point: nothing about this row's own figures changes.
    await expect(dialog(app)).toContainText(/balances do not change/i);
    await expect(dialog(app)).toContainText(/budget/i);
    await dialogCancel(app).click();
    await expect(dialog(app)).toHaveCount(0);
    await expect(row.getByTestId("txn-excluded-badge")).toHaveCount(0);
  });

  test("confirming excludes it, and including it again is immediate and undoable", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-toggle-exclude" });
    await clickRowMenuItem(app, row, "txn-toggle-exclude");
    await dialogConfirm(app).click();
    await expect(row.getByTestId("txn-excluded-badge")).toBeVisible();
    // Both directions post an undoable toast naming the reversal path.
    await expect(app.locator(".toast").first()).toContainText(/undo/i);
    // The menu entry flips, and the restorative direction needs no confirmation.
    const reopened = await openMenu(app, row, row.locator(String.raw`[data-testid^="txn-kebab-"]`));
    await expect(reopened.locator(String.raw`[data-testid="txn-toggle-exclude"]`)).toHaveText(/include in reports/i);
    await clickRowMenuItem(app, row, "txn-toggle-exclude");
    await expect(dialog(app)).toHaveCount(0);
    await expect(row.getByTestId("txn-excluded-badge")).toHaveCount(0);
  });
});

test.describe("C563 — editing a row is visible and keyboard-reachable", () => {
  test("every row carries a named Edit control that opens the editor", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    const edit = row.getByTestId("txn-rowedit");
    await expect(edit).toBeVisible();
    // Named for assistive tech, and it names the transaction it edits.
    await expect(edit).toHaveAttribute("aria-label", /^Edit .+/);
    await edit.click();
    await expect(app.getByTestId("txn-edit-amount")).toBeVisible();
  });

  test("the Edit control is in the tab order and does not swallow the kebab", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    const edit = row.getByTestId("txn-rowedit");
    // Reachable by keyboard — the <tr> click target never was.
    await edit.focus();
    await expect(edit).toBeFocused();
    // Both controls fit their cell; the kebab is still clickable, which is what
    // broke when the Edit control first landed in a 48px column.
    const box = await row.locator("td.td-actions").boundingBox();
    const kebab = await row.locator('[data-testid^="txn-kebab-"]').boundingBox();
    expect(kebab.x + kebab.width).toBeLessThanOrEqual(box.x + box.width + 1);
    await openMenu(app, row, row.locator(String.raw`[data-testid^="txn-kebab-"]`));
  });

  test("Enter on the focused Edit control opens the editor", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    await row.getByTestId("txn-rowedit").focus();
    await app.keyboard.press("Enter");
    await expect(app.getByTestId("txn-edit-amount")).toBeVisible();
  });
});

test.describe("C564 — Split from receipt never no-ops silently", () => {
  test("without an inference key it explains what is needed instead of doing nothing", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-receipt-split-open" });
    await clickRowMenuItem(app, row, "txn-receipt-split-open");
    // The menu closes on activation either way, so silence is indistinguishable
    // from a broken feature. Whichever branch is taken — no key, no resolvable
    // transaction, or a browser that declined the file picker — it must SAY so,
    // and say something about this flow rather than leaving a stale toast to
    // satisfy the assertion.
    await expect(app.locator(".toast-msg").first()).toBeVisible();
    await expect(app.locator(".toast-msg").first()).toContainText(
      /receipt|OpenAI key|file picker/i,
    );
  });
});

test.describe("C565 — a payment-link dialog cannot save a no-op", () => {
  test("Save is disabled on an unlinked charge and explains the required choice", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-markbill-open" });
    await clickRowMenuItem(app, row, "txn-markbill-open");
    await expect(app.getByTestId("txnlink-save")).toBeVisible();
    await expect(app.getByTestId("txnlink-save")).toBeDisabled();
    await expect(app.getByTestId("txnlink-nothing")).toContainText(/pick a bill or a subscription/i);
  });

  test("choosing a real target enables Save", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-markbill-open" });
    await clickRowMenuItem(app, row, "txn-markbill-open");
    const picker = app.getByTestId("txnlink-bill-select");
    await expect(picker).toBeVisible();
    await picker.selectOption({ index: 1 });
    await expect(app.getByTestId("txnlink-save")).toBeEnabled();
    await expect(app.getByTestId("txnlink-preview")).toBeVisible();
  });
});

test.describe("C566 — a split with an unfinished line is neither balanced nor saveable", () => {
  test("the seeded blank line is named, and Save is withheld", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    await clickRowMenuItem(app, row, "txn-split-open");
    await expect(app.getByTestId("split-editor")).toBeVisible();
    // The money adds up, but the split does not exist yet — and the footer says so
    // rather than reporting "Balanced" in the affirmative.
    //
    // Which explanation appears depends on the seeded row: an already-categorized
    // transaction yields one complete line plus a blank ("needs two complete
    // lines"), an uncategorized one yields a half-filled line ("needs a category
    // and an amount"). Both are correct refusals, so the assertion is that it does
    // NOT claim balance and DOES explain itself — the AC, not one wording of it.
    await expect(app.getByTestId("split-remainder")).not.toHaveText(/^\s*balanced\s*$/i);
    await expect(app.getByTestId("split-remainder")).toContainText(
      /at least two complete lines|needs a category and an amount/i,
    );
    await expect(app.getByTestId("split-save")).toBeDisabled();
  });

  test("a half-filled line is called out and still blocks Save", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    await clickRowMenuItem(app, row, "txn-split-open");
    // A category with no amount: started, not finished.
    await app.getByTestId("split-cat-1").selectOption({ index: 1 });
    await expect(app.getByTestId("split-remainder")).toContainText(/needs a category and an amount/i);
    await expect(app.getByTestId("split-save")).toBeDisabled();
  });

  test("finishing every line enables Save and the split persists", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    await clickRowMenuItem(app, row, "txn-split-open");
    const whole = await app.getByTestId("split-amt-0").inputValue();
    // Strip any grouping/symbol the field might carry before doing arithmetic on it.
    const total = Math.round(parseFloat(whole.replace(/[^0-9.]/g, "")) * 100);
    expect(Number.isFinite(total) && total > 1, `unreadable seeded amount ${whole}`).toBe(true);
    const half = (Math.floor(total / 2) / 100).toFixed(2);
    const rest = ((total - Math.floor(total / 2)) / 100).toFixed(2);
    await app.getByTestId("split-amt-0").fill(half);
    await app.getByTestId("split-cat-1").selectOption({ index: 1 });
    await app.getByTestId("split-amt-1").fill(rest);
    await expect(app.getByTestId("split-remainder")).toContainText(/balanced/i);
    await expect(app.getByTestId("split-save")).toBeEnabled();
    await app.getByTestId("split-save").click();
    await expect(app.getByTestId("split-editor")).toHaveCount(0);
  });

  test("Enter still commits the draft rather than discarding it", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    await clickRowMenuItem(app, row, "txn-split-open");
    const whole = await app.getByTestId("split-amt-0").inputValue();
    const total = Math.round(parseFloat(whole.replace(/[^0-9.]/g, "")) * 100);
    const half = (Math.floor(total / 2) / 100).toFixed(2);
    const rest = ((total - Math.floor(total / 2)) / 100).toFixed(2);
    await app.getByTestId("split-amt-0").fill(half);
    await app.getByTestId("split-cat-1").selectOption({ index: 1 });
    await app.getByTestId("split-amt-1").fill(rest);
    await expect(app.getByTestId("split-save")).toBeEnabled();
    // Moving the footer into the editor body must not change what the keyboard
    // does. Enter from a field submits the form; the panel's fallback would have
    // been "no save handler, so close" — silently discarding the draft.
    await app.getByTestId("split-amt-1").press("Enter");
    await expect(app.getByTestId("split-editor")).toHaveCount(0);
    await expect(app.locator(".toast-msg").first()).toContainText(/split/i);
  });
});

test.describe("C567 — a read-only history modal offers nothing to save", () => {
  test("the footer is a single Close", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app, { needs: "txn-history-open" });
    await clickRowMenuItem(app, row, "txn-history-open");
    await expect(app.getByTestId("txn-history-panel")).toBeVisible();
    const foot = app.locator(".set-foot button");
    await expect(foot).toHaveCount(1);
    await expect(foot.first()).toHaveText(/close/i);
    // Specifically: no Save that could only ever do nothing.
    await expect(app.getByTestId("flip-save")).toHaveCount(0);
  });
});

test.describe("C568 — quick-filter counts describe the current view", () => {
  test("the counts shrink to the searched cohort and say what they count", async ({ app }) => {
    await nav(app, "/transactions");
    const readCounts = async () =>
      app.evaluate(() =>
        [...document.querySelectorAll('[data-testid^="txn-preset-"]')].map((c) =>
          Number((c.getAttribute("aria-label") || "").match(/(\d+) in the current view/)?.[1] ?? -1),
        ),
      );
    await openVia(app, app.locator(".filters-trigger"), app.getByTestId("txn-presets"));
    const wide = await readCounts();
    // -1 means the accessible name did not carry a count at all — the chip would
    // then be announcing a bare number with no population, which is the defect.
    expect(wide.length, "no preset chips found").toBeGreaterThan(0);
    expect(wide.includes(-1), "a preset chip's accessible name carries no count").toBe(false);
    expect(wide.some((n) => n > 0)).toBe(true);
    // The note qualifies the figures at rest, not on hover.
    await expect(app.getByTestId("txn-presets-note")).toContainText(/current search and filters/i);
    // A search that matches a small cohort must pull every count down with it.
    await app.locator("input.fctrl-input").fill("Car payment");
    await expect
      .poll(async () => {
        const narrow = await readCounts();
        return narrow.length === wide.length && narrow.every((n, i) => n <= wide[i]) && narrow.some((n, i) => n < wide[i]);
      }, { timeout: 15_000 })
      .toBe(true);
  });
});

test.describe("the Amount column sorts by the figure it displays", () => {
  test("ascending runs most-negative to most-positive, not by size", async ({ app }) => {
    await nav(app, "/transactions");
    // Sort by Amount ascending. The first click takes the column's default
    // direction (descending), so click until the header reports ascending — read
    // through evaluate so a second `.txn-table` on the page cannot trip strict mode.
    const amountSort = () =>
      app.evaluate(() => {
        const th = [...document.querySelectorAll(".txn-table thead th")].find(
          (h) => h.textContent.trim().startsWith("Amount"),
        );
        return th ? th.getAttribute("aria-sort") : null;
      });
    // The re-sort is deferred a macrotask so the busy state paints first, so a
    // read taken straight after the click sees the PREVIOUS direction. Poll the
    // header's own state and click only while it is not yet ascending; the column
    // cycles none → descending → ascending, so this settles in two clicks.
    const amountHeader = app
      .locator(".txn-table thead th", { hasText: "Amount" })
      .first()
      .locator("button.th-sort");
    await expect
      .poll(async () => {
        if ((await amountSort()) === "ascending") return "ascending";
        await amountHeader.click().catch(() => {});
        await app.waitForTimeout(800);
        return amountSort();
      }, { timeout: 30_000 })
      .toBe("ascending");

    // Read the displayed figures. Parentheses are the ledger's notation for money
    // out, so "($620.00)" is -620 — the sort must agree with THAT reading.
    const values = await app.evaluate(() =>
      [...document.querySelectorAll('tr[data-testid^="txn-row-"] td.td-amount')].map((td) => {
        const raw = td.textContent.trim();
        const n = Number(raw.replace(/[^0-9.]/g, ""));
        return /^\(/.test(raw) || raw.startsWith("-") ? -n : n;
      }),
    );
    expect(values.length).toBeGreaterThan(2);
    // Non-decreasing down the column. Under the old magnitude ordering a small
    // expense preceded a large one and income was interleaved throughout.
    const sorted = [...values].sort((a, b) => a - b);
    expect(values, `column is not in ascending order: ${values.slice(0, 8)}`).toEqual(sorted);
  });
});

test.describe("the header checkbox selects every row in view", () => {
  test("it selects all shown rows, then clears them", async ({ app }) => {
    await nav(app, "/transactions");
    const box = app.getByTestId("txn-select-all-visible");
    await expect(box).toBeVisible();
    await expect(box).not.toBeChecked();

    const rowBoxes = app.locator('tr[data-testid^="txn-row-"] input[type=checkbox]');
    const shown = await rowBoxes.count();
    expect(shown).toBeGreaterThan(1);

    await box.click();
    await expect(box).toBeChecked();
    // Every row on screen, not merely some of them.
    await expect
      .poll(async () => {
        let n = 0;
        for (let i = 0; i < shown; i++) if (await rowBoxes.nth(i).isChecked()) n++;
        return n;
      }, { timeout: 15_000 })
      .toBe(shown);
    // And the bulk bar agrees about the count.
    await expect(app.getByTestId("bulk-category-select")).toBeVisible();

    // Clicking again clears them.
    await box.click();
    await expect(box).not.toBeChecked();
    await expect
      .poll(async () => {
        let n = 0;
        for (let i = 0; i < shown; i++) if (await rowBoxes.nth(i).isChecked()) n++;
        return n;
      }, { timeout: 15_000 })
      .toBe(0);
  });

  test("a partial selection shows the middle state, not an empty box", async ({ app }) => {
    await nav(app, "/transactions");
    const rowBoxes = app.locator('tr[data-testid^="txn-row-"] input[type=checkbox]');
    await rowBoxes.first().click();
    const box = app.getByTestId("txn-select-all-visible");
    // An unchecked box over a partial selection would claim nothing is selected;
    // `indeterminate` is a property, so this is read off the element itself.
    await expect
      .poll(() => box.evaluate((el) => el.indeterminate), { timeout: 15_000 })
      .toBe(true);
    await expect(box).not.toBeChecked();
    // From partial, one click completes the selection rather than clearing it.
    await box.click();
    await expect(box).toBeChecked();
    await expect.poll(() => box.evaluate((el) => el.indeterminate)).toBe(false);
  });
});

test.describe("C569 — the ledger says it is re-sorting", () => {
  test("aria-busy and the column spinner hold across the re-sort", async ({ app }) => {
    await nav(app, "/transactions");
    const table = app.locator(".txn-table").first();
    const header = app.locator(".txn-table th button.th-sort").first();
    // The busy window is short by design, so it is watched with a FRAME-polled
    // predicate armed BEFORE the click rather than sampled in a loop after it. A
    // sampling loop that happens to miss the window would report "never busy" on a
    // correct build — the flake would look exactly like the bug.
    const sawBusy = app.waitForFunction(
      () => {
        const t = document.querySelector(".txn-table");
        // The two signals must agree in the same frame: a spinner without the
        // table marked busy would announce nothing to assistive tech.
        return !!t && t.getAttribute("aria-busy") === "true" && !!t.querySelector(".dt-spin");
      },
      null,
      { timeout: 15_000, polling: "raf" },
    );
    await header.click();
    await sawBusy;
    // It is transient, not stuck, and the rows survive the transition.
    await expect.poll(() => table.getAttribute("aria-busy"), { timeout: 15_000 }).toBeNull();
    await expect(app.locator(".txn-table .dt-spin")).toHaveCount(0);
    await expect(app.locator('tr[data-testid^="txn-row-"]').first()).toBeVisible();
  });
});

test.describe("C570 — every category picker names the full path", () => {
  test("bulk categorize lists qualified paths, sorted, behind a prompt", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator(String.raw`tr[data-testid^="txn-row-"]`).nth(4);
    await row.scrollIntoViewIfNeeded();
    // The bulk bar is a tile the surface adds once a selection exists, so wait for
    // it rather than reading the DOM in the same tick as the click.
    await selectRow(app, row);
    const opts = await app.evaluate(() =>
      [...document.querySelectorAll('[data-testid="bulk-category-select"] option')].map((o) => o.text),
    );
    expect(opts.length, "the bulk category picker rendered no options").toBeGreaterThan(1);
    expect(opts[0]).toMatch(/choose a category/i);
    // A nested category shows its parent, so two leaves with the same name differ.
    expect(opts.some((t) => t.includes(" > ")), `no qualified path in ${opts.slice(0, 8)}`).toBe(true);
  });

  test("the split editor lists qualified paths", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    await clickRowMenuItem(app, row, "txn-split-open");
    const opts = await app.evaluate(() =>
      [...document.querySelectorAll('[data-testid="split-cat-1"] option')].map((o) => o.text),
    );
    expect(opts.some((t) => t.includes(" > "))).toBe(true);
  });

  test("quick-add lists qualified paths", async ({ app }) => {
    await nav(app, "/transactions");
    await openVia(app, app.getByTestId("add-transaction-btn"), app.getByTestId("txn-add-amount"));
    const opts = await app.evaluate(() =>
      [...document.querySelectorAll('[data-testid="txn-add-category"] option')].map((o) => o.text),
    );
    expect(opts.some((t) => t.includes(" > "))).toBe(true);
  });
});

test.describe("C571 — duplicate review names what survives", () => {
  test("merge names the kept entry, the count and the real recovery path", async ({ app }) => {
    await nav(app, "/transactions");
    await clickToolbarMenuItem(app, "txn-dupes-btn");
    const merge = app.getByTestId("dup-merge-btn").first();
    // The seeded household may hold no duplicate group at the pinned clock; the
    // copy is still worth locking down when one exists, and skipping is honest
    // about the gap rather than asserting against an empty panel.
    if (!(await merge.count())) test.skip(true, "the seeded dataset has no duplicate group");
    await expect(merge).toBeVisible();
    await merge.click();
    await expect(dialog(app)).toBeVisible();
    await expect(dialog(app)).toContainText(/stays/i);
    await expect(dialog(app)).toContainText(/(copy is|copies are) removed/i);
    // The old copy claimed "this can't be undone" while the code captured an undo
    // point. The confirmation now describes the recovery that actually exists.
    await expect(dialog(app)).toContainText(/undo this from Activity/i);
    await expect(dialogConfirm(app)).toHaveText(/merge into one/i);
    await dialogCancel(app).click();
  });

  test("deleting a duplicate names the copy and leaves the kept entry alone", async ({ app }) => {
    await nav(app, "/transactions");
    await clickToolbarMenuItem(app, "txn-dupes-btn");
    const del = app.getByTestId("dup-delete-btn").first();
    if (!(await del.count())) test.skip(true, "the seeded dataset has no deletable duplicate copy");
    await del.click();
    await expect(dialog(app)).toBeVisible();
    await expect(dialog(app)).toContainText(/kept at the top of the group is untouched/i);
    await expect(dialog(app)).toContainText(/undo this from Activity/i);
    await expect(dialog(app)).not.toContainText(/can't be undone/i);
    await dialogCancel(app).click();
  });
});

test.describe("C572 — the row menu communicates risk before activation", () => {
  test("entries are grouped by cost, in order, with the destructive ones marked", async ({ app }) => {
    await nav(app, "/transactions");
    const row = await openRowMenu(app);
    const shape = await app.evaluate(() => {
      const menu = [...document.querySelectorAll(".add-menu")].find((m) => !m.classList.contains("hidden-menu"));
      if (!menu) return null;
      return {
        sections: [...menu.querySelectorAll(".add-menu-section")].map((s) => s.textContent.trim()),
        exclude: [...menu.querySelectorAll(".add-item")]
          .find((x) => x.dataset.testid === "txn-toggle-exclude")?.classList.contains("danger"),
        del: [...menu.querySelectorAll(".add-item")]
          .find((x) => x.dataset.testid === "txn-delete")?.classList.contains("danger"),
        rule: [...menu.querySelectorAll(".add-item")]
          .find((x) => x.dataset.testid === "txn-create-rule")?.classList.contains("danger"),
      };
    });
    expect(shape.sections).toEqual(["Organize", "Links", "Reporting", "Remove"]);
    // Risk is legible from the item itself, not only from its position.
    expect(shape.exclude, "Exclude from reports should read as destructive").toBe(true);
    expect(shape.del, "Delete should read as destructive").toBe(true);
    expect(shape.rule, "creating a rule is routine and must not read as destructive").toBe(false);
    void row;
  });

  test("the grouped menu stays usable at a narrow width", async ({ app }) => {
    await app.setViewportSize({ width: 860, height: 900 });
    await nav(app, "/transactions");
    await settle(app);
    const row = await openRowMenu(app, { needs: "txn-delete" });
    const del = row.locator('[data-testid="txn-delete"]');
    await expect(del).toBeVisible();
    // Inside the viewport, not clipped off the right edge.
    const box = await del.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(860);
  });
});
