// flip_inert.spec.mjs — Quick Add must open as a usable modal (C611).
//
// Quick Add uses the shared AccessibleOverlay, whose modal backdrop is
// `.cf-overlay__backdrop--modal`. It does not use FlipPanel's `.flip-backdrop`
// or its `.show` reveal class.
import { test, expect, nav, openVia, openAddTransaction } from "./fixtures.mjs";

// panelState reports what a user (and an assistive technology) would actually
// find, independent of how it was achieved.
async function panelState(app) {
  return app.evaluate(() => {
    const dialog = document.querySelector('[data-testid="txn-add-dialog"]');
    const backdrop = document.querySelector(".cf-overlay__backdrop--modal, .flip-backdrop");
    if (!dialog || !backdrop) return { present: false };
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
      revealed: cs.visibility === "visible" && cs.display !== "none",
      visibility: cs.visibility,
      opacity: cs.opacity,
      hitTestable,
    };
  });
}

test.describe("C611 · Quick Add modal is usable", { tag: "@prod" }, () => {
  test("quick-add from the dashboard is on screen once open", async ({ app }) => {
    // The dashboard is the heaviest route, so it is where the failure was first seen.
    await openAddTransaction(app, app.getByTestId("txn-add-dialog"));

    // The flip panel turns into place over a moment; hit-testing is only true once it settles.
    await expect.poll(async () => (await panelState(app)).hitTestable, { timeout: 10_000 }).toBe(true);
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
    await openAddTransaction(app, app.getByTestId("txn-add-dialog"));

    // The flip panel turns into place over a moment; hit-testing is only true once it settles.
    await expect.poll(async () => (await panelState(app)).hitTestable, { timeout: 10_000 }).toBe(true);
    const s = await panelState(app);
    expect(s.revealed).toBe(true);
    expect(s.visibility).toBe("visible");
    expect(s.hitTestable).toBe(true);
  });

  // Focus placement is a separate follow-up: AccessibleOverlay currently starts
  // on the sentence field rather than honoring Quick Add's Amount autofocus.
  test.fixme("opening Quick Add focuses the Amount field", async ({ app }) => {
    await nav(app, "/transactions");
    await openAddTransaction(app, app.getByTestId("txn-add-dialog"));

    const focus = await app.evaluate(() => {
      const el = document.activeElement;
      return {
        insidePanel: !!(el && el.closest && el.closest('[data-testid="txn-add-dialog"]')),
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
    await openAddTransaction(app, app.getByTestId("txn-add-dialog"));
    await expect(app.getByTestId("txn-add-amount")).toBeVisible();

    await app.keyboard.press("Escape");
    await expect(app.getByTestId("txn-add-amount")).toHaveCount(0);

    const landedInPanel = await app.evaluate(() => {
      const el = document.activeElement;
      return !!(el && el.closest && el.closest('[data-testid="txn-add-dialog"]'));
    });
    expect(landedInPanel).toBe(false);
  });
});
