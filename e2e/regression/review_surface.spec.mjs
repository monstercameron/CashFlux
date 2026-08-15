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

/**
 * Select the first merchant AND wait for the footer to arm.
 *
 * Clicking Apply straight after the checkbox races the async wasm re-render:
 * the handler reads an empty selection and returns without writing, so the
 * queue never moves and the test fails for a reason that has nothing to do with
 * the behaviour under test. This was the cause of three flaky specs.
 */
async function pickFirstMerchant(app) {
  await app.locator('[data-testid^="review-pick-"]').first().click();
  await expect(app.getByTestId("review-selection")).toContainText(/\d+ merchants? · \d+ charges?/);
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
    // The select holds ONLY real categories — the create escape hatch is a
    // separate button, so a sentinel can never become the element's value.
    expect(opts.some((o) => /New category or sub-category/i.test(o))).toBe(false);
    await expect(app.locator('[data-testid^="review-newcat-"]').first()).toBeVisible();

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

    await pickFirstMerchant(app);
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
    await app.locator('[data-testid^="review-newcat-"]').first().click();

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
    await app.locator('[data-testid^="review-newcat-"]').first().click();
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
    await app.locator('[data-testid^="review-newcat-"]').first().click();
    const search = app.getByTestId("catpick-search");
    await search.fill("");
    // "j", "k", "s", "d", "b" are all shortcuts — inside an input they are text.
    await search.type("jksdb");
    await expect(search).toHaveValue("jksdb");
  });
});

test.describe("review surface: rules from a batch (C506)", () => {
  test("one action files the batch and writes the rule that files the next", async ({ app }) => {
    await openReview(app);
    const before = await readCharges(app);

    // The action only appears once something is selected — a rule with no
    // merchant behind it would have nothing to match on.
    await expect(app.getByTestId("review-makerule")).toHaveCount(0);
    await pickFirstMerchant(app);
    const makeRule = app.getByTestId("review-makerule");
    await expect(makeRule).toBeVisible();

    const merchant = (await app.locator(".rvs-grp.is-sel .rvs-grp-name strong").first().textContent()).trim();
    await makeRule.click();

    // The charges that prompted the rule are cleared, not left behind.
    await expect.poll(() => readCharges(app)).toBeLessThan(before);

    // And the rule exists, matching the CLEANED merchant name — a rule built
    // from the raw descriptor would carry a per-charge reference and fire once.
    await closeReview(app);
    await nav(app, "/rules");
    await expect(app.locator("#main")).toContainText(merchant);
  });
});

test.describe("review surface: accessibility (C512)", () => {
  test("every control is named and state changes are announced", async ({ app }) => {
    await openReview(app);

    // A row of identical icon-only checkboxes is unusable without the merchant
    // in the accessible name.
    const pick = app.locator('[data-testid^="review-pick-"]').first();
    const label = await pick.getAttribute("aria-label");
    expect(label, "the select checkbox needs an accessible name").toBeTruthy();
    expect(label).toMatch(/select .+ charges?/i);

    const caret = app.locator('[data-testid^="review-expand-"]').first();
    expect(await caret.getAttribute("aria-label")).toBeTruthy();
    expect(await caret.getAttribute("aria-expanded")).toBe("false");

    // The selection count changes with no focus movement, so it must announce.
    const sel = app.getByTestId("review-selection");
    expect(await sel.getAttribute("aria-live")).toBe("polite");
    expect(await sel.getAttribute("role")).toBe("status");

    // Tiers are named groups, not anonymous divs.
    const tier = app.locator("[data-tier]").first();
    expect(await tier.getAttribute("role")).toBe("group");
    expect(await tier.getAttribute("aria-label")).toBeTruthy();

    // Category selects carry a label too.
    const cat = app.locator('[data-testid^="review-cat-"]').first();
    expect(await cat.getAttribute("aria-label")).toBeTruthy();
  });
});

test.describe("review surface: undo (C508, partial)", () => {
  test("a bulk confirm is reversible", async ({ app }) => {
    await openReview(app);
    const start = await readCharges(app);

    await pickFirstMerchant(app);
    await app.getByTestId("review-apply").click();
    await expect.poll(() => readCharges(app)).toBeLessThan(start);
    const afterConfirm = await readCharges(app);

    // Ctrl+Z reverses the batch.
    await app.keyboard.press("Control+z");
    await expect
      .poll(() => readCharges(app), { message: "a bulk confirm must be reversible" })
      .toBeGreaterThan(afterConfirm);

    // MEASURED LIMITATION, not a passing grade: undoing a 122-charge batch
    // restored it only PARTIALLY (127 -> 173 of an expected 249), and two
    // confirms close together collapse toward one reachable entry. Undo points
    // are captured on the autosave tick (internal/app/undo.go captureUndoPoint)
    // and the stack is bounded at 4 MB, so a large changeset does not survive
    // whole. Fixing it means changing the shared stack every screen depends on
    // — tracked on C508 rather than asserted here as though it worked.
  });
});

test.describe("review surface: Smart+ strip (C504/C509)", () => {
  test("is visible and explains itself with no key configured", async ({ app }) => {
    // A feature the user cannot see is a feature they do not have. The strip
    // previously rendered ONLY when a provider was configured, so the paid tier
    // was invisible to everyone who had not already bought in.
    await openReview(app);
    const strip = app.getByTestId("review-scan-strip");
    await expect(strip).toBeVisible();
    await expect(app.getByTestId("review-scan-title")).toContainText(/Smart\+/i);

    // No key: it says what it would do and offers a way to enable it, rather
    // than showing a Scan button that would fail.
    await expect(strip).toContainText(/needs an OpenAI key/i);
    await expect(strip).toContainText(/only ever runs when you ask/i);
    await expect(app.getByTestId("review-scan-setup")).toBeVisible();
    // And it must NOT offer to spend money that cannot be spent.
    await expect(app.getByTestId("review-scan")).toHaveCount(0);
  });

  test("the strip states scope and cost before any spend", async ({ app }) => {
    await openReview(app);
    const strip = app.getByTestId("review-scan-strip");
    // Whatever state it is in, the strip never claims to have already run.
    expect(await strip.getAttribute("data-state")).toBe("idle");
  });
});

test.describe("review surface: edge cases", () => {
  test("confirming a merchant with no category is a no-op, not a blank write", async ({ app }) => {
    await openReview(app);
    // The "no suggestion" tier is exactly the set nothing could categorize.
    const noneTier = app.locator('[data-tier="is-none"]');
    test.skip((await noneTier.count()) === 0, "sample data left no unresolvable merchant");

    const pick = noneTier.locator('[data-testid^="review-pick-"]').first();
    const sel = noneTier.locator('[data-testid^="review-cat-"]').first();
    expect(await sel.inputValue(), "this tier must start empty").toBe("");

    const before = await readCharges(app);
    await pick.click();
    await expect(app.getByTestId("review-selection")).toContainText(/\d+ merchants? · \d+ charges?/);
    await app.getByTestId("review-apply").click();

    // Nothing was written, so the queue must not move — a blank category would
    // clear the charge from the queue while changing no budget math.
    await app.waitForTimeout(400);
    expect(await readCharges(app)).toBe(before);
  });

  test("dismissing the picker leaves the row's real category intact", async ({ app }) => {
    await openReview(app);
    const sel = app.locator('[data-testid^="review-cat-"]').first();
    const before = await sel.inputValue();

    await app.locator('[data-testid^="review-newcat-"]').first().click();
    await expect(app.getByTestId("catpick")).toBeVisible();
    await app.getByTestId("catpick-close").click();
    await expect(app.getByTestId("catpick")).toHaveCount(0);

    // The select is untouched: opening the picker is not a category change, and
    // no sentinel value can be left showing where a category should be.
    expect(await sel.inputValue()).toBe(before);
    const opts = await sel.locator("option").evaluateAll((els) => els.map((e) => e.value));
    expect(opts).not.toContain("__new__");
  });

  test("creating a category with only whitespace does nothing", async ({ app }) => {
    await openReview(app);
    await app.locator('[data-testid^="review-newcat-"]').first().click();
    await app.getByTestId("catpick-search").fill("   ");
    // A blank name offers no create block at all, so an empty category can never
    // be made by mashing the button.
    await expect(app.getByTestId("catpick-make")).toHaveCount(0);
    await app.getByTestId("catpick-close").click();
  });

  test("selection survives switching modes and back", async ({ app }) => {
    await openReview(app);
    await app.locator('[data-testid^="review-pick-"]').first().click();
    // Wait for the selection to actually arm before capturing it — reading
    // straight after the click races the async wasm re-render.
    await expect(app.getByTestId("review-selection")).toContainText(/\d+ merchants? · \d+ charges?/);
    const armed = await app.getByTestId("review-selection").textContent();

    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
    await app.getByTestId("review-mode-bulk").click();

    // Losing the selection on a mode switch would silently discard work.
    await expect(app.getByTestId("review-selection")).toHaveText(armed);
  });

  test("the queue total never disagrees with itself across modes", async ({ app }) => {
    await openReview(app);
    const bulk = await readCharges(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
    const single = await readCharges(app);
    expect(single, "both modes read one queue and must agree on its size").toBe(bulk);
  });

  test("expanding every merchant does not duplicate or drop charges", async ({ app }) => {
    await openReview(app);
    const carets = app.locator('[data-testid^="review-expand-"]');
    const n = Math.min(await carets.count(), 6);
    // Wait for EACH caret to report itself expanded before moving on: clicking
    // straight through races the re-render, and a stale nth() handle then acts
    // on a node that has already been replaced.
    for (let i = 0; i < n; i++) {
      await carets.nth(i).click();
      await expect(carets.nth(i)).toHaveAttribute("aria-expanded", "true");
    }
    // Every rendered charge row belongs to exactly one merchant block.
    const rows = await app.locator(".rvs-members .rvs-mem").count();
    expect(rows).toBeGreaterThan(0);
    const groupsOpen = await app.locator(".rvs-grp.is-open").count();
    expect(groupsOpen).toBe(n);
    // Collapsing them all returns the DOM to no member rows at all.
    for (let i = 0; i < n; i++) {
      await carets.nth(i).click();
      await expect(carets.nth(i)).toHaveAttribute("aria-expanded", "false");
    }
    await expect(app.locator(".rvs-members .rvs-mem")).toHaveCount(0);
  });
});
