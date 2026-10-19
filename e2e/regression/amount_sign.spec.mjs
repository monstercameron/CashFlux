// amount_sign.spec.mjs — typing a minus sign into an amount field (C546/C547).
//
// Cam's report was "I get 'Enter a positive amount.' … users should be allowed to
// adjust mistaken incomes back into debits". C520 gave the edit form a Direction
// control, but it never addressed what people actually DO to change a direction:
// they type a minus in front of the number. Two different failures were hiding
// behind that one gesture.
//
//   C546 — quick-add and the recurring inline editor guarded on `amt == 0`, so a
//          typed "-50" parsed to −5000, survived the guard, and was then negated
//          by the Expense branch into +50 INCOME. Saved silently — the opposite
//          of what was typed, with no error and nothing to notice.
//   C547 — the transaction edit form guarded on `amt <= 0` and rejected it
//          outright, producing the message Cam quoted.
//
// Both now route through amountmath.ParseSigned: a typed minus is read as intent,
// resolves to "money out", and the direction control follows so the form can
// never display a direction that disagrees with what was stored.
//
// The ledger's own statement of direction is the tone class on the amount cell —
// .text-down for spending, .text-up for income — so that is what these assert
// against rather than a formatted string.
import { test, expect, nav, openVia } from "./fixtures.mjs";

// Both panels open through openVia: a single click can land while the wasm app
// is mid-render and be swallowed, so the helper retries until the panel's own
// control is actually present.
// Quick-add is opened from /transactions because that is where a user adds a
// transaction. (It also used to be the only route where the panel reliably
// became visible — an unrevealed flip modal was `opacity: 0` but still in the
// DOM and the Tab order, so on the dashboard it could be fillable while nothing
// was on screen. Fixed in C560; the panel is now `visibility: hidden` until
// revealed. `flip_inert.spec.mjs` guards that directly.)
async function openQuickAdd(app) {
  await nav(app, "/transactions");
  await openVia(app, app.getByTestId("add-transaction-btn"), app.getByTestId("txn-add-amount"));
}

async function openMoreDetails(app) {
  const more = app.getByTestId("txn-add-more");
  if ((await more.getAttribute("open")) === null) {
    await more.locator("summary").click();
  }
  await expect(app.getByTestId("txn-add-desc")).toBeVisible();
}

async function openFirstTxnEdit(app) {
  await nav(app, "/transactions");
  const firstRow = app.locator('tr[data-testid^="txn-row-"]').first();
  await openVia(
    app,
    firstRow.getByTestId("txn-rowedit"),
    app.getByTestId("txn-edit-amount"),
  );
}

// saveAndClose commits a flip panel and waits for it to actually go away.
//
// The same reason openVia exists applies to closing: the footer Save is
// re-rendered across the wasm boundary as the form's validity gate recomputes,
// so a single click can land on a node that is being replaced and do nothing.
// Poll the click against the panel's own disappearance rather than asserting
// once and hoping.
async function saveAndClose(app, closes) {
  await expect
    .poll(
      async () => {
        if ((await closes.count()) === 0) return true;
        // A SHORT click timeout is the point: Save is briefly `disabled` while
        // the form's validity gate re-renders after a fill, and the dashboard's
        // recap header can transiently intercept the pointer. With the default
        // (test-length) timeout the first attempt would consume the whole budget
        // and this poll would never get a second try.
        await app
          .getByRole("button", { name: /^save$/i })
          .last()
          .click({ timeout: 2000 })
          .catch(() => {});
        await closes.waitFor({ state: "detached", timeout: 1500 }).catch(() => {});
        return (await closes.count()) === 0;
      },
      { timeout: 30_000 },
    )
    .toBe(true);
}

// saveEdit commits the transaction edit form. Unlike quick-add, this panel stays
// open after a successful save (it offers the TX7 "N more look like this"
// follow-up), so there is no disappearance to wait on — retry the click until it
// lands, then let the caller assert on the result.
// `until` is a predicate describing the OBSERVABLE result of the save. Passing
// one matters: without a success signal, a click that silently missed (the panel
// runs a 550ms flip animation, and the footer re-renders as the validity gate
// recomputes) is indistinguishable from a save that succeeded, and the test
// would then assert against unchanged data and blame the product.
async function saveEdit(app, until) {
  // Commit the amount field FIRST. It carries a blur handler (TX16 evaluates an
  // arithmetic entry on blur), and blurring re-renders the form — so clicking
  // Save straight from a focused amount input lands on a node that is being
  // replaced and the submit never runs. Pressing Tab is what a keyboard user
  // does anyway, and it makes the click deterministic.
  await app.keyboard.press("Tab");
  await app.waitForTimeout(300);
  const save = app.getByTestId("txn-edit-save");
  await expect
    .poll(
      async () => {
        if (until && (await until())) return true;
        await save.click({ timeout: 2000 }).catch(() => {});
        await app.waitForTimeout(400);
        return until ? await until() : true;
      },
      { timeout: 25_000 },
    )
    .toBe(true);
}

// typeAmount replaces an amount field's contents with REAL keystrokes.
//
// Playwright's fill() sets .value and dispatches one synthetic input event. For
// the edit form's amount that is not enough to move the Go-side state behind the
// controlled input — the field shows the new text while `amountS` still holds the
// old value, so Save recomputes from the ORIGINAL amount and appears to do
// nothing at all (no error, no toast, no change). Diagnosed by probe: a
// direction-only edit persisted while an amount edit did not.
async function typeAmount(app, locator, text) {
  await locator.click();
  await app.keyboard.press("ControlOrMeta+a");
  await locator.pressSequentially(text, { delay: 20 });
}

// closeEditPanel dismisses the edit form so the row can be reopened fresh.
async function closeEditPanel(app) {
  await app.keyboard.press("Escape");
  await expect(app.getByTestId("txn-edit-amount")).toHaveCount(0);
}

// rowTone returns "down" (spending) or "up" (income) for the ledger row whose
// description matches, or "" when neither tone is present.
async function rowTone(app, desc) {
  const row = app.locator('tr[data-testid^="txn-row-"]').filter({ hasText: desc }).first();
  if ((await row.count()) === 0) return "";
  if (await row.locator("td.td-amount.text-down").count()) return "down";
  if (await row.locator("td.td-amount.text-up").count()) return "up";
  return "";
}

// expectRowTone is rowTone as an assertion, polling so the ledger's re-render
// across the wasm boundary is waited on rather than raced.
async function expectRowTone(app, desc, want) {
  await expect.poll(async () => rowTone(app, desc), { timeout: 20_000 }).toBe(want);
}

test.describe("C546 · a typed minus is never inverted", { tag: "@prod" }, () => {
  test("quick-add stores a spend, not its opposite", async ({ app }) => {
    await openQuickAdd(app);
    await openMoreDetails(app);

    // "Expense" is the default kind; type the amount WITH a minus, the way
    // someone writing money-out would.
    await app.getByTestId("txn-add-amount").fill("-50");
    await app.getByTestId("txn-add-desc").fill("MINUSPROBE");
    await saveAndClose(app, app.getByTestId("txn-add-amount"));

    await expectRowTone(app, "MINUSPROBE", "down");
  });

  test("a typed minus overrides an Income control rather than flipping", async ({ app }) => {
    await openQuickAdd(app);
    await openMoreDetails(app);

    // The nastiest case: the control says Income and the text says minus. The
    // typed sign is the more deliberate signal, so it wins — and the control
    // moves to match, never displaying "Income" over a stored expense.
    await app.getByTestId("txn-add-direction-income").click();
    await app.getByTestId("txn-add-amount").fill("-42");
    await app.getByTestId("txn-add-desc").fill("CONTRAPROBE");
    await saveAndClose(app, app.getByTestId("txn-add-amount"));

    await expectRowTone(app, "CONTRAPROBE", "down");
  });

  test("without a minus the Income control still means income", async ({ app }) => {
    await openQuickAdd(app);
    await openMoreDetails(app);

    // The control has to keep working — the fix must not make everything a spend.
    await app.getByTestId("txn-add-direction-income").click();
    await app.getByTestId("txn-add-amount").fill("42");
    await app.getByTestId("txn-add-desc").fill("PLAINPROBE");
    await saveAndClose(app, app.getByTestId("txn-add-amount"));

    await expectRowTone(app, "PLAINPROBE", "up");
  });
});

test.describe("C547 · the edit form reads a typed minus as direction", { tag: "@prod" }, () => {
  test("a typed minus saves instead of being refused", async ({ app }) => {
    await openFirstTxnEdit(app);

    const amount = app.getByTestId("txn-edit-amount");
    const magnitude = (await amount.inputValue()).replace(/[^\d.]/g, "") || "25";
    await app.getByTestId("txn-edit-direction-income").click(); // make it income first
    await typeAmount(app, amount, `-${magnitude}`);
    await saveEdit(app);

    // The defect was the sticky error banner reading "Enter an amount greater
    // than zero" — the exact message Cam reported, for the exact gesture that
    // was supposed to fix his mistake. Its absence is the fix.
    //
    // The form legitimately STAYS OPEN after a successful save (it offers the
    // TX7 "N more look like this" follow-up), so its closing is not the signal
    // here — the absence of the rejection is.
    await expect(app.locator(".form-err-sticky")).toHaveCount(0);
  });

  // C561 was the reason this could not pass: the amount input was type=number with
  // min="0", so a typed minus failed HTML constraint validation and the browser
  // silently refused to submit the form — the Go handler never ran at all. With the
  // field now text + inputmode=decimal, the save reaches the handler.
  test("the direction control follows the typed sign", async ({ app }) => {
    // Author the row under test rather than borrowing a seeded one: a seeded row
    // may be a transfer (whose sign is structural, not a classification) or carry
    // splits (whose total must reconcile), and either would make this assert
    // something other than what it claims to.
    await openQuickAdd(app);
    await openMoreDetails(app);
    await app.getByTestId("txn-add-direction-income").click();
    await app.getByTestId("txn-add-amount").fill("77");
    await app.getByTestId("txn-add-desc").fill("DIRPROBE");
    await saveAndClose(app, app.getByTestId("txn-add-amount"));

    // It starts as income, by its own control.
    const openProbe = () =>
      openVia(
        app,
        app
          .locator('tr[data-testid^="txn-row-"]')
          .filter({ hasText: "DIRPROBE" })
          .first()
          .getByTestId("txn-rowedit"),
        app.getByTestId("txn-edit-amount"),
      );
    await openProbe();
    await expect(app.getByTestId("txn-edit-direction-income")).toHaveAttribute("aria-pressed", "true");

    // Now contradict it with a typed minus and save. The ledger row flipping to
    // the spending tone is the proof the write landed — assert on that rather
    // than on the click having been dispatched.
    await typeAmount(app, app.getByTestId("txn-edit-amount"), "-77");
    await saveEdit(app, async () => (await rowTone(app, "DIRPROBE")) === "down");
    await expect(app.locator(".form-err-sticky")).toHaveCount(0);
    await closeEditPanel(app);

    // Reopening must show "out": a form that stored a spend while displaying
    // "Money in" is lying about the row the user just saved.
    await openProbe();
    await expect(app.getByTestId("txn-edit-direction-expense")).toHaveAttribute("aria-pressed", "true");
  });

  test("a zero amount is still refused, visibly", async ({ app }) => {
    await openFirstTxnEdit(app);
    await typeAmount(app, app.getByTestId("txn-edit-amount"), "0");
    // A plain click here, NOT saveAndClose: this save must be refused, so the
    // panel is expected to stay open.
    await saveEdit(app);

    // Reading a minus as direction must not weaken the zero guard — and the
    // message still has to be on screen next to the button that produced it.
    await expect(app.locator(".form-err-sticky")).toBeInViewport({ timeout: 10000 });
  });
});
