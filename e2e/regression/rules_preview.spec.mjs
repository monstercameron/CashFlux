// rules_preview.spec.mjs — rule preview: the affected-transactions disclosure
// behind each rule's count, and the retroactive-vs-future-only choice at
// creation (apply-to-existing checkbox → precedence-honouring backfill).
import { test, expect, nav } from "./fixtures.mjs";

test.describe("rules: affected-transactions preview + retroactive choice", () => {
  test("clicking a rule's count expands the matched transactions", async ({ app }) => {
    await nav(app, "/rules");
    const btn = app.locator('[data-testid^="rule-matches-btn-"]').first();
    await btn.scrollIntoViewIfNeeded();
    await expect(btn).toHaveAttribute("aria-expanded", "false");
    await btn.click();
    await expect(btn).toHaveAttribute("aria-expanded", "true");
    const list = app.locator('[data-testid^="rule-matches-list-"]').first();
    await expect(list).toBeVisible();
    // Rows carry a date and a money figure.
    await expect(list).toContainText(/[A-Z][a-z]{2} \d{1,2}, \d{4}/);
    await expect(list).toContainText(/[\d,]+\.\d{2}/);
    // Collapse again.
    await btn.click();
    await expect(app.locator('[data-testid^="rule-matches-list-"]')).toHaveCount(0);
  });

  test("the add form's apply-to-existing choice backfills the new rule", async ({ app }) => {
    await nav(app, "/rules");
    const form = app.getByTestId("rule-add-form").last();
    await form.scrollIntoViewIfNeeded();
    // "trattoria" matches sample charges NO existing rule covers — the
    // precedence-honouring backfill only claims first-match transactions, so a
    // phrase an earlier rule owns would (correctly) apply to zero.
    await form.getByLabel(/match text/i).fill("trattoria");
    await form.getByLabel(/category to assign/i).selectOption({ index: 1 });
    await form.getByTestId("rule-add-apply-existing").check({ force: true });
    await form.getByTestId("rule-add-submit").click();
    // The new rule exists…
    await expect(app.locator("#main")).toContainText(/Contains "trattoria"/);
    // …and the backfill actually WROTE: the Trattoria charges now carry the
    // chosen category ("Auto loans" is option 1) in the ledger.
    await nav(app, "/transactions");
    await app.locator('input[type="search"]').first().fill("Trattoria");
    // The row displays the cleaned description ("Dinner out"), not the payee —
    // the search matches on payee, and the category cell proves the write.
    const firstRow = app.locator('[data-testid^="txn-row-"]').first();
    await expect(firstRow).toContainText(/Auto loans/);
  });
});
