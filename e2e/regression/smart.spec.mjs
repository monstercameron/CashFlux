// smart.spec.mjs — the Smart layer's top-bar trigger (2026-07-17 visual audit P0).
// The per-page insight strip renders NOTHING by default: its icon+count trigger
// lives in the top bar, and clicking it opens the full insight card at the top of
// the content column (inside the stable slot wrapper). Collapse restores the
// zero-footprint default, and navigation resets the strip to collapsed.
import { test, expect, nav } from "./fixtures.mjs";

// Routes the seeded dataset tends to surface Free-engine insights on. We probe a
// few and use the first that renders the top-bar trigger, so the test doesn't
// hard-code which page the seed happens to light up.
const PROBE = ["/accounts", "/transactions", "/budgets", "/goals", "/planning", "/subscriptions", "/bills", "/"];

test.describe("smart trigger", () => {
  test("the top-bar trigger opens the insight card at the top of the content column", async ({ app }) => {
    // Find a route whose Smart layer is active (renders the top-bar trigger).
    let key = null;
    let route = null;
    for (const r of PROBE) {
      await nav(app, r);
      const peek = app.locator('[data-testid^="smart-peek-"]').first();
      if (await peek.count()) {
        key = (await peek.getAttribute("data-testid")).replace("smart-peek-", "");
        route = r;
        break;
      }
    }
    expect(key, "at least one seeded route surfaces the Smart trigger").not.toBeNull();

    const slot = app.getByTestId(`smart-strip-slot-${key}`);
    const peek = app.getByTestId(`smart-peek-${key}`);
    await expect(peek).toBeVisible();
    await expect(peek).toHaveAttribute("aria-expanded", "false");

    // The trigger sits in the top bar — near the very top of the viewport.
    const peekBox = await peek.boundingBox();
    expect(peekBox.y, "trigger lives in the top bar").toBeLessThan(120);

    // Collapsed default: the page carries NO strip card at all.
    await expect(slot.getByTestId(`smart-strip-${key}`)).toHaveCount(0);

    // Expand: click the trigger.
    await peek.click();

    // The open card renders INSIDE the stable slot wrapper at the top of the
    // content column; the trigger stays mounted as an expanded toggle.
    const card = slot.getByTestId(`smart-strip-${key}`);
    await expect(card).toBeVisible();
    await expect(peek).toHaveAttribute("aria-expanded", "true");
    const cardBox = await card.boundingBox();
    expect(cardBox.y, "the card opens at the top of the content column").toBeLessThan(400);

    // Collapse from the card header restores the zero-footprint default.
    await app.getByTestId("smart-strip-collapse").click();
    await expect(slot.getByTestId(`smart-strip-${key}`)).toHaveCount(0);
    await expect(peek).toHaveAttribute("aria-expanded", "false");

    // Navigation resets the strip to collapsed: open it, leave, come back.
    await peek.click();
    await expect(slot.getByTestId(`smart-strip-${key}`)).toBeVisible();
    await nav(app, "/settings");
    await nav(app, route);
    await expect(app.getByTestId(`smart-strip-slot-${key}`).getByTestId(`smart-strip-${key}`)).toHaveCount(0);
  });
});
