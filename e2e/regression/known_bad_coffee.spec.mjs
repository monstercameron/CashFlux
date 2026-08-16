// known_bad_coffee.spec.mjs — C601.
//
// The sample ledger seeds one deliberately-suspect charge: `tx-coffee-anomaly-
// 2026-06`, a $68 "Coffee — $68?!" at Blue Bottle, filed under Dining and tagged
// needs-review. It is the control case the whole review surface demonstrates
// itself on, and the question this file answers is not "does the edit form work"
// — other specs cover that — but whether correcting THIS charge through the real
// workflow actually STICKS: across the re-render, across a reload, and across a
// search that finds it again.
//
// It found the reason it had not. The review surface strips the needs-review tag
// when a category is confirmed (assignReviewCategory → removeReviewTag); the edit
// form never did. So a user who corrected the charge from the ledger recorded the
// fix and left the charge flagged — still queued, still counted in "Review inbox
// (N)", permanently. Two paths for one decision disagreed about what the decision
// meant, and only one of them was tested.
//
// The seed is NOT modified. This charge is the fixture every review demo starts
// from; "fixing" it in sample.go would delete the case rather than prove the
// workflow. The test corrects it at runtime and asserts the correction holds.
import { test, expect } from "@playwright/test";
import { boot, nav, settle } from "./fixtures.mjs";

const ROW = "tx-coffee-anomaly-2026-06";
const NEW_CATEGORY = "Groceries";

// Reads the seeded control charge straight out of the dataset, so the assertions
// are about STORED state rather than about what the table happened to paint.
async function storedRow(page, id) {
  return page.evaluate(async (rowID) => {
    const db = await new Promise((res, rej) => {
      const r = indexedDB.open("cashflux-kv");
      r.onsuccess = () => res(r.result);
      r.onerror = () => rej(r.error);
    });
    const store = db.transaction(db.objectStoreNames[0]).objectStore(db.objectStoreNames[0]);
    const all = await new Promise((res, rej) => {
      const r = store.getAll();
      r.onsuccess = () => res(r.result);
      r.onerror = () => rej(r.error);
    });
    for (const v of all) {
      const s = typeof v === "string" ? v : JSON.stringify(v);
      if (!s.includes(rowID)) continue;
      const data = typeof v === "string" ? JSON.parse(v) : v;
      const txns = data.transactions || data.Transactions || (data.value && data.value.transactions);
      if (!Array.isArray(txns)) continue;
      const hit = txns.find((t) => (t.id || t.ID) === rowID);
      if (hit) return hit;
    }
    return null;
  }, id);
}

test.describe("C601 — the known-bad coffee control", () => {
  test("correcting it through the edit path sticks, and clears the review flag", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    // Find it the way a person would: search, don't deep-link.
    await page.locator(".fctrl-input").fill("Coffee — $68");
    const row = page.locator(`[data-testid="txn-row-${ROW}"]`);
    await expect(row).toBeVisible({ timeout: 25_000 });

    const before = await storedRow(page, ROW);
    expect(before, "the seeded control charge must exist in the dataset").not.toBeNull();
    expect(before.categoryId || before.CategoryID).toBeTruthy(); // seeded under Dining
    const beforeTags = before.tags || before.Tags || [];
    expect(beforeTags, "seeded flagged for review").toContain("needs-review");

    // Correct it through the intended path: open the row, change the category, save.
    await row.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
    // Pick by the category's own name. The picker labels options with their full
    // path ("Housing > Mortgage"), so match the leaf rather than the whole string.
    const catSelect = page.locator(".txn-cat-row .field").first();
    const targetValue = await catSelect.evaluate((el, name) => {
      const opt = [...el.options].find((o) => o.text.trim().split(">").pop().trim() === name);
      return opt ? opt.value : "";
    }, NEW_CATEGORY);
    expect(targetValue, `a "${NEW_CATEGORY}" category exists to correct it into`).not.toBe("");
    await catSelect.selectOption(targetValue);
    await page.locator("[data-testid=txn-edit-save]").click();

    // Saving a category change can raise the apply-to-similar offer, which keeps the
    // dialog open on purpose. This correction is about ONE charge, so decline it.
    await page.waitForFunction(
      () => !document.querySelector(".flip-backdrop") || !!document.querySelector("[data-testid=txn-recat-offer]"),
      // Generous on purpose: the save re-filters and re-renders a 3,000-row ledger,
      // and with two Playwright workers driving this wasm app on one machine that
      // has taken over 25s. A tight bound here fails as "the offer never appeared"
      // when the truth is "the machine was busy".
      null, { timeout: 60_000 });
    if (await page.locator("[data-testid=txn-recat-offer]").count()) {
      await page.locator("[data-testid=txn-recat-dismiss]").click();
    }
    await expect(page.locator(".flip-backdrop")).toHaveCount(0, { timeout: 15_000 });
    await settle(page);

    // --- the correction landed, and nothing else moved -----------------------
    const after = await storedRow(page, ROW);
    expect(after, "the charge survived the edit").not.toBeNull();
    expect(after.categoryId || after.CategoryID,
      "the category changed").not.toBe(before.categoryId || before.CategoryID);

    for (const [k, label] of [["amount", "amount"], ["date", "date"], ["payee", "payee"], ["desc", "description"], ["accountId", "account"], ["memberId", "member"]]) {
      const b = before[k] ?? before[k[0].toUpperCase() + k.slice(1)];
      const a = after[k] ?? after[k[0].toUpperCase() + k.slice(1)];
      expect(JSON.stringify(a), `${label} must not change`).toBe(JSON.stringify(b));
    }

    // --- the review state reflects the completed decision --------------------
    const afterTags = after.tags || after.Tags || [];
    expect(afterTags, "the needs-review flag is cleared by the decision").not.toContain("needs-review");
    expect(after.reviewed ?? after.Reviewed, "the decision is recorded as a person's").toBe(true);
    await expect(page.locator(`[data-testid="txn-row-${ROW}"] .td-status`)).not.toHaveText(/Needs review/);
    await expect(page.locator(`[data-testid="txn-row-${ROW}"] [data-testid=txn-auto-mark]`)).toHaveCount(0);

    // --- and it survives a reload, found again by search ---------------------
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });
    await page.locator(".fctrl-input").fill("Coffee — $68");
    await expect(page.locator(`[data-testid="txn-row-${ROW}"]`)).toBeVisible({ timeout: 25_000 });

    const reloaded = await storedRow(page, ROW);
    expect(reloaded.categoryId || reloaded.CategoryID,
      "the correction persisted across a reload").toBe(after.categoryId || after.CategoryID);
    expect(reloaded.tags || reloaded.Tags || [],
      "and so did clearing the review flag").not.toContain("needs-review");
  });
});
