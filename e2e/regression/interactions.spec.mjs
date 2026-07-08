// interactions.spec.mjs — per-page interaction regressions. Each test drives a
// real user action and asserts the RESULT (not just "the page loaded"), pinning a
// specific fixed bug so it can't silently regress. Ported from the v1 wave-1/2
// bespoke scripts onto the Playwright runner with web-first waits.
import { test, expect, nav, mainText, setTheme } from "./fixtures.mjs";

test.describe("wave-1 fixes", () => {
  test("todo: adding a task refreshes the list and toasts", async ({ app }) => {
    await nav(app, "/todo");
    // Open the add-task form via the page's own visible "Add task" affordance
    // (data-testid=todo-add) — not the global "+" menu's hidden "New task" item.
    await app.getByTestId("todo-add").first().click();
    const title = app.locator("#task-add");
    await expect(title).toBeVisible();
    await title.fill("AAA regression check task");
    // Wait until the value is actually committed before submitting — under CPU
    // contention the fill can lag the click.
    await expect(title).toHaveValue("AAA regression check task");
    await app.getByTestId("task-add-submit").click();
    // The new task appears (data-revision bump) and a toast confirms — both async
    // re-renders, so give them headroom beyond the default expect timeout.
    await expect(app.locator("#main")).toContainText("AAA regression check task", { timeout: 20_000 });
    await expect(app.locator("body")).toContainText(/task added/i, { timeout: 20_000 });
  });

  test("bills: a liability + its recurring flow are not double-counted", async ({ app }) => {
    await nav(app, "/bills");
    const text = await mainText(app);
    const marcusCarRows = (text.match(/car payment \(marcus\)/gi) || []).length;
    expect(marcusCarRows, "Marcus car payment should appear at most once").toBeLessThanOrEqual(1);
  });

  test("subscriptions: share is sane and planned recurring isn't flagged cancellable", async ({ app }) => {
    await nav(app, "/subscriptions");
    const text = await mainText(app);
    const share = text.match(/share of spending\s*\n?\s*(\d+)%/i);
    if (share) expect(Number(share[1]), "share of spending ≤ 100%").toBeLessThanOrEqual(100);
    await expect(app.locator("body")).not.toContainText(/how to cancel hoa/i);
  });

  test("investments: holdings carry real security-type badges", async ({ app }) => {
    await nav(app, "/investments");
    await expect(app.locator("#main")).toContainText(/mutual fund|etf|stock/i);
  });

  test("budgets: no '1 budgets are over' plural bug", async ({ app }) => {
    await nav(app, "/budgets");
    await expect(app.locator("#main")).not.toContainText(/\b1 budgets are over\b/i);
  });

  test("light theme: meter track is not the fixed dark hex", async ({ app }) => {
    await setTheme(app, "light");
    await nav(app, "/allocate");
    const bar = app.locator("#main .cf-bar, #main [role='meter']").first();
    if (await bar.count()) {
      const bg = await bar.evaluate((el) => getComputedStyle(el).backgroundColor);
      expect(bg, "meter track must not be the hardcoded dark #232325 in light theme").not.toBe("rgb(35, 35, 37)");
    }
  });
});

test.describe("wave-2 fixes", () => {
  test("custom-page list rows show a distinguishing date sub-line", async ({ app }) => {
    await nav(app, "/p/side-hustle");
    // No leading \b: innerText concatenates the row label with the sub-line
    // ("revenueApr 23, 2026"), so a word boundary before the month never matches.
    await expect(app.locator("#main")).toContainText(
      /(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{1,2},\s+20\d\d/i,
    );
  });

  test("costs chart plots positive magnitudes (no negative $ axis)", async ({ app }) => {
    await nav(app, "/p/priya-business");
    const hasNeg = await app.evaluate(() => {
      const tiles = [...document.querySelectorAll("*")].filter(
        (e) => e.textContent && e.textContent.trim() === "Shop costs (live, 12 months)",
      );
      if (!tiles.length) return "no-tile";
      let card = tiles[0];
      for (let i = 0; i < 6 && card && !card.querySelector("svg"); i++) card = card.parentElement;
      if (!card) return "no-card";
      return /[-−]\s?\$/.test(card.innerText || "") ? "HAS_NEG" : "positive";
    });
    expect(hasNeg, "costs chart should read positive").toBe("positive");
  });

  test("Settings Cloud: backend defaults off and live actions are hidden", async ({ app }) => {
    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
    const main = app.locator("#main");
    await expect(main).toContainText(/backend off|fully local/i);
    await expect(app.locator("[role=switch]").first()).toHaveAttribute("aria-checked", "false");
    await expect(main).not.toContainText(/test connection/i);
    await expect(main).not.toContainText(/sync now/i);
    await expect(main).not.toContainText(/upload key/i);
  });

  test("Settings Advanced: single-language picker is hidden with a hint", async ({ app }) => {
    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "Advanced" }).first().click();
    await expect(app.locator("#main")).toContainText(/only language installed/i);
    await expect(
      app.locator(".settings-page select[title='Display language'], .settings-page select[aria-label='Display language']"),
    ).toHaveCount(0);
  });
});

test.describe("payment linkage", () => {
  // Open a mid-list row's ⋯ menu and click one of its "Mark as…" items. A mid-list row
  // (nth 6) avoids row 1, which sits under the sticky topbar/toolbar where the click
  // auto-scroll parks the menu item under the sticky chrome.
  async function openLinkModal(app, itemTestId) {
    const row = app.locator('[data-testid^="txn-row-"]').nth(6);
    await row.scrollIntoViewIfNeeded();
    await row.locator('[data-testid^="txn-kebab-"]').click();
    const item = row.locator(`[data-testid="${itemTestId}"]`);
    await expect(item).toBeVisible();
    await item.click();
    await expect(app.getByTestId("txnlink-summary")).toBeVisible();
    // The FlipPanel does a ~550ms 3D flip; wait past it so the picker inside isn't
    // mid-transform (transforming elements read as "not stable" to Playwright clicks).
    await app.waitForTimeout(600);
  }

  test("bill: mark via the flip modal → the account (any account) shows it and drills to it", async ({ app }) => {
    await nav(app, "/transactions");
    await openLinkModal(app, "txn-markbill-open");

    // The bill picker offers ANY account (not just liabilities). Pick the first one
    // (option 0 is the "not a bill payment" clear option) and read its id from the value.
    const select = app.getByTestId("txnlink-bill-select");
    await expect(select).toBeVisible();
    const [acctId] = await select.selectOption({ index: 1 });
    expect(acctId).toBeTruthy();
    await app.getByTestId("txnlink-save").click();

    // The Accounts page shows a bill-payment line on that account (works for any
    // account, not only debts), and its link drills to exactly the one we marked.
    await nav(app, "/accounts");
    await expect(app.locator(`[data-testid="acct-bill-${acctId}"]`)).toBeVisible();
    await app.locator(`[data-testid="acct-bill-link-${acctId}"]`).click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
    await expect(app.locator('[data-testid^="txn-row-"]')).toHaveCount(1);
  });

  test("bill: a liability still shows the payment on the Debt page", async ({ app }) => {
    // Read a real liability's name off the Debt page so the account we link is a debt.
    await nav(app, "/debt");
    const debtName = (await app.locator(".debt-name").first().innerText()).trim();
    expect(debtName.length).toBeGreaterThan(0);

    await nav(app, "/transactions");
    await openLinkModal(app, "txn-markbill-open");
    const select = app.getByTestId("txnlink-bill-select");
    const [acctId] = await select.selectOption({ label: debtName });
    expect(acctId).toBeTruthy();
    await app.getByTestId("txnlink-save").click();

    await nav(app, "/debt");
    await expect(app.locator(`[data-testid="debt-bill-${acctId}"]`)).toBeVisible();
    await app.locator(`[data-testid="debt-bill-link-${acctId}"]`).click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
    await expect(app.locator('[data-testid^="txn-row-"]')).toHaveCount(1);
  });

  test("subscription: mark via the flip modal → subscriptions row shows it and drills to it", async ({ app }) => {
    // Read a real subscription name off the panel first, so the one we link is
    // guaranteed to be both offered by the picker and displayed on the page.
    await nav(app, "/subscriptions");
    const subName = (await app.locator(".sub-row .sub-drill").first().innerText()).trim();
    expect(subName.length).toBeGreaterThan(0);

    await nav(app, "/transactions");
    await openLinkModal(app, "txn-marksub-open");

    // The modal opens on the Subscription picker; choose the subscription by name.
    const select = app.getByTestId("txnlink-sub-select");
    await expect(select).toBeVisible();
    await select.selectOption({ label: subName });
    await app.getByTestId("txnlink-save").click();

    // The subscriptions page now shows exactly one "last paid" line (the one we linked),
    // and its link drills to exactly that transaction.
    await nav(app, "/subscriptions");
    await expect(app.locator('[data-testid^="sub-pay-"]:not([data-testid^="sub-pay-link-"])')).toHaveCount(1);
    await app.locator('[data-testid^="sub-pay-link-"]').first().click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
    await expect(app.locator('[data-testid^="txn-row-"]')).toHaveCount(1);
  });
});

test.describe("account class override", () => {
  test("an Other-type account can be counted as a liability", async ({ app }) => {
    await nav(app, "/accounts");
    // Open the add-account form via the top-bar "+" menu.
    await app.locator('.add-wrap > button[aria-haspopup="menu"]').first().click();
    await app.getByRole("menuitem", { name: /new account/i }).click();
    const form = app.locator('[data-testid="account-add-form"]');
    await expect(form).toBeVisible();

    await form.locator('input[type="text"]').first().fill("Test HOA Dues");
    await form.locator("select").first().selectOption({ label: "Other" });
    const liab = app.getByTestId("acct-add-as-liability");
    await expect(liab).toBeVisible(); // the toggle appears only for the Other type
    await liab.click();
    await expect(liab).toBeChecked();
    await form.locator('button[type="submit"]').click();

    // It now appears under the Liabilities filter, not Assets — the class formulas
    // read the stored class, so the override takes effect.
    await nav(app, "/accounts");
    await app.getByTestId("acct-class-liabilities").click();
    await expect(app.locator("#main")).toContainText("Test HOA Dues");
    await app.getByTestId("acct-class-assets").click();
    await expect(app.locator("#main")).not.toContainText("Test HOA Dues");
  });
});
