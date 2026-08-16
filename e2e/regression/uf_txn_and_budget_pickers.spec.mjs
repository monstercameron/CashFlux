// uf_txn_and_budget_pickers.spec.mjs — three reported defects that share a
// theme: the app knew something and did not say it.
//
//   C517 — transactions could be filtered by direction, but only by typing a
//          natural-language search. The Criteria field, the matching logic and
//          the chip label all existed; there was no control.
//   C519 — the budget category picker silently withheld every income category.
//   C520 — the direction of a transaction was immutable, and the error that
//          said so rendered below the fold of a scrolling modal.
//
// NOT in the deploy gate (@prod), deliberately. These pass locally and on a warm
// machine but have failed twice on a cold single-worker CI runner, with a
// DIFFERENT test failing each time — the signature of a timing sensitivity in the
// app rather than in any one assertion (screens re-render briefly after mounting;
// a click landing in that window is discarded — see C541). A gate exists to be
// believed, so a spec that fails intermittently belongs in the nightly full suite
// until the underlying re-render is fixed, not in the thing that decides whether
// production updates. They still run every night and on every local run.
import { test, expect, nav, openVia } from "./fixtures.mjs";

test.describe("C517 · filtering by direction", () => {
  // Spending renders in accounting parentheses with a .text-down tone; income
  // renders plain with .text-up. Those classes are the ledger's own statement of
  // direction, so they are what the filter has to agree with.
  async function openFilters(app) {
    await nav(app, "/transactions");
    // The flow control only exists once the panel is open, so it is the proof
    // the click landed — checking aria-expanded would pass on a panel that then
    // got re-rendered away underneath.
    await openVia(app, app.getByRole("button", { name: /filters/i }).first(), app.getByLabel(/money in or out/i));
  }

  test("a control exists for money in vs money out", async ({ app }) => {
    await openFilters(app);
    const flow = app.getByLabel(/money in or out/i).first();
    await expect(flow).toBeVisible();
    // "Both" is the default: a filter that starts applied hides rows the user
    // never asked to hide.
    await expect(flow).toHaveValue("");
  });

  test("money-in leaves only income; money-out leaves only spending", async ({ app }) => {
    await openFilters(app);
    const flow = app.getByLabel(/money in or out/i).first();
    const up = app.locator('tr[data-testid^="txn-row-"] td.td-amount.text-up');
    const down = app.locator('tr[data-testid^="txn-row-"] td.td-amount.text-down');

    // Poll the WHOLE condition rather than asserting the two counts in sequence:
    // the list re-renders across the wasm boundary, so a moment exists where the
    // incoming rows are present and the outgoing ones have not gone yet. Reading
    // one count and then the other catches that moment and fails a filter that
    // is working.
    await flow.selectOption("in");
    await expect
      .poll(async () => `${(await up.count()) > 0}/${await down.count()}`, { timeout: 15000 })
      .toBe("true/0");

    await flow.selectOption("out");
    await expect
      .poll(async () => `${(await down.count()) > 0}/${await up.count()}`, { timeout: 15000 })
      .toBe("true/0");

    // Back to both: every row returns, so the control is reversible.
    await flow.selectOption("");
    await expect
      .poll(async () => (await up.count()) > 0 && (await down.count()) > 0, { timeout: 15000 })
      .toBe(true);
  });

  test("the active filter announces itself as a chip", async ({ app }) => {
    await openFilters(app);
    await app.getByLabel(/money in or out/i).first().selectOption("in");
    // The chip machinery already handled Flow; this proves the control reaches it.
    await expect(app.locator(".filter-chips, .chips").first()).toContainText(/money in|income/i);
  });
});

test.describe("C519 · the budget picker explains what it withholds", () => {
  test("income categories are absent, and the picker says why", async ({ app }) => {
    await nav(app, "/budgets");
    // Use the page own Add control, not a name match — the global add menu has a
    // "New budget" item that opens a different host and would silently test
    // something else.
    await app.getByTestId("budgets-add").click();
    // The multi-category picker lives behind Advanced on the add form.
    const adv = app.getByTestId("budget-add-advanced");
    if (await adv.count()) await adv.click();

    const note = app.getByTestId("budgetcats-kind-note");
    await expect(note).toBeVisible();
    // It must count them and give the reason, not just assert a rule.
    await expect(note).toContainText(/\d+ income categories are not shown/i);
    await expect(note).toContainText(/spend/i);
  });
});

test.describe("C520 · a mistaken income can be corrected to a spend", () => {
  // The edit form opens by clicking a row's description.
  async function openFirstTxnEdit(app) {
    await nav(app, "/transactions");
    await openVia(
      app,
      app.locator('tr[data-testid^="txn-row-"] .row-desc-text').first(),
      app.getByTestId("txn-edit-amount"),
    );
  }

  test("direction is a control, not a constant", async ({ app }) => {
    await openFirstTxnEdit(app);
    const dir = app.getByTestId("txn-edit-direction");
    await expect(dir).toBeVisible();
    // Seeded from the transaction's current sign rather than defaulting blindly.
    await expect(dir).toHaveValue(/^(in|out)$/);
  });

  test("flipping the direction actually changes the stored sign", async ({ app }) => {
    await openFirstTxnEdit(app);
    const dir = app.getByTestId("txn-edit-direction");
    const before = await dir.inputValue();
    const flipped = before === "in" ? "out" : "in";
    await dir.selectOption(flipped);
    await app.getByRole("button", { name: /^save$/i }).last().click();

    // Reopen and confirm it stuck. Before this fix the form re-applied the
    // ORIGINAL sign after validating, so direction could never move and a charge
    // filed as income had to be deleted and re-entered to be corrected.
    await openFirstTxnEdit(app);
    await expect(app.getByTestId("txn-edit-direction")).toHaveValue(flipped);

    // Put it back so the run leaves no residue for other specs.
    await app.getByTestId("txn-edit-direction").selectOption(before);
    await app.getByRole("button", { name: /^save$/i }).last().click();
  });

  test("a rejected save shows its reason without scrolling", async ({ app }) => {
    await openFirstTxnEdit(app);
    await app.getByTestId("txn-edit-amount").fill("0");
    await app.getByRole("button", { name: /^save$/i }).last().click();

    // The message used to render at the bottom of a scrolling body, out of sight
    // of the button that produced it, so Save looked inert.
    const err = app.locator(".form-err-sticky");
    await expect(err).toBeInViewport({ timeout: 10000 });
    // ...and it must say how to fix it, not merely that it is wrong.
    await expect(err).toContainText(/direction/i);
  });
});

test.describe("C522 · the re-categorizer's dead end", () => {
  test("the no-provider notice offers the action it names", async ({ app }) => {
    await nav(app, "/transactions");
    // The entry point lives in the toolbar's ⋯ menu; its handler is live even
    // though that menu's open state is a separate defect (C541).
    await openVia(
      app,
      app.locator('[data-testid="txn-smartcat-btn"]'),
      app.getByTestId("smartcat-no-provider"),
    );
    const notice = app.getByTestId("smartcat-no-provider");
    // Telling someone to add a key in Settings without a way to get to Settings
    // is indistinguishable from the feature being broken.
    const connect = app.getByTestId("smartcat-connect");
    await expect(connect).toBeVisible();
    await connect.click();

    // It must land where the key actually lives, not on Settings' default tab.
    await expect(app).toHaveURL(/\/settings\/ai/);
  });

  test("the selected mode is visibly selected", async ({ app }) => {
    await nav(app, "/transactions");
    await app.evaluate(() => document.querySelector('[data-testid="txn-smartcat-btn"]').click());
    const active = app.locator(".seg-btn.active");
    await expect(active).toHaveCount(1);
    // An outline alone reads as a focus ring on a dark surface; the active tab
    // has to carry a fill so a glance can tell which mode is chosen.
    const bg = await active.evaluate((el) => getComputedStyle(el).backgroundColor);
    expect(bg).not.toBe("rgba(0, 0, 0, 0)");
  });
});
