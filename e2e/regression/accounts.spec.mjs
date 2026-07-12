// accounts.spec.mjs — regressions for the accounts-page refinements: the page-level
// transfer flip modal, the ⋯-menu (Transactions moved in) + inline quick actions, the
// merged update-value/edit modal, and the readable notes line.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("accounts: transfer flip modal", () => {
  test("Transfer money opens a centered flip modal (not an inline tile)", async ({ app }) => {
    await nav(app, "/accounts");
    await app.getByTestId("page-transfer-btn").click();
    await app.waitForTimeout(650); // past the FlipPanel flip
    // The transfer form renders inside a dialog overlay, with both account pickers.
    await expect(app.locator('[role="dialog"]')).toBeVisible();
    await expect(app.getByTestId("page-transfer-form")).toBeVisible();
    await expect(app.getByTestId("page-xfer-from-select")).toBeVisible();
    await expect(app.getByTestId("page-xfer-to-select")).toBeVisible();
    // The accounts surface stays mounted behind the modal (not an in-place takeover).
    await expect(app.locator('.bento-accounts')).toBeVisible();
    // Escape dismisses.
    await app.keyboard.press("Escape");
    await expect(app.getByTestId("page-transfer-form")).toHaveCount(0);
  });
});

test.describe("accounts: kebab + quick actions", () => {
  test("Transactions lives in the ⋯ menu; no inline Transactions button", async ({ app }) => {
    await nav(app, "/accounts");
    const row = app.locator(".bento-accounts .row").first();
    await row.scrollIntoViewIfNeeded();
    // The inline quick actions are Edit (+ Update value for stale/valuation) — never a
    // standalone Transactions button.
    await expect(row.getByRole("button", { name: /^Transactions$/ })).toHaveCount(0);
    await expect(row.locator('[data-testid^="edit-account-btn-"]')).toBeVisible();
    // Open the ⋯ menu — the Transactions drill is now a menu item.
    await row.locator('.add-wrap > button[aria-haspopup="menu"]').click();
    const drill = row.locator('[data-testid^="acct-view-txns-"]');
    await expect(drill).toBeVisible();
    await drill.click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
  });

  test("the list-header Smart shortcut beside the class filter is gone", async ({ app }) => {
    await nav(app, "/accounts");
    // The class filter itself remains…
    await expect(app.getByTestId("acct-class-all")).toBeVisible();
    // …but the Smart sparkle section-action no longer sits in the accounts list header.
    await expect(app.locator('.bento-accounts [data-testid="smart-section-action"]')).toHaveCount(0);
  });
});

test.describe("accounts: merged edit + readable notes", () => {
  test("one modal edits details and updates the value; notes become a readable, expandable line", async ({ app }) => {
    await nav(app, "/accounts");
    const editBtn = app.locator('[data-testid^="edit-account-btn-"]').first();
    await editBtn.scrollIntoViewIfNeeded();
    const acctId = (await editBtn.getAttribute("data-testid")).replace("edit-account-btn-", "");
    await editBtn.click();
    await app.waitForTimeout(650); // flip
    const dialog = app.locator('[role="dialog"]');
    // The merged editor carries BOTH the value-update section and the detail fields.
    await expect(app.getByTestId("acct-value-section")).toBeVisible();
    await expect(dialog.locator('input#acct-edit-' + acctId)).toBeVisible(); // name field

    // Attach a note via the merged form and save.
    const note = "Refi locked at 5.9% until Aug — call the broker before it resets.";
    const notesArea = dialog.locator("textarea").first();
    await notesArea.fill(note);
    await expect(notesArea).toHaveValue(note);
    await dialog.locator('button[type="submit"]').click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    // The row now shows the note as a readable line (not a hover-only glyph), collapsed
    // by default and expandable on click.
    const notesLine = app.locator(`[data-testid="acct-notes-${acctId}"]`);
    await expect(notesLine).toBeVisible();
    await expect(notesLine).toContainText("Refi locked at 5.9%");
    await expect(notesLine).toHaveAttribute("aria-expanded", "false");
    await notesLine.click();
    await expect(notesLine).toHaveAttribute("aria-expanded", "true");
  });

  test("the Cancel/Save bar stays pinned while the edit form scrolls", async ({ app }) => {
    await nav(app, "/accounts");
    await app.locator('[data-testid^="edit-account-btn-"]').first().click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    const foot = dialog.locator(".modal-foot");
    await expect(foot).toBeVisible();
    await expect(foot.getByRole("button", { name: /save/i })).toBeVisible();
    // Scroll the field region to the bottom; the footer must stay put at the dialog bottom.
    const res = await dialog.evaluate((dlg) => {
      const s = dlg.querySelector(".modal-scroll");
      const f = dlg.querySelector(".modal-foot");
      s.scrollTop = s.scrollHeight;
      return {
        scrolled: s.scrollTop > 50,
        gap: Math.round(dlg.getBoundingClientRect().bottom - f.getBoundingClientRect().bottom),
      };
    });
    expect(res.scrolled, "the field region actually scrolled").toBeTruthy();
    expect(Math.abs(res.gap), "footer stays pinned to the dialog bottom").toBeLessThan(4);
  });

  test("entering a new value in the merged modal records a balance adjustment", async ({ app }) => {
    await nav(app, "/accounts");
    const editBtn = app.locator('[data-testid^="edit-account-btn-"]').first();
    await editBtn.scrollIntoViewIfNeeded();
    await editBtn.click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    // Type a new current value; the delta preview appears, proving the setbal path is live.
    await dialog.getByTestId("acct-setbal-input").fill("999999");
    await expect(dialog.getByTestId("setbal-delta-preview")).toBeVisible();
    await dialog.locator('button[type="submit"]').click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });
    // A toast confirms the balance was updated (OnSetBalance path fired).
    await expect(app.locator("body")).toContainText(/updated/i, { timeout: 15000 });
  });
});
