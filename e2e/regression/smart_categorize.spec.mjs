// smart_categorize.spec.mjs — SMART (local, non-LLM) category suggestion in the
// review inbox: the merchant dictionary and correction history resolving a
// charge, the evidence line that explains WHY a category was proposed, and the
// batch flag being honoured by the suggestion button.
//
// Nothing here touches SMART+ / OpenAI: every assertion must hold with no API
// key configured and no network. If one of these ever needs a key, it is in the
// wrong file.
//
// Scope note: the 3-observation learning THRESHOLD is proved by unit tests
// (internal/learntally, internal/catsuggest). Driving it through the UI would
// mean walking a 250-item queue hunting for repeat merchants, which buys no
// extra confidence and a lot of flake.
import { test, expect, nav } from "./fixtures.mjs";

/** Open the review inbox from the transactions toolbar. */
async function openReviewInbox(app) {
  await nav(app, "/transactions");
  const btn = app.getByTestId("txn-review-btn");
  await expect(btn).toBeVisible();
  await btn.click();
  await expect(app.getByTestId("review-inbox")).toBeVisible();
}

/**
 * Step through the queue until the head charge has a SMART suggestion.
 *
 * The queue is ordered newest-first, NOT by confidence, so the head is often a
 * genuinely unresolvable charge — the sample's first item is a Venmo P2P
 * transfer, which correctly gets no suggestion (payment processors are stripped,
 * never guessed at). Skipping to a resolvable one is the test's job, not a
 * workaround. Confidence ordering is C494.
 */
async function skipToASuggestion(app, maxSkips = 25) {
  for (let i = 0; i < maxSkips; i++) {
    if ((await app.getByTestId("review-suggest").count()) > 0) return true;
    const skip = app.getByTestId("review-skip");
    if ((await skip.count()) === 0) return false;
    await skip.click();
    await expect(app.getByTestId("review-payee")).toBeVisible();
  }
  return false;
}

test.describe("SMART categorization (local, no LLM)", () => {
  test("suggests a category and says where the suggestion came from", async ({ app }) => {
    await openReviewInbox(app);
    const found = await skipToASuggestion(app);
    expect(found, "no charge in the first 25 was resolvable by any local source").toBe(true);

    const suggest = app.getByTestId("review-suggest");
    await expect(suggest).toBeVisible();

    // The suggestion names a real category, not a placeholder.
    await expect(suggest).toContainText(/Suggested: \S+/);

    // It carries provenance, so tiering and the UI can both be trusted.
    const source = await suggest.getAttribute("data-source");
    expect(["rule", "history-exact", "history-merchant", "dictionary"]).toContain(source);

    // And it explains itself in plain English rather than asserting.
    const why = app.getByTestId("review-suggest-why");
    await expect(why).toBeVisible();
    const text = (await why.textContent()).trim();
    expect(text.length).toBeGreaterThan(0);
    // Never call a local heuristic "AI" — that word belongs to SMART+ only.
    expect(text).not.toMatch(/\bAI\b/i);
  });

  test("applying the suggestion categorizes the charge and advances", async ({ app }) => {
    await openReviewInbox(app);
    expect(await skipToASuggestion(app)).toBe(true);

    // Compare the live queue COUNT, not the payee: the next charge is very often
    // another one from the same merchant, so an unchanged payee does not mean the
    // inbox is stuck.
    const readLeft = async () => {
      if ((await app.getByTestId("review-done").count()) > 0) return 0;
      const txt = await app.getByTestId("review-progress").textContent();
      return parseInt(txt.match(/(\d+)\s*left/)?.[1] ?? "-1", 10);
    };
    const leftBefore = await readLeft();
    expect(leftBefore).toBeGreaterThan(0);

    await app.getByTestId("review-suggest").click();

    // The queue shrank, proving the write landed and the inbox did not freeze
    // on the same item with no feedback (QA CF-02).
    await expect
      .poll(readLeft, { message: "the queue should shrink after applying a suggestion" })
      .toBeLessThan(leftBefore);
  });

  test("the batch checkbox is honoured by the suggestion button, not just confirm", async ({ app }) => {
    // C496: "also apply to N others" used to be read ONLY by the confirm button,
    // so ticking it and then clicking the suggestion silently categorized ONE
    // transaction while looking like a batch.
    await openReviewInbox(app);

    // Need a charge that has BOTH same-merchant siblings and a suggestion.
    let ready = false;
    for (let i = 0; i < 30; i++) {
      const hasBoth =
        (await app.getByTestId("review-similar").count()) > 0 &&
        (await app.getByTestId("review-suggest").count()) > 0;
      if (hasBoth) {
        ready = true;
        break;
      }
      const skip = app.getByTestId("review-skip");
      if ((await skip.count()) === 0) break;
      await skip.click();
      await expect(app.getByTestId("review-payee")).toBeVisible();
    }
    test.skip(!ready, "no queued merchant in the first 30 has both siblings and a suggestion");

    const readLeft = async () => {
      if ((await app.getByTestId("review-done").count()) > 0) return 0;
      const txt = await app.getByTestId("review-progress").textContent();
      return parseInt(txt.match(/(\d+)\s*left/)?.[1] ?? "0", 10);
    };
    const leftBefore = await readLeft();

    await app.getByTestId("review-similar").locator('input[type="checkbox"]').check({ force: true });
    await app.getByTestId("review-suggest").click();

    // The batch applied: the queue dropped by more than the single charge.
    await expect
      .poll(readLeft, { message: "queue should drop by more than one when the batch box is ticked" })
      .toBeLessThan(leftBefore - 1);
  });

  test("a confirmed categorization survives a reload", async ({ app }) => {
    // The review inbox writes through the same persist path as every other
    // setting; a categorization that evaporates on refresh has not been saved.
    await openReviewInbox(app);

    const payee = (await app.getByTestId("review-payee").textContent()).trim();
    await app.getByTestId("review-category-select").selectOption({ label: "Groceries" });
    await app.getByTestId("review-commit").click();

    // The charge left the queue.
    await expect
      .poll(async () => {
        if ((await app.getByTestId("review-done").count()) > 0) return "finished";
        return (await app.getByTestId("review-payee").textContent()).trim();
      })
      .not.toBe(payee);

    // After a reload it is categorized in the ledger, not back in the queue.
    await nav(app, "/transactions");
    await app.locator('input[type="search"]').first().fill(payee);
    await expect(app.locator('[data-testid^="txn-row-"]').first()).toContainText(/Groceries/);
  });
});
