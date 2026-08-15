// smart_categorize.spec.mjs — SMART (local, non-LLM) categorization surfacing in
// the review surface: the merchant dictionary and correction history filling a
// category with no key configured, and the evidence line that says WHY.
//
// Nothing here touches SMART+ / OpenAI: every assertion must hold with no API
// key and no network. If one of these ever needs a key, it is in the wrong file.
//
// Scope note: the 3-observation learning THRESHOLD and the resolver's precedence
// are proved by unit tests (internal/learntally, internal/catsuggest). Driving
// them through the UI buys no extra confidence and a lot of flake.
import { test, expect, nav } from "./fixtures.mjs";

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

const readTotal = async (app) =>
  parseInt((await app.getByTestId("review-total").first().textContent()).trim(), 10);

test.describe("SMART categorization (local, no LLM)", () => {
  test("fills categories with no API key configured", async ({ app }) => {
    await openReview(app);

    // Every merchant group carries a category select. At least one must arrive
    // pre-filled from a local source — that is the entire point of Phase E, and
    // it must hold with no provider configured.
    const sels = app.locator('[data-testid^="review-cat-"]');
    const n = await sels.count();
    expect(n, "there should be merchant groups to categorize").toBeGreaterThan(0);

    let filled = 0;
    for (let i = 0; i < n; i++) {
      if (await sels.nth(i).inputValue()) filled++;
    }
    expect(filled, "local sources should pre-fill at least one merchant").toBeGreaterThan(0);
  });

  test("tiers reflect what the local sources could answer", async ({ app }) => {
    await openReview(app);
    // A merchant nothing could resolve belongs in the "no suggestion" tier with
    // an empty select — never guessed at.
    const noneTier = app.locator('[data-tier="is-none"]');
    if (await noneTier.count()) {
      const sel = noneTier.locator('[data-testid^="review-cat-"]').first();
      if (await sel.count()) {
        expect(await sel.inputValue()).toBe("");
      }
    }
    // A "ready" merchant must have a category chosen for it.
    const ready = app.locator('[data-tier="is-ready"] [data-testid^="review-cat-"]').first();
    if (await ready.count()) {
      expect(await ready.inputValue()).not.toBe("");
    }
  });

  test("single mode explains where a suggestion came from", async ({ app }) => {
    await openReview(app);
    await app.getByTestId("review-mode-single").click();
    await expect(app.getByTestId("review-payee")).toBeVisible();

    const why = app.getByTestId("review-suggest-why");
    if ((await why.count()) === 0) {
      test.skip(true, "the first merchant has no local suggestion to explain");
    }
    const text = (await why.textContent()).trim();
    expect(text.length).toBeGreaterThan(0);
    // Never call a local heuristic "AI" — that word belongs to SMART+ only.
    expect(text).not.toMatch(/\bAI\b/i);
    // It cites evidence rather than asserting.
    expect(text).toMatch(/rule|filed|merchant/i);
  });

  test("a confirmed categorization survives a reload", async ({ app }) => {
    await openReview(app);
    const before = await readTotal(app);
    expect(before).toBeGreaterThan(0);

    await pickFirstMerchant(app);
    await app.getByTestId("review-apply").click();
    await expect.poll(() => readTotal(app)).toBeLessThan(before);
    const after = await readTotal(app);

    // Reopen from scratch — the write must be in the ledger, not just the view.
    await closeReview(app);
    await openReview(app);
    await expect
      .poll(() => readTotal(app), { message: "a confirmed categorization must persist" })
      .toBeLessThanOrEqual(after);
  });
});
