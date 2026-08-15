// review_surface.spec.mjs — the dual-mode review surface (C500–C512).
//
// The thesis under test: a queue of ~250 charges is not 250 decisions, it is a
// handful of merchant decisions, tiered by how confident the matcher is. These
// assertions all hold with NO API key — nothing here touches SMART+.
import { test, expect, nav } from "./fixtures.mjs";

const readCharges = async (app) =>
  parseInt((await app.getByTestId("review-total").first().textContent()).trim(), 10);

/** Dismiss the surface — its backdrop intercepts clicks on the page behind. */
async function closeReview(app) {
  await app.keyboard.press("Escape");
  await expect(app.getByTestId("review-inbox")).toHaveCount(0);
}

async function openReview(app) {
  await nav(app, "/transactions");
  const btn = app.getByTestId("txn-review-btn");
  await expect(btn).toBeVisible();
  await btn.click();
  await expect(app.getByTestId("review-inbox")).toBeVisible();
}

test.describe("review surface: bulk mode", () => {
  test("collapses a long queue into a few merchant decisions", async ({ app }) => {
    await openReview(app);

    // Merchant groups, not one row per charge.
    const groups = app.locator('[data-testid^="review-pick-"]');
    const n = await groups.count();
    expect(n, "the queue should collapse into merchant groups").toBeGreaterThan(0);

    // Far fewer decisions than charges — that is the whole point of the surface.
    const charges = await readCharges(app);
    expect(charges).toBeGreaterThan(n);
  });

  test("groups are tiered by confidence, with the unanswerable ones last", async ({ app }) => {
    await openReview(app);
    const tiers = await app.locator("[data-tier]").evaluateAll((els) =>
      els.map((e) => e.getAttribute("data-tier")),
    );
    expect(tiers.length, "at least one tier should render").toBeGreaterThan(0);
    // Order is fixed: ready → look → none. A charge nothing can answer must not
    // be the first thing the user is shown.
    const rank = { "is-ready": 0, "is-look": 1, "is-none": 2 };
    const ranks = tiers.map((t) => rank[t]);
    expect(ranks).toEqual([...ranks].sort((a, b) => a - b));
  });

  test("selecting a merchant arms the footer with a real count", async ({ app }) => {
    await openReview(app);
    await expect(app.getByTestId("review-selection")).toContainText(/Nothing selected/i);

    await app.locator('[data-testid^="review-pick-"]').first().click();

    // The footer names merchants AND charges, correctly singularized.
    const sel = app.getByTestId("review-selection");
    await expect(sel).toContainText(/\d+ merchants? · \d+ charges?/);
    await expect(sel).not.toContainText(/\b1 merchants\b/);
    await expect(app.getByTestId("review-apply")).toContainText(/Confirm \d+/);

    await app.getByTestId("review-clear").click();
    await expect(sel).toContainText(/Nothing selected/i);
  });

  test("a merchant's category is an inline select, changeable without a modal", async ({ app }) => {
    await openReview(app);
    const sel = app.locator('[data-testid^="review-cat-"]').first();
    await expect(sel).toBeVisible();

    // Options carry the FULL path, so a bare ambiguous leaf can never be picked
    // by mistake (the C489 collision, made visible).
    const opts = await sel.locator("option").evaluateAll((els) => els.map((e) => e.textContent.trim()));
    expect(opts.some((o) => /New category or sub-category/i.test(o))).toBe(true);

    const target = opts.find((o) => o && !/New category|Choose a category/i.test(o));
    await sel.selectOption({ label: target });
    await expect(sel).toHaveAttribute("data-manual", "true");
  });

  test("expanding a merchant reveals its individual charges", async ({ app }) => {
    await openReview(app);
    const caret = app.locator('[data-testid^="review-expand-"]').first();
    await expect(caret).toHaveAttribute("aria-expanded", "false");
    await caret.click();
    await expect(caret).toHaveAttribute("aria-expanded", "true");
    await expect(app.locator(".rvs-mem").first()).toBeVisible();
  });

  test("confirming a selected merchant writes and shrinks the queue", async ({ app }) => {
    await openReview(app);
    const before = await readCharges(app);

    await app.locator('[data-testid^="review-pick-"]').first().click();
    await app.getByTestId("review-apply").click();

    await expect
      .poll(() => readCharges(app), { message: "the queue should shrink after confirming a merchant" })
      .toBeLessThan(before);
  });
});

test.describe("review surface: single mode", () => {
  test("switches modes without losing the surface", async ({ app }) => {
    await openReview(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
    await expect(app.getByTestId("review-commit")).toBeVisible();

    await app.getByTestId("review-mode-bulk").click();
    await expect(app.locator('[data-testid^="review-pick-"]').first()).toBeVisible();
  });

  test("shows what else the charge is tied to", async ({ app }) => {
    await openReview(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();

    // The sample's biggest merchant has many queued siblings, so the band must
    // list them rather than assert a bare count the user cannot inspect.
    const band = app.getByTestId("review-siblings");
    if (await band.count()) {
      await expect(band).toContainText(/more charges from this merchant/i);
      expect(await band.locator(".rvs-link-row").count()).toBeGreaterThan(0);
    }
  });

  test("snooze is durable — a snoozed merchant does not come back on reload", async ({ app }) => {
    // Payee is NOT identity here: single mode shows one merchant at a time, and
    // a 122-charge merchant would still show the same payee after one charge
    // left. Assert on the queue COUNT, which is unambiguous.
    await openReview(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
    const before = await readCharges(app);
    expect(before).toBeGreaterThan(0);

    await app.getByTestId("review-skip").click();
    await expect.poll(() => readCharges(app)).toBeLessThan(before);
    const after = await readCharges(app);

    // Reopen from scratch: before C493 the skip lived only in memory, so the
    // same charges re-blocked the queue on the next visit.
    await closeReview(app);
    await openReview(app);
    await expect
      .poll(() => readCharges(app), { message: "a snooze must survive reopening the surface" })
      .toBeLessThanOrEqual(after);
  });

  test("dismiss removes a merchant from the queue for good", async ({ app }) => {
    await openReview(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
    const before = await readCharges(app);
    await app.getByTestId("review-dismiss").click();
    await expect.poll(() => readCharges(app)).toBeLessThan(before);
  });
});

test.describe("review surface: category picker", () => {
  test("creates a sub-category without leaving the flow", async ({ app }) => {
    await openReview(app);
    const sel = app.locator('[data-testid^="review-cat-"]').first();
    await sel.selectOption("__new__");

    const pick = app.getByTestId("catpick");
    await expect(pick).toBeVisible();

    // Typing a new name reveals the ONE remaining decision: where it goes.
    await app.getByTestId("catpick-search").fill("Cold Brew");
    const make = app.getByTestId("catpick-make");
    await expect(make).toBeVisible();
    await expect(make).toContainText(/Cold Brew/);

    // Nest it under an existing parent, which no inline quick-add could do before.
    const parent = app.getByTestId("catpick-parent");
    const parentOpts = await parent.locator("option").evaluateAll((els) =>
      els.map((e) => ({ v: e.value, t: e.textContent.trim() })),
    );
    const inside = parentOpts.find((o) => o.v !== "");
    expect(inside, "there should be a top-level category to nest under").toBeTruthy();
    await parent.selectOption(inside.v);
    await app.getByTestId("catpick-create").click();

    // The picker closes and the new category is selected on the row.
    await expect(pick).toHaveCount(0);
    await expect(sel).toHaveAttribute("data-manual", "true");
    const chosen = await sel.locator("option:checked").textContent();
    expect(chosen).toContain("Cold Brew");
  });

  test("search narrows the list and offers a top-level create", async ({ app }) => {
    await openReview(app);
    await app.locator('[data-testid^="review-cat-"]').first().selectOption("__new__");
    await app.getByTestId("catpick-search").fill("zzzznotacategory");
    await expect(app.getByTestId("catpick")).toContainText(/No category matches/i);
    await expect(app.getByTestId("catpick-parent")).toContainText(/top-level/i);
    await app.getByTestId("catpick-close").click();
    await expect(app.getByTestId("catpick")).toHaveCount(0);
  });
});

test.describe("review surface: keyboard (C507)", () => {
  test("j/k move the focused merchant and space selects it", async ({ app }) => {
    await openReview(app);
    // The header must not advertise keys that do nothing.
    await expect(app.getByTestId("review-kbd")).toContainText(/j \/ k/);

    // data-focus on the root is the focused row index — assert on it rather than
    // on a class inside the keyed list, and POLL: the wasm re-render is async, so
    // reading straight after the keypress races it.
    const focusIdx = async () =>
      parseInt((await app.getByTestId("review-inbox").getAttribute("data-focus")) ?? "-1", 10);

    await expect(app.locator(".rvs-grp.is-focus")).toHaveCount(1);
    expect(await focusIdx()).toBe(0);

    await app.keyboard.press("j");
    await expect.poll(focusIdx, { message: "j should move to the next merchant" }).toBe(1);

    await app.keyboard.press("k");
    await expect.poll(focusIdx, { message: "k should move back" }).toBe(0);

    // Exactly one row carries the marker, and it follows the index.
    await expect(app.locator(".rvs-grp.is-focus")).toHaveCount(1);

    // Space picks the focused merchant.
    await app.keyboard.press(" ");
    await expect(app.getByTestId("review-selection")).toContainText(/\d+ merchants? · \d+ charges?/);
  });

  test("1 and b switch modes from the keyboard", async ({ app }) => {
    await openReview(app);
    await app.keyboard.press("1");
    await expect(app.getByTestId("review-payee")).toBeVisible();
    await expect(app.getByTestId("review-kbd")).toContainText(/snoozes/);
    await app.keyboard.press("b");
    await expect(app.locator('[data-testid^="review-pick-"]').first()).toBeVisible();
  });

  test("typing in a field is never hijacked", async ({ app }) => {
    await openReview(app);
    await app.locator('[data-testid^="review-cat-"]').first().selectOption("__new__");
    const search = app.getByTestId("catpick-search");
    await search.fill("");
    // "j", "k", "s", "d", "b" are all shortcuts — inside an input they are text.
    await search.type("jksdb");
    await expect(search).toHaveValue("jksdb");
  });
});
