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

test.describe("accounts: labeled toolbar buttons", () => {
  test("toolbar actions carry a visible text label; Add account anchors the group", async ({ app }) => {
    await nav(app, "/accounts");
    // Transfer is a standard labeled button (.btn-tool) that opens a flip modal: it
    // shows its text label inline (no hover needed) and aria-haspopup="dialog".
    const transfer = app.getByTestId("page-transfer-btn");
    await expect(transfer).toHaveClass(/btn-tool/);
    await expect(transfer).not.toHaveClass(/tbar-btn/);
    await expect(transfer).toContainText("Transfer money"); // label is visible, not hover-only
    await expect(transfer).toHaveAttribute("aria-haspopup", "dialog");
    // Page-local creation: "+ Add account" is the primary at the group's right end.
    const add = app.getByTestId("accounts-add");
    await expect(add).toBeVisible();
    await expect(add).toHaveClass(/btn-primary/);
    // 2026-07-17 audit: the management surfaces (Groups, Institutions, Sweep
    // rules, Exchange rates) live under ONE labeled "Manage" menu — the toolbar
    // stops presenting seven equally weighted verbs before the account list.
    const manage = app.getByTestId("acct-manage-btn");
    await expect(manage).toBeVisible();
    await expect(manage).toHaveClass(/btn-tool/);
    await expect(manage).toContainText("Manage");
    await manage.click();
    await expect(app.getByTestId("acct-groups-btn")).toBeVisible();
    await expect(app.getByTestId("acct-institutions-btn")).toBeVisible();
    // Exchange rates (when present) navigates to Settings from inside the menu.
    const fx = app.getByTestId("acct-fx-btn");
    if (await fx.isVisible()) {
      await fx.click();
      await expect(app.locator('#main[data-route="/settings"]').first()).toBeVisible();
    } else {
      await app.keyboard.press("Escape");
    }
  });
});

test.describe("accounts: row actions + type-aware kebab", () => {
  test("Transactions is inline; the balance figure opens the update editor", async ({ app }) => {
    await nav(app, "/accounts");
    const row = app.locator(".bento-accounts .row").first();
    await row.scrollIntoViewIfNeeded();
    // Transactions (high-frequency navigation) is a visible row button, beside Edit.
    const drill = row.locator('[data-testid^="acct-view-txns-"]');
    await expect(drill).toBeVisible();
    await expect(row.locator('[data-testid^="edit-account-btn-"]')).toBeVisible();
    // G2/C4: the balance figure itself is the one consistent update affordance.
    const balBtn = row.locator('[data-testid^="acct-balance-btn-"]');
    await expect(balBtn).toBeVisible();
    await balBtn.click();
    await app.waitForTimeout(650);
    await expect(app.locator('[role="dialog"]')).toBeVisible();
    await app.keyboard.press("Escape");
    await app.waitForTimeout(300);
    // Navigation still works from the inline button.
    await drill.click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
  });

  test("a property row offers no Reconcile/Transfer; a cash row offers both", async ({ app }) => {
    await nav(app, "/accounts");
    // The Condo (property): reconciling a valuation to a statement is nonsense.
    await app.locator('[data-testid="edit-account-btn-acct-home"] ~ .add-wrap > button').click();
    await expect(app.locator('.add-menu:not(.hidden-menu) [data-testid="reconcile-start-btn-acct-home"]')).toHaveCount(0);
    await expect(app.locator('.add-menu:not(.hidden-menu) [data-testid="transfer-start-btn-acct-home"]')).toHaveCount(0);
    await app.keyboard.press("Escape");
    await app.waitForTimeout(200);
    // Joint Checking (cash): both rituals available, plus quick institution assignment.
    await app.locator('[data-testid="edit-account-btn-acct-checking"] ~ .add-wrap > button').click();
    await expect(app.locator('.add-menu:not(.hidden-menu) [data-testid="reconcile-start-btn-acct-checking"]')).toBeVisible();
    await expect(app.locator('.add-menu:not(.hidden-menu) [data-testid="transfer-start-btn-acct-checking"]')).toBeVisible();
    await expect(app.locator('.add-menu:not(.hidden-menu) [data-testid="set-institution-acct-checking"]')).toBeVisible();
  });

  test("the page transfer form filters sources to liquid cash and previews FX", async ({ app }) => {
    await nav(app, "/accounts");
    await app.getByTestId("page-transfer-btn").click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    const fromOpts = await dialog.getByTestId("page-xfer-from-select").locator("option").allInnerTexts();
    // No 401(k)/loans/property/brokerage as transfer sources.
    expect(fromOpts.join("|")).not.toMatch(/401|Mortgage|Condo|Car Loan|Roth|Stonks|Student/i);
    // Liability destinations read as payments.
    const toOpts = await dialog.getByTestId("page-xfer-to-select").locator("option").allInnerTexts();
    expect(toOpts.some((s) => /payment/.test(s))).toBeTruthy();
    // USD → EUR shows the denomination + converted preview before anything posts.
    await dialog.getByTestId("page-xfer-from-select").selectOption({ label: fromOpts.find((s) => /Joint Checking/.test(s)) });
    const eur = (await dialog.getByTestId("page-xfer-to-select").locator("option").allInnerTexts()).find((s) => /Travel/.test(s));
    await dialog.getByTestId("page-xfer-to-select").selectOption({ label: eur });
    await dialog.getByTestId("page-xfer-amt").fill("100");
    await expect(dialog.locator('[data-testid="xfer-fx-note"]')).toContainText(/lands in EUR/i);
  });

  test("the transfer form previews both accounts' before/after balances", async ({ app }) => {
    await nav(app, "/accounts");
    await app.getByTestId("page-transfer-btn").click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    // No preview until both accounts and a valid amount exist.
    await expect(dialog.locator('[data-testid="xfer-balance-preview"]')).toHaveCount(0);
    const fromOpts = await dialog.getByTestId("page-xfer-from-select").locator("option").allInnerTexts();
    const fromLabel = fromOpts.find((s) => /Joint Checking/.test(s));
    await dialog.getByTestId("page-xfer-from-select").selectOption({ label: fromLabel });
    const toOpts = await dialog.getByTestId("page-xfer-to-select").locator("option").allInnerTexts();
    const toLabel = toOpts.find((s) => s.trim() && !/Joint Checking/.test(s) && !/^Choose/.test(s));
    await dialog.getByTestId("page-xfer-to-select").selectOption({ label: toLabel });
    await expect(dialog.locator('[data-testid="xfer-balance-preview"]')).toHaveCount(0); // still no amount
    await dialog.getByTestId("page-xfer-amt").fill("100");
    // Both sides show "<name>: <before> → <after>" from the same math the post uses.
    const fromLine = dialog.locator('[data-testid="xfer-preview-from"]');
    const toLine = dialog.locator('[data-testid="xfer-preview-to"]');
    await expect(fromLine).toBeVisible();
    await expect(toLine).toBeVisible();
    await expect(fromLine).toContainText("Joint Checking");
    await expect(fromLine).toContainText("→");
    await expect(toLine).toContainText("→");
    // Clearing the amount removes the preview again (no stale numbers).
    await dialog.getByTestId("page-xfer-amt").fill("");
    await expect(dialog.locator('[data-testid="xfer-balance-preview"]')).toHaveCount(0);
  });

  test("stale-balance controls: snooze-until persists and the exemption hides it", async ({ app }) => {
    await nav(app, "/accounts");
    // Edit lives in the row's ⋯ menu (the .add-wrap holds trigger + menu).
    const kebab = app.locator('.add-wrap:has([data-testid="edit-account-btn-acct-checking"]) > button');
    const editBtn = app.locator('[data-testid="edit-account-btn-acct-checking"]');
    await kebab.scrollIntoViewIfNeeded();
    await kebab.click();
    await editBtn.click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    await dialog.getByTestId("acct-edit-more").click();
    // Snooze a couple of weeks out, save.
    const snooze = dialog.getByTestId("acct-edit-fresh-snooze");
    await snooze.scrollIntoViewIfNeeded();
    await snooze.fill("2026-08-01");
    // Regression: the score inputs enforce the canonical 0-100 scale, so a
    // seeded account (liquidity 100) saves without touching those fields.
    await dialog.locator('button[type="submit"]').click();
    await app.waitForTimeout(650);
    // Reopen: the date persisted.
    await kebab.scrollIntoViewIfNeeded();
    await kebab.click();
    await editBtn.click();
    await app.waitForTimeout(650);
    await dialog.getByTestId("acct-edit-more").click();
    await expect(dialog.getByTestId("acct-edit-fresh-snooze")).toHaveValue("2026-08-01");
    // Ticking the exemption hides the snooze field (it no longer applies).
    await dialog.getByTestId("acct-edit-fresh-exempt").click({ force: true });
    await expect(dialog.getByTestId("acct-edit-fresh-snooze")).toHaveCount(0);
    await app.keyboard.press("Escape");
  });

  test("reconciliation records history and a reconciled-through status", async ({ app }) => {
    await nav(app, "/accounts");
    const kebab = app.locator('.add-wrap:has([data-testid="reconcile-start-btn-acct-checking"]) > button');
    await kebab.scrollIntoViewIfNeeded();
    await kebab.click();
    await app.locator('[data-testid="reconcile-start-btn-acct-checking"]').click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("reconcile-statement-mode")).toBeVisible();
    // First visit: never reconciled — no through-status yet.
    await expect(dialog.getByTestId("reconcile-through")).toHaveCount(0);
    // Type the exact cleared balance (read from the modal) so the diff is zero.
    const clearedText = await dialog.locator(".modal-scroll > div > span").first().innerText();
    const amount = clearedText.match(/-?[\d,]+\.\d{2}/)[0].replace(/,/g, "");
    await dialog.getByTestId("reconcile-statement-input").fill(amount);
    await dialog.getByTestId("reconcile-statement-date").fill("2026-06-30");
    await expect(dialog.getByTestId("reconcile-confirmed")).toBeVisible();
    // Record it — the modal closes and the event lands on the account.
    await dialog.getByTestId("reconcile-done").click();
    await expect(dialog.getByTestId("reconcile-statement-mode")).toHaveCount(0);
    // Reopen: reconciled-through + a history row with the statement balance.
    await kebab.scrollIntoViewIfNeeded();
    await kebab.click();
    await app.locator('[data-testid="reconcile-start-btn-acct-checking"]').click();
    await app.waitForTimeout(650);
    await expect(dialog.getByTestId("reconcile-through")).toContainText("Jun 30, 2026");
    const row = dialog.getByTestId("reconcile-history-row").first();
    await expect(row).toContainText("Jun 30, 2026");
    await app.keyboard.press("Escape");
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

    // The note renders as a readable line inside the row's details fold (the AC-series
    // disclosure) — expand it first, then the line itself expands on click.
    await app.locator(`[data-testid="acct-details-toggle-${acctId}"]`).click();
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
