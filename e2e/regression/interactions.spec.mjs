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
    // Custom-page tiles below the fold hydrate ~300ms after first paint (useAfterSettle),
    // so wait for the costs tile to mount before inspecting its axis.
    await app
      .waitForFunction(
        () => [...document.querySelectorAll("*")].some(
          (e) => e.textContent && e.textContent.trim() === "Shop costs (live, 12 months)",
        ),
        null,
        { timeout: 8000 },
      )
      .catch(() => {});
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
    // The add modal was standardized onto the FlipPanel FormID footer (internal/app/
    // addhost.go), so the submit button lives in the pinned panel footer and submits
    // the body form via form="account-add-form" — it is no longer inside the form.
    await app.locator('button[form="account-add-form"]').click();

    // It now appears under the Liabilities filter, not Assets — the class formulas
    // read the stored class, so the override takes effect.
    await nav(app, "/accounts");
    await app.getByTestId("acct-class-liabilities").click();
    await expect(app.locator("#main")).toContainText("Test HOA Dues");
    await app.getByTestId("acct-class-assets").click();
    await expect(app.locator("#main")).not.toContainText("Test HOA Dues");
  });
});

test.describe("account filter includes linked payments", () => {
  test("filtering by an account surfaces bill payments linked to it (booked elsewhere)", async ({ app }) => {
    await nav(app, "/debt");
    const debtName = (await app.locator(".debt-name").first().innerText()).trim();

    await nav(app, "/transactions");
    const row = app.locator('[data-testid^="txn-row-"]').nth(6);
    await row.scrollIntoViewIfNeeded();
    const rowId = await row.getAttribute("data-testid");
    await row.locator('[data-testid^="txn-kebab-"]').click();
    await row.locator('[data-testid="txn-markbill-open"]').click();
    await expect(app.getByTestId("txnlink-summary")).toBeVisible();
    await app.waitForTimeout(600); // past the FlipPanel flip
    const [acctId] = await app.getByTestId("txnlink-bill-select").selectOption({ label: debtName });
    await app.getByTestId("txnlink-save").click();

    // The debt card's "Transactions" drill filters by Account:<acctId>. The linked
    // payment shows even though it's booked on a different account.
    await nav(app, "/debt");
    await app.locator(`[data-testid="debt-view-${acctId}"]`).click();
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
    await expect(app.locator(`[data-testid="${rowId}"]`)).toBeVisible();
  });
});

test.describe("auto budget", () => {
  test("suggests budgets from history, tunes with sliders, switches method, and saves", async ({ app }) => {
    await nav(app, "/budgets");
    await app.getByTestId("budgets-autobudget").click();
    await expect(app.getByTestId("autobudget-rows")).toBeVisible();
    await app.waitForTimeout(650); // FlipPanel flip

    const rows = app.locator('[data-testid^="autobudget-row-"]');
    expect(await rows.count()).toBeGreaterThan(0);

    // Tune the first category to 50% and confirm its target amount changes.
    const firstAmt = app.locator('[data-testid^="autobudget-amt-"]').first();
    const before = (await firstAmt.innerText()).trim();
    await app.locator('[data-testid^="autobudget-slider-"]').first().fill("50");
    await expect(firstAmt).not.toHaveText(before);

    // The Smart+ "Healthy average" method reviews a longer window (3→6 months).
    await expect(app.getByTestId("autobudget-intro")).toContainText(/3 months/);
    await app.getByTestId("autobudget-method-healthy").click();
    await expect(app.getByTestId("autobudget-intro")).toContainText(/6 months/);

    // Ensure at least one category is selected, then save.
    const firstPick = app.locator('[data-testid^="autobudget-pick-"]').first();
    if (!(await firstPick.isChecked())) await firstPick.click();
    await app.getByTestId("autobudget-save").click();
    await expect(app.locator("body")).toContainText(/saved/i, { timeout: 15000 });
    await expect(app.getByTestId("autobudget-rows")).toHaveCount(0);
  });
});

test.describe("bill auto-link rule", () => {
  test("linking a bill with auto-link creates a rule for future payments", async ({ app }) => {
    await nav(app, "/transactions");
    const row = app.locator('[data-testid^="txn-row-"]').nth(6);
    await row.scrollIntoViewIfNeeded();
    await row.locator('[data-testid^="txn-kebab-"]').click();
    await row.locator('[data-testid="txn-markbill-open"]').click();
    await expect(app.getByTestId("txnlink-summary")).toBeVisible();
    await app.waitForTimeout(650); // FlipPanel flip

    // The auto-link toggle appears once an account is chosen.
    await app.getByTestId("txnlink-bill-select").selectOption({ index: 1 });
    const toggle = app.getByTestId("txnlink-autolink");
    await expect(toggle).toBeVisible();
    await toggle.click();
    await expect(toggle).toBeChecked();
    await app.getByTestId("txnlink-save").click();

    // The toast confirms a rule was created so future payments auto-link.
    await expect(app.locator("body")).toContainText(/link automatically/i, { timeout: 15000 });
  });
});

test.describe("multi-category budgets", () => {
  test("edit a budget's tracked categories via the kebab modal", async ({ app }) => {
    await nav(app, "/budgets");
    const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
    await kebab.scrollIntoViewIfNeeded();
    const bid = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
    await kebab.click();
    await app.locator(`[data-testid="edit-budget-cats-btn-${bid}"]`).click();
    await expect(app.getByTestId("budgetcats-rows")).toBeVisible();
    await app.waitForTimeout(650); // FlipPanel flip

    // Check the first two categories → a multi-category budget.
    const picks = app.locator('[data-testid^="budgetcat-pick-"]');
    const n = await picks.count();
    let checked = 0;
    for (let i = 0; i < n && checked < 2; i++) {
      const p = picks.nth(i);
      if (!(await p.isChecked())) await p.click();
      checked++;
    }
    await app.getByTestId("budgetcats-save").click();
    await expect(app.locator("body")).toContainText(/tracked categories updated/i, { timeout: 15000 });
    await expect(app.locator(`[data-testid="budget-tracked-cats-${bid}"]`)).toBeVisible();
  });
});

test.describe("budget category picker", () => {
  test("search filters the list; add form embeds the picker", async ({ app }) => {
    await nav(app, "/budgets");
    // Kebab modal: search narrows the checklist.
    const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
    await kebab.scrollIntoViewIfNeeded();
    const bid = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
    await kebab.click();
    await app.locator(`[data-testid="edit-budget-cats-btn-${bid}"]`).click();
    await expect(app.getByTestId("budgetcats-rows")).toBeVisible();
    await app.waitForTimeout(650);
    const before = await app.locator('[data-testid^="budgetcat-pick-"]').count();
    await app.getByTestId("budgetcats-search").fill("din");
    await app.waitForTimeout(150);
    expect(await app.locator('[data-testid^="budgetcat-pick-"]').count()).toBeLessThan(before);
    await app.getByTestId("budgetcats-cancel").click();

    // The add-budget form embeds the same picker.
    await app.getByTestId("budgets-add").click();
    await expect(app.getByTestId("budget-add-form")).toBeVisible();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("budgetcats-search")).toBeVisible();
  });
});

test.describe("budgets last-month toggle", () => {
  test("one click flips the budgets view to last month and back", async ({ app }) => {
    await nav(app, "/budgets");
    const toggle = app.getByTestId("budgets-last-month");
    await expect(toggle).toBeVisible();
    await expect(toggle).toHaveAttribute("aria-pressed", "false");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
    // Pressed-state label is "Showing last month's spend" (budgets.lastMonthOn); the
    // off-state label is "Last month's spend", so "showing" pins the active state.
    await expect(toggle).toContainText(/showing last month/i);
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "false");
  });
});

test.describe("statement import", () => {
  test("the Import statement modal opens with the upload UI", async ({ app }) => {
    await nav(app, "/transactions");
    const btn = app.getByTestId("txn-statement-import-btn");
    await expect(btn).toBeVisible();
    await btn.click();
    await expect(app.getByTestId("statementimport-choose")).toBeVisible();
    await expect(app.getByTestId("statementimport-run")).toBeVisible();
    await app.getByTestId("statementimport-cancel").click();
    await expect(app.getByTestId("statementimport-choose")).toHaveCount(0);
  });
});
