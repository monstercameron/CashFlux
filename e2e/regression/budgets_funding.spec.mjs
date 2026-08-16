// budgets_funding.spec.mjs — C606: the funding lists for Cover and Top up must
// offer the amount that can ACTUALLY be moved, show what a source is left with,
// and refuse a move the chosen sources cannot fund.
//
// The defect was a figure: a source's whole LIMIT printed under the word
// "available" — the size of its plan, most of which had already been spent — so
// the number on screen and the number the app would move were different, and
// only the smaller one was true.
import { test, expect, nav, openVia } from "./fixtures.mjs";

// cardLeft reads a budget row's own "$X left" figure — the number the funding
// list has to agree with.
async function cardLeft(app, budgetID) {
  const txt = await app.getByTestId(`budget-card-${budgetID}`).innerText();
  const m = txt.match(/\$([\d,]+\.\d{2}) left/);
  return m ? m[1] : null;
}

// openTopup opens a budget's Top up form (in the compact density it lives in the
// row's ⋯ menu) and expands the funding list.
async function openTopup(app, budgetID) {
  const kebab = app.getByTestId(`budget-kebab-${budgetID}`);
  await kebab.scrollIntoViewIfNeeded();
  await openVia(app, kebab, app.locator(`.add-menu:not(.hidden-menu) [data-testid="budget-topup-btn-${budgetID}"]`));
  await app.locator(`.add-menu:not(.hidden-menu) [data-testid="budget-topup-btn-${budgetID}"]`).click();
  await expect(app.getByTestId("topup-cover-toggle")).toBeVisible();
  await openVia(app, app.getByTestId("topup-cover-toggle"), app.locator(".budget-topup-cover"));
}

test.describe("budgets: funding a top-up", () => {
  test("a source offers what it can move, not its limit", async ({ app }) => {
    await nav(app, "/budgets");
    // Splurges & getaways is the sample household's healthiest budget: a large
    // limit with most of it unspent, which is exactly where a limit-vs-left
    // mix-up is most visible.
    const left = await cardLeft(app, "bud-splurges");
    expect(left, "the sample budget should have a left figure to compare against").toBeTruthy();

    await openTopup(app, "bud-transport");
    const avail = app.getByTestId("topup-src-avail-bud-splurges");
    await expect(avail).toBeVisible();
    // The figure matches the card exactly — that agreement IS the ticket.
    await expect(avail).toContainText(left);
    // And it is not described as "available", which is what made the limit
    // read as spendable money.
    await expect(avail).toContainText(/can move/i);
  });

  test("picking a source shows what it would be left with", async ({ app }) => {
    await nav(app, "/budgets");
    await openTopup(app, "bud-transport");

    await app.locator("#budget-topup-amt").fill("50");
    await app.getByTestId("topup-src-bud-splurges").click();
    const after = app.getByTestId("topup-src-after-bud-splurges");
    await expect(after).toBeVisible();
    await expect(after).toContainText(/left after this/i);
  });

  test("a top-up the sources cannot fund is refused, not silently shrunk", async ({ app }) => {
    await nav(app, "/budgets");
    const before = await app.getByTestId("budget-card-bud-splurges").innerText();

    await openTopup(app, "bud-transport");
    await app.getByTestId("topup-src-bud-splurges").click();
    await app.locator("#budget-topup-amt").fill("99999");
    await app.locator('[role="dialog"] button[type="submit"]').first().click();

    // The form stays open and says by how much the selection falls short.
    await expect(app.locator('[role="dialog"]')).toContainText(/short of that/i);
    await expect(app.getByTestId("topup-cover-toggle")).toBeVisible();

    // Nothing was written: the source budget is untouched.
    await app.keyboard.press("Escape");
    await expect(app.getByTestId("budget-card-bud-splurges")).toHaveText(before.replace(/\s+/g, " ").trim(), {
      useInnerText: true,
    });
  });
});
