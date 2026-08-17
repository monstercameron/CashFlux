// txn_classify.spec.mjs — the movement classifier in the edit modal.
//
// The rules are unit-tested (internal/txnclassify) and the control is
// component-tested (internal/screens/txn_classify_wasm_test.go). Neither can see
// the thing this file exists for: that choosing an account in the modal, pressing
// Save, and coming back actually changes what the ledger REPORTS.
//
// The motivating case is a statement import. A credit-union export files every
// savings sweep and card payment as plain income or spending, because that is all
// a bank line says. Until this control existed such a row could not be corrected
// at all — a transfer could only be BORN a transfer, never become one — so the
// gap it closes is measured here in the only terms that matter: the net figure
// the transactions page prints above the ledger.
import { test, expect } from "@playwright/test";
import { boot, nav } from "./fixtures.mjs";

// openFirstPlainRow opens the edit modal for the first row that still reads as
// income or spending, and returns that row's locator.
async function openFirstPlainRow(page) {
  const rows = page.locator(".txn-table tbody tr.row");
  const count = await rows.count();
  for (let i = 0; i < count; i++) {
    const row = rows.nth(i);
    const edit = row.locator("[data-testid=txn-rowedit]");
    if ((await edit.count()) === 0) continue;

    await edit.click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
    // Only a row that is NOT already a transfer can prove the change; an
    // already-classified one would leave the net where it was.
    const picker = page.locator("[data-testid=txn-edit-classify]");
    await expect(picker).toBeVisible({ timeout: 15_000 });
    if ((await picker.inputValue()) === "") return row;
    await page.keyboard.press("Escape");
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeHidden({ timeout: 15_000 });
  }
  throw new Error("no unclassified row with an edit control was found");
}

test.describe("the movement classifier", () => {
  // The app treats a transfer leg's sign as structural — which side of the move
  // this row is — so the edit form withholds the direction control from one. That
  // makes the control's ABSENCE the app's own statement that a row is now a
  // transfer, reached through the real save and a real reload of the row.
  //
  // (The ledger's "net" figure is deliberately NOT asserted here: it sums every
  // shown row, transfers included, so it would not move — see the classify notes
  // in DEVLOG. The reporting effect is pinned instead against ledger.PeriodTotals
  // over a real imported statement in internal/txnclassify/scenario_test.go.)
  test("a classified row is treated as a transfer, not as income or spending", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    const row = await openFirstPlainRow(page);

    // The modal opens saying the row is NOT a transfer — the way out is a real
    // answer, not an empty slot, so it has to be the one selected. And because it
    // is plain income or spending, its direction IS editable.
    const picker = page.locator("[data-testid=txn-edit-classify]");
    await expect(picker).toHaveValue("");
    await expect(page.locator("[data-testid=txn-edit-classify-effect]")).toHaveCount(0);
    await expect(page.locator("[data-testid=txn-edit-direction]")).toHaveCount(1);

    const target = await picker.locator("option").nth(1).getAttribute("value");
    expect(target).toBeTruthy();
    await picker.selectOption(target);

    // Before saving, the modal states what the save will do.
    await expect(page.locator("[data-testid=txn-edit-classify-effect]")).toBeVisible();

    await page.locator("[data-testid=txn-edit-form]").evaluate((f) => f.requestSubmit());
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeHidden({ timeout: 20_000 });

    // Reopened, the app itself now says the row is a transfer: the direction
    // control is gone, because a leg's sign is no longer the user's to flip.
    await row.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
    await expect(page.locator("[data-testid=txn-edit-classify]")).toHaveValue(target);
    await expect(page.locator("[data-testid=txn-edit-direction]")).toHaveCount(0);
  });

  test("the classification survives a save and reopen", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    const row = await openFirstPlainRow(page);
    const picker = page.locator("[data-testid=txn-edit-classify]");
    const target = await picker.locator("option").nth(1).getAttribute("value");
    await picker.selectOption(target);
    await page.locator("[data-testid=txn-edit-form]").evaluate((f) => f.requestSubmit());
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeHidden({ timeout: 20_000 });

    // Reopen the same row: the modal must show what the row now IS, or the user
    // cannot tell a saved classification from a lost one.
    await row.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
    await expect(page.locator("[data-testid=txn-edit-classify]")).toHaveValue(target);
    await expect(page.locator("[data-testid=txn-edit-classify-effect]")).toBeVisible();
  });

  test("the debt claim appears only for a card or loan", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    await openFirstPlainRow(page);
    const picker = page.locator("[data-testid=txn-edit-classify]");
    const claim = page.locator("[data-testid=txn-edit-classify-debt]");

    // Nothing chosen: no claim, no effect.
    await expect(claim).toHaveCount(0);

    // Walk every offered account. A card or loan offers the claim; an asset
    // never does — paying down a savings account is not a thing.
    const options = picker.locator("option");
    const n = await options.count();
    let sawDebt = false;
    let sawAsset = false;
    for (let i = 1; i < n; i++) {
      const value = await options.nth(i).getAttribute("value");
      await picker.selectOption(value);
      await expect(page.locator("[data-testid=txn-edit-classify-effect]")).toBeVisible();
      const effect = await page.locator("[data-testid=txn-edit-classify-effect]").innerText();
      if (/pays down what you owe/i.test(effect)) {
        sawDebt = true;
        await expect(claim).toHaveCount(1);
      } else {
        sawAsset = true;
        await expect(claim).toHaveCount(0);
      }
    }
    // The sample household has both, so a run that saw only one kind means the
    // walk did not actually exercise the branch.
    expect(sawDebt, "no liability account was offered — the branch went untested").toBe(true);
    expect(sawAsset, "no asset account was offered — the branch went untested").toBe(true);
  });

  test("a card payment can be claimed as the payment and reaches the debt page", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    await openFirstPlainRow(page);
    const picker = page.locator("[data-testid=txn-edit-classify]");
    const claim = page.locator("[data-testid=txn-edit-classify-debt]");

    // Find the first liability among the offered accounts.
    const options = picker.locator("option");
    const n = await options.count();
    let debtValue = null;
    for (let i = 1; i < n; i++) {
      const value = await options.nth(i).getAttribute("value");
      await picker.selectOption(value);
      if ((await claim.count()) === 1) {
        debtValue = value;
        break;
      }
    }
    expect(debtValue, "the sample household has no debt account to pay").toBeTruthy();

    await claim.check();
    await expect(claim).toBeChecked();
    await page.locator("[data-testid=txn-edit-form]").evaluate((f) => f.requestSubmit());
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeHidden({ timeout: 20_000 });

    // Reopening shows both halves held: the counterparty AND the claim.
    const row = page.locator(".txn-table tbody tr.row").first();
    await row.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
  });
});
