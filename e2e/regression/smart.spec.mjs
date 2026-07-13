// smart.spec.mjs — the per-page Smart strip's collapsed "peek" affordance. The
// strip defaults to a slim one-line peek bar (near-zero vertical footprint) and
// expands to the full insight card ON CLICK. This guards the reconciler regression
// where the strip's root element flipped <button>↔<div> across the peek/card swap
// and GWC re-anchored the replacement at the BOTTOM of the page (or dropped it):
// the fix wraps both states in a stable <div> slot so the card opens IN PLACE.
import { test, expect, nav } from "./fixtures.mjs";

// Routes the seeded dataset tends to surface Free-engine insights on. We probe a
// few and use the first that renders a peek, so the test doesn't hard-code which
// page the seed happens to light up.
const PROBE = ["/accounts", "/transactions", "/budgets", "/goals", "/planning", "/subscriptions", "/bills", "/"];

test.describe("smart peek", () => {
  test("clicking the peek expands the full card in place — not at the page bottom", async ({ app }) => {
    // Find a route whose Smart strip is enabled (renders a collapsed peek).
    let key = null;
    for (const r of PROBE) {
      await nav(app, r);
      const peek = app.locator('[data-testid^="smart-peek-"]').first();
      if (await peek.count()) {
        key = (await peek.getAttribute("data-testid")).replace("smart-peek-", "");
        break;
      }
    }
    expect(key, "at least one seeded route surfaces a Smart peek").not.toBeNull();

    const slot = app.getByTestId(`smart-strip-slot-${key}`);
    const peek = app.getByTestId(`smart-peek-${key}`);
    await expect(slot).toBeVisible();
    await expect(peek).toBeVisible();

    // The collapsed peek sits near the TOP of the content column.
    const peekBox = await peek.boundingBox();
    expect(peekBox.y, "peek starts near the top of the page").toBeLessThan(400);

    // Expand: click the peek.
    await peek.click();

    // The open card renders INSIDE the same stable slot wrapper. If the reconciler
    // had orphaned it (the bug), it would not be a descendant of the slot at all.
    const card = slot.getByTestId(`smart-strip-${key}`);
    await expect(card).toBeVisible();
    // The peek is replaced in place, not left behind.
    await expect(peek).toHaveCount(0);

    // The card opens WHERE THE PEEK SAT — its top is close to the old peek top, and
    // nowhere near the page bottom (which is what the regression produced).
    const cardBox = await card.boundingBox();
    expect(
      Math.abs(cardBox.y - peekBox.y),
      "the opened card sits where the peek was (in place), not detached below",
    ).toBeLessThan(140);

    // Collapse restores the peek in the same slot.
    await app.getByTestId("smart-strip-collapse").click();
    await expect(app.getByTestId(`smart-peek-${key}`)).toBeVisible();
    await expect(slot.getByTestId(`smart-strip-${key}`)).toHaveCount(0);
  });
});
