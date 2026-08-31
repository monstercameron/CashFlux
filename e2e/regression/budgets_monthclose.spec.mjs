// budgets_monthclose.spec.mjs — the month-end review flow (2026-08-31).
//
// Five defects were fixed here at once, and four of them are SILENT: nothing
// throws, nothing renders empty, and every figure stays arithmetically correct.
// They can only be caught by asserting on wording and on what survives a click,
// which is exactly what this file does.
//
//   1. The flow ejected you at the first action. Four of its six controls closed
//      the review before opening the surface they pointed at, so a guided walk
//      through the month could not be walked.
//   2. The offer appeared in every period's final days whether or not there was
//      anything to review — Summary.Clean() existed for this and had no
//      production caller, only tests.
//   3. Step 5 carried the PREVIOUS period's top-ups into the period being
//      reviewed. Correct at a period's start; the offer only appears at its end.
//      Unit-pinned in monthclose.TestCarryTargets; here we check the copy agrees.
//   4. Every line read in the past tense while the period was still running.
//   5. "Close out" promised a finality the flow never had, and called budgets
//      "categories".
import { test, expect } from "@playwright/test";
import { bootAt } from "./fixtures.mjs";

const OFFER = "[data-testid=budgets-monthclose-offer]";
const DIALOG = "[role=dialog]";

// The seeded household runs on July 2026 (fixtures.FIXED_NOW). July ends Aug 1,
// so "the last five days" is Jul 27 onward — these two instants sit either side
// of that edge and are the only reason the clock moves in this file.
const MID_PERIOD = "2026-07-05T12:00:00.000Z";
const NEAR_END = "2026-07-29T12:00:00.000Z";

async function openBudgets(page) {
  await page.evaluate(() => {
    history.pushState({}, "", "/budgets");
    dispatchEvent(new PopStateEvent("popstate"));
  });
  await page.waitForSelector("#main[data-route='/budgets']", { timeout: 45_000 });
  await expect(page.getByTestId("budgets-status-strip")).toBeVisible({ timeout: 45_000 });
}

// reviewText returns the review dialog's text, or "" when it is not open. The
// month-close dialog is identified by its own title so a stacked sub-modal (the
// point of the first test below) can never be mistaken for it.
async function reviewText(page) {
  const dialogs = await page.locator(DIALOG).allInnerTexts();
  const mine = dialogs.find((t) => /Review .*\d{4}/.test(t));
  return (mine || "").replace(/\s+/g, " ").trim();
}

test.describe("budgets · month-end review", () => {
  test("the offer stays away until the period is nearly over", async ({ page }) => {
    // Timing gate only — 27 days of July still to run. This half of the gate was
    // always correct; it is here so a change to the Clean() gate below cannot
    // quietly widen the window as a side effect.
    await bootAt(page, MID_PERIOD);
    await openBudgets(page);
    await expect(page.locator(OFFER)).toHaveCount(0);
  });

  test("the offer names a review, and only appears with something to review", async ({ page }) => {
    await bootAt(page, NEAR_END);
    await openBudgets(page);

    const offer = page.locator(OFFER);
    const shown = (await offer.count()) > 0;

    // Both branches are assertions, not an escape hatch: the gate has to hold in
    // BOTH directions or it is not a gate. When the chip is hidden the page must
    // agree that there is nothing to review — no over-budget attention chip and
    // no over-assignment on the rail.
    if (!shown) {
      await expect(page.getByTestId("budgets-hero-attn")).toHaveCount(0);
      await expect(page.getByTestId("budgets-rail-resolve")).toHaveCount(0);
      return;
    }

    // #5: it is a review, not a closure. Nothing here locks a period.
    await expect(offer).toHaveText(/review the month/i);
    await expect(offer).not.toHaveText(/close out/i);

    await offer.click();
    await expect.poll(() => reviewText(page), { timeout: 20_000 }).toMatch(/Review /);
    const text = await reviewText(page);

    // #5: the title follows the button, and budgets are not categories.
    expect(text).toMatch(/^Review \w+ \d{4}/);
    expect(text).not.toMatch(/categories went over|categories are over/i);

    // #2 (the other direction): it opened because something needs a decision, so
    // it must NOT be reporting an all-clear on both counts.
    const allClear =
      /Nothing (is over budget|ended over budget)/i.test(text) &&
      /plan fits the expected income/i.test(text);
    expect(allClear, "the offer appeared on a period with nothing to review").toBe(false);
  });

  test("a live period is never described in the past tense", async ({ page }) => {
    // Jul 29: two days left, so money not yet spent is not money that "went"
    // unspent. The rest of this surface already switches tense on a closed
    // period; this flow did not.
    await bootAt(page, NEAR_END);
    await openBudgets(page);
    test.skip((await page.locator(OFFER).count()) === 0, "nothing to review in the seeded period");

    await page.locator(OFFER).click();
    await expect.poll(() => reviewText(page), { timeout: 20_000 }).toMatch(/Review /);
    const text = await reviewText(page);

    expect(text, "past-tense copy on a period that is still running").not.toMatch(
      /went over|went unspent|ended over budget|ended with money left/i,
    );
    expect(text).toMatch(/so far|still unspent|right now/i);
  });

  test("step 5 carries top-ups forward, never back into the period under review", async ({ page }) => {
    // The direction itself is pinned by monthclose.TestCarryTargets. What this
    // guards is the pairing: copy that still says "into this period" beside a
    // plan that now writes to the next one would be worse than either bug alone.
    await bootAt(page, NEAR_END);
    await openBudgets(page);
    test.skip((await page.locator(OFFER).count()) === 0, "nothing to review in the seeded period");

    await page.locator(OFFER).click();
    await expect.poll(() => reviewText(page), { timeout: 20_000 }).toMatch(/Review /);
    const text = await reviewText(page);

    expect(text).toMatch(/next period/i);
    expect(text).not.toMatch(/carry the rest into this period|last period's top-ups/i);
  });

  test("acting on a step leaves the review standing", async ({ page }) => {
    // The headline fix. Opening the income basis from step 3 used to dismiss the
    // review first, so the "guided pass" ended at the first thing you touched.
    await bootAt(page, NEAR_END);
    await openBudgets(page);
    test.skip((await page.locator(OFFER).count()) === 0, "nothing to review in the seeded period");

    await page.locator(OFFER).click();
    await expect.poll(() => reviewText(page), { timeout: 20_000 }).toMatch(/Review /);

    const income = page.getByTestId("monthclose-resolve-income");
    test.skip((await income.count()) === 0, "seeded period is not over-assigned");

    await income.click();
    // Two dialogs: the basis modal on top, the review still mounted beneath.
    await expect(page.locator(DIALOG)).toHaveCount(2, { timeout: 20_000 });
    await expect.poll(() => reviewText(page), { timeout: 10_000 }).toMatch(/Review /);

    // Backing out of the sub-modal returns to the step you were on rather than
    // to the page — Cancel, never Save, so the basis is left exactly as found.
    await page.getByRole("button", { name: "Cancel" }).first().click();
    await expect(page.locator(DIALOG)).toHaveCount(1, { timeout: 20_000 });
    expect(await reviewText(page)).toMatch(/Review /);
  });
});
