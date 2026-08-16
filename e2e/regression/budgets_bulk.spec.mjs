// budgets_bulk.spec.mjs — the C592 "Adjust all" form and the C589 custom-range
// workflow. Both replaced controls that changed the app BEFORE the user could see
// what they would do, so every test here asserts the same shape: the preview is
// visible and correct first, and nothing has moved until the explicit apply.
import { test, expect, nav, openVia } from "./fixtures.mjs";

// openAdjustAll opens the Budget settings popover and the Adjust-all form. The
// popover swallows a click while the tile is still re-rendering, so the trigger
// is retried until its menu item is actually visible (the openVia pattern).
// The check has to be VISIBILITY, not presence: the popover's contents stay in
// the DOM and are hidden by a class, so a count-based check passes against a
// closed menu.
const SETTINGS_MENU = ".bud-set-menu:not(.hidden-menu)";
// Same trap on the period pill: its popover is hidden by a class, not unmounted.
const PERIOD_POP = ".period-pop:not(.hidden-menu)";

async function openAdjustAll(app) {
  await app.getByTestId("budgets-settings").scrollIntoViewIfNeeded();
  await openVia(app, app.getByTestId("budgets-settings"), app.locator(SETTINGS_MENU));
  await openVia(app, app.getByTestId("budgets-adjust-all"), app.getByTestId("adjustall-form"));
}

// budgetTotals reads every budget row's "spent / limit" pair, so a test can prove
// the page did or did not change.
async function budgetLimits(app) {
  return app.evaluate(() =>
    [...document.querySelectorAll('[data-testid^="budget-card-"]')].map((el) => el.innerText.replace(/\s+/g, " ").trim()),
  );
}

test.describe("budgets: adjust all", () => {
  test("rejects blank, zero and out-of-range values with inline guidance", async ({ app }) => {
    await nav(app, "/budgets");
    await openAdjustAll(app);

    const apply = app.getByTestId("adjustall-apply");
    const pct = app.getByTestId("adjustall-pct");

    // Blank is not an error — it is simply nothing typed yet — but it cannot apply.
    await expect(apply).toBeDisabled();
    await expect(app.getByTestId("adjustall-err")).toHaveCount(0);

    await pct.fill("0");
    await expect(app.getByTestId("adjustall-err")).toContainText(/leave every budget exactly as it is/i);
    await expect(apply).toBeDisabled();

    await pct.fill("900");
    await expect(app.getByTestId("adjustall-err")).toContainText(/between -90% and 500%/i);
    await expect(apply).toBeDisabled();
  });

  test("previews the total and every budget before applying, and cancel changes nothing", async ({ app }) => {
    await nav(app, "/budgets");
    const before = await budgetLimits(app);
    await openAdjustAll(app);

    await app.getByTestId("adjustall-pct").fill("5");
    // The preview names the scope and shows the total's before → after.
    await expect(app.getByTestId("adjustall-count")).toContainText(/budgets? will change by 5%/i);
    const total = app.getByTestId("adjustall-total");
    await expect(total).toBeVisible();
    await expect(total).toContainText("→");
    // Every affected budget has its own before → after line.
    await expect(app.locator('[data-testid^="adjustall-row-"]').first()).toBeVisible();
    // A modest raise needs no acknowledgement.
    await expect(app.getByTestId("adjustall-ack")).toHaveCount(0);
    await expect(app.getByTestId("adjustall-apply")).toBeEnabled();

    await app.getByTestId("adjustall-cancel").click();
    await expect(app.getByTestId("adjustall-form")).toHaveCount(0);
    expect(await budgetLimits(app), "cancel must not write anything").toEqual(before);
  });

  test("a reduction requires an explicit acknowledgement, then applies to every budget", async ({ app }) => {
    await nav(app, "/budgets");
    await openAdjustAll(app);
    await app.getByTestId("adjustall-pct").fill("-10");

    const apply = app.getByTestId("adjustall-apply");
    const ack = app.getByTestId("adjustall-ack");
    await expect(ack).toBeVisible();
    // The sentence states the direction in words and the size as a magnitude.
    await expect(app.getByTestId("adjustall-ack-label")).toContainText(/lower .* by 10%/i);
    await expect(apply).toBeDisabled();

    // The preview's first line is the contract the write has to honour.
    const firstRow = app.locator('[data-testid^="adjustall-row-"]').first();
    const rowId = (await firstRow.getAttribute("data-testid")).replace("adjustall-row-", "");
    const predicted = (await firstRow.innerText()).trim().split(/\s+/).pop();

    await ack.click();
    await expect(apply).toBeEnabled();
    await apply.click();
    await expect(app.getByTestId("adjustall-form")).toHaveCount(0);

    // The budget now carries exactly the limit the preview promised.
    await expect(app.getByTestId(`budget-card-${rowId}`)).toContainText(predicted);
  });
});

test.describe("period: custom range", () => {
  test("drafts the range, previews its span, and applies only on demand", async ({ app }) => {
    await nav(app, "/budgets");
    const pill = app.getByTestId("period-pill");
    const applied = (await pill.innerText()).trim();

    // The pill's popover, then the range editor. Both openings are retried: the
    // page keeps re-rendering briefly after mount and a click landing in that
    // window is discarded (the openVia pattern).
    await openVia(app, pill, app.locator(PERIOD_POP));
    await openVia(app, app.locator(PERIOD_POP).getByTestId("period-range-open"), app.getByTestId("period-range-editor"));

    // Opening the editor must not relabel the pill — nothing has been chosen yet.
    expect((await pill.innerText()).trim()).toBe(applied);
    await expect(app.getByTestId("period-range-preview")).toContainText(/only — a single period/i);
    await expect(app.getByTestId("period-range-apply")).toBeDisabled();
    // The note says what a range does and does not change.
    await expect(app.getByTestId("period-range-note")).toContainText(/keeps its own cadence/i);

    // Move the end two periods out: the draft moves, the applied view does not.
    const later = app.locator('[data-testid="period-range-editor"] [aria-label="Move end later"]');
    await later.click();
    await later.click();
    await expect(app.getByTestId("period-range-preview")).toContainText(/— 3 periods/);
    expect((await pill.innerText()).trim()).toBe(applied);

    await app.getByTestId("period-range-apply").click();
    await expect(pill).toContainText("–");

    // Cancel on an applied range returns to a single period, and says so.
    await expect(app.getByTestId("period-range-cancel")).toContainText(/back to a single period/i);
    // Applying re-renders this control, and a click landing in that window is
    // discarded by the framework — the same effect openVia() exists for. Retry
    // until the view actually collapses rather than asserting a single click
    // landed; the assertion is on the OUTCOME, which is what the ticket is about.
    await expect
      .poll(
        async () => {
          const cancel = app.getByTestId("period-range-cancel");
          if (await cancel.count()) await cancel.click().catch(() => {});
          return (await pill.innerText()).trim();
        },
        { timeout: 15_000 },
      )
      .toBe(applied);
  });
});
