// flip_inert.spec.mjs — a flip modal that is open must be usable (C611).
//
// A flip modal mounts unrevealed and a mount effect adds `.show` so the card
// animates in. Driving this directly shows that reveal is NOT reliable on a cold
// first open: with a single click the backdrop can still be `flip-backdrop` (no
// `.show`) more than a second later, and focus never enters the dialog. A second
// click — or any other event that re-renders — reveals it.
//
// These tests open through `openVia`, which RETRIES the click, so they assert the
// weaker but still meaningful property: once the panel is open it is genuinely
// usable — on screen, hit-testable, and leaving nothing behind when closed. The
// single-click reveal itself is tracked as C611 and is deliberately NOT asserted
// here; a test that passes only because the harness clicked twice would be
// claiming something untrue.
import { test, expect, nav, openVia } from "./fixtures.mjs";

// panelState reports what a user (and an assistive technology) would actually
// find, independent of how it was achieved.
async function panelState(app) {
  return app.evaluate(() => {
    const backdrop = document.querySelector(".flip-backdrop");
    if (!backdrop) return { present: false };
    const cs = getComputedStyle(backdrop);
    const amount = document.querySelector('[data-testid="txn-add-amount"]');
    let hitTestable = false;
    if (amount) {
      const r = amount.getBoundingClientRect();
      const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
      hitTestable = !!top && (top === amount || amount.contains(top) || top.contains(amount));
    }
    return {
      present: true,
      revealed: backdrop.classList.contains("show"),
      visibility: cs.visibility,
      opacity: cs.opacity,
      hitTestable,
    };
  });
}

test.describe("C611 · an open flip modal is usable", { tag: "@prod" }, () => {
  test("quick-add from the dashboard is on screen once open", async ({ app }) => {
    // The dashboard is the heaviest route, so it is where the reveal is slowest
    // and the failure was first seen.
    await openVia(app, app.getByTestId("add-transaction-btn"), app.locator(".flip-backdrop.show"));

    const s = await panelState(app);
    expect(s.present).toBe(true);
    expect(s.revealed).toBe(true);
    expect(s.visibility).toBe("visible");
    // Revealed means genuinely interactive: the field a user is about to type in
    // must be the thing under the pointer at its own centre, not page content.
    expect(s.hitTestable).toBe(true);
  });

  test("quick-add from the ledger is on screen once open", async ({ app }) => {
    await nav(app, "/transactions");
    await openVia(app, app.getByTestId("add-transaction-btn"), app.locator(".flip-backdrop.show"));

    const s = await panelState(app);
    expect(s.revealed).toBe(true);
    expect(s.visibility).toBe("visible");
    expect(s.hitTestable).toBe(true);
  });

  // BLOCKED BY C611. A dialog must take focus when it opens, or a keyboard/screen
  // reader user is left behind it. Driven directly, focus stays on the trigger
  // button — `document.activeElement` is still `add-transaction-btn` and is not
  // inside `.flip-backdrop`. It shares a root cause with the unreliable reveal
  // (the panel's mount effects not completing on a cold open), so it is recorded
  // rather than worked around: focusing on a timer would hide the real defect.
  test.fixme("opening a panel lands focus inside it", async ({ app }) => {
    await nav(app, "/transactions");
    await openVia(app, app.getByTestId("add-transaction-btn"), app.locator(".flip-backdrop.show"));

    const focus = await app.evaluate(() => {
      const el = document.activeElement;
      return {
        insidePanel: !!(el && el.closest && el.closest(".flip-backdrop")),
        testid: el ? el.getAttribute("data-testid") : null,
      };
    });
    expect(focus.insidePanel).toBe(true);
    // Quick-add marks its Amount field [autofocus] so the cursor lands on the
    // field a user actually fills first (C43).
    expect(focus.testid).toBe("txn-add-amount");
  });

  test("a closed panel leaves nothing focusable behind", async ({ app }) => {
    await nav(app, "/transactions");
    await openVia(app, app.getByTestId("add-transaction-btn"), app.getByTestId("txn-add-amount"));
    await expect(app.getByTestId("txn-add-amount")).toBeVisible();

    await app.keyboard.press("Escape");
    await expect(app.getByTestId("txn-add-amount")).toHaveCount(0);

    const landedInPanel = await app.evaluate(() => {
      const el = document.activeElement;
      return !!(el && el.closest && el.closest(".flip-backdrop"));
    });
    expect(landedInPanel).toBe(false);
  });
});
