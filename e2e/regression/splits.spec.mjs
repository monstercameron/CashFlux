// splits.spec.mjs — the split-transaction editor: percentage entry mode
// (Amounts/Percentages toggle, mode conversion, 100% balance gate, save).
import { test, expect, nav } from "./fixtures.mjs";

// Opens the split flip modal from a mid-list row's ⋯ menu (nth 6 avoids the
// sticky-chrome click hazard, like interactions.spec) and returns the row id so
// callers can reopen the SAME transaction. Skips transfer rows (no split item).
async function openSplitEditor(app, rowId) {
  await nav(app, "/transactions");
  let row;
  if (rowId) {
    row = app.locator(`[data-testid="${rowId}"]`);
  } else {
    for (let i = 6; i < 12; i++) {
      row = app.locator('[data-testid^="txn-row-"]').nth(i);
      await row.scrollIntoViewIfNeeded();
      await row.locator('[data-testid^="txn-kebab-"]').click();
      if (await row.locator('[data-testid="txn-split-open"]').isVisible()) break;
      await app.keyboard.press("Escape");
      row = null;
    }
  }
  if (rowId) {
    await row.scrollIntoViewIfNeeded();
    await row.locator('[data-testid^="txn-kebab-"]').click();
  }
  await row.locator('[data-testid="txn-split-open"]').click();
  await expect(app.getByTestId("split-editor")).toBeVisible();
  await app.waitForTimeout(600); // past the FlipPanel flip
  return await row.getAttribute("data-testid");
}

test.describe("transactions: percent split mode", () => {
  test("Percentages mode converts the draft, gates on 100%, and saves exact amounts", async ({ app }) => {
    const rowId = await openSplitEditor(app);
    // Amounts is the default mode.
    await expect(app.getByTestId("split-mode-amount")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("split-mode-percent")).toHaveAttribute("aria-pressed", "false");
    // Switching converts the seeded whole-amount line into its percentage: 100.00.
    await app.getByTestId("split-mode-percent").click();
    await expect(app.getByTestId("split-mode-percent")).toHaveAttribute("aria-pressed", "true");
    await expect(app.getByTestId("split-amt-0")).toHaveValue("100.00");
    // 60/40 across two categories — entered unbalanced first.
    await app.getByTestId("split-amt-0").fill("60");
    await app.getByTestId("split-amt-1").fill("30");
    await app.getByTestId("split-cat-1").selectOption({ index: 1 });
    // 60 + 30 = 90% → the remainder line demands the missing 10%.
    await expect(app.getByTestId("split-remainder")).toContainText(/10\.00% left/i);
    // Saving while unbalanced is rejected with the percent message.
    await app.getByTestId("split-save").click();
    await expect(app.getByTestId("split-editor")).toContainText(/must add up to 100%/i);
    // Balance it and save.
    await app.getByTestId("split-amt-1").fill("40");
    await expect(app.getByTestId("split-remainder")).toContainText(/balanced/i);
    await app.getByTestId("split-save").click();
    await expect(app.getByTestId("split-editor")).toHaveCount(0);
    // The editor round-trips: reopening the SAME transaction shows the split as
    // exact amounts that balance, in the 60/40 (1.5×) ratio.
    await openSplitEditor(app, rowId);
    await expect(app.getByTestId("split-mode-amount")).toHaveAttribute("aria-pressed", "true");
    const amt0 = parseFloat(await app.getByTestId("split-amt-0").inputValue());
    const amt1 = parseFloat(await app.getByTestId("split-amt-1").inputValue());
    expect(amt0).toBeGreaterThan(0);
    expect(amt1).toBeGreaterThan(0);
    await expect(app.getByTestId("split-remainder")).toContainText(/balanced/i);
    expect(Math.abs(amt0 - 1.5 * amt1)).toBeLessThan(0.05);
  });

  test("switching back to Amounts converts percentages into money", async ({ app }) => {
    await openSplitEditor(app);
    await app.getByTestId("split-mode-percent").click();
    await app.getByTestId("split-amt-0").fill("50");
    await app.getByTestId("split-mode-amount").click();
    await expect(app.getByTestId("split-mode-amount")).toHaveAttribute("aria-pressed", "true");
    // 50% became half the transaction amount — a plain decimal, not "50".
    const v = await app.getByTestId("split-amt-0").inputValue();
    expect(v).not.toBe("50");
    expect(parseFloat(v)).toBeGreaterThan(0);
  });
});
