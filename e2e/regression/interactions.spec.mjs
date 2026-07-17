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
  test("edit a budget's tracked categories via the card's ⋯ menu", async ({ app }) => {
    await nav(app, "/budgets");
    // "Edit tracking" lives in the card's ⋯ overflow (footer diet) — open it first.
    const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
    await kebab.scrollIntoViewIfNeeded();
    const bid = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
    await kebab.click();
    await app.locator(`.add-menu [data-testid="edit-budget-cats-btn-${bid}"]`).click();
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
    // Tracked-categories modal (opened from the card's ⋯ menu): search narrows the checklist.
    const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
    await kebab.scrollIntoViewIfNeeded();
    const kbid = (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
    await kebab.click();
    await app.locator(`.add-menu [data-testid="edit-budget-cats-btn-${kbid}"]`).click();
    await expect(app.getByTestId("budgetcats-rows")).toBeVisible();
    await app.waitForTimeout(650);
    const before = await app.locator('[data-testid^="budgetcat-pick-"]').count();
    await app.getByTestId("budgetcats-search").fill("din");
    await app.waitForTimeout(150);
    expect(await app.locator('[data-testid^="budgetcat-pick-"]').count()).toBeLessThan(before);
    await app.getByTestId("budgetcats-cancel").click();

    // The add-budget form embeds the same picker — behind the "More options"
    // disclosure (the essentials-first layout keeps the default form two fields).
    await app.getByTestId("budgets-add").first().click();
    await expect(app.getByTestId("budget-add-form")).toBeVisible();
    await app.waitForTimeout(650);
    await app.getByTestId("budget-add-advanced").click();
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

test.describe("budgets actions widget", () => {
  test("icon+label actions, a Sort picker, and no metrics/template/smart buttons", async ({ app }) => {
    await nav(app, "/budgets");
    // The budgets toolbar now uses the shared filter-toolbar vocabulary (the old
    // .budgets-toolbar-actions wrapper was retired with the 2-row toolbar).
    const actions = app.locator(".budgets-tb .filter-toolbar-actions");
    await expect(actions).toBeVisible();
    // Retired from the toolbar: the Smart sparkle shortcut, the Budget-metrics toggle,
    // and the 50/30/20 template (moved into the Add-budget modal).
    await expect(actions.locator('[data-testid="smart-section-action"]')).toHaveCount(0);
    await expect(app.locator('[data-testid="budgets-toggle-formulas"]')).toHaveCount(0);
    await expect(actions.locator('[data-testid="budgets-template-503020"]')).toHaveCount(0);
    // The remaining actions each carry a glyph beside their text.
    for (const id of ["budgets-last-month", "budgets-autobudget", "budgets-add"]) {
      await expect(actions.locator(`[data-testid="${id}"] svg`)).toBeVisible();
    }
    // The Sort picker is present with the health/overage/underused options.
    const sort = app.getByTestId("budgets-sort");
    await expect(sort).toBeVisible();
    await expect(sort.locator("option")).toHaveCount(6);
  });
});

test.describe("import wizard", () => {
  const MODAL = '[role="dialog"][aria-label="Import"]';

  // Open the single merged Import flip modal and wait past the ~550ms 3D flip.
  async function openImport(app) {
    await nav(app, "/transactions");
    const btn = app.getByTestId("txn-import-btn");
    await expect(btn).toBeVisible();
    await btn.click();
    await app.waitForTimeout(650); // past the FlipPanel flip
  }

  test("Stage 1 is a Smart / Smart+ document-type picker (one Import button)", async ({ app }) => {
    await openImport(app);
    await expect(app.getByTestId("import-type-picker")).toBeVisible();
    // All four sources across the two branches are offered.
    for (const id of ["import-type-csv", "import-type-stmt", "import-type-pdf", "import-type-receipt"]) {
      await expect(app.getByTestId(id)).toBeVisible();
    }
    // The two Smart+ (generative-AI) tiles carry the brand accent class.
    await expect(app.locator('[data-testid="import-type-pdf"].smartplus')).toHaveCount(1);
    await expect(app.locator('[data-testid="import-type-receipt"].smartplus')).toHaveCount(1);
    // The old standalone "Import statement" toolbar button is retired.
    await expect(app.getByTestId("txn-statement-import-btn")).toHaveCount(0);
  });

  test("picking the Statement PDF tile reveals its form; back returns to the grid", async ({ app }) => {
    await openImport(app);
    await app.getByTestId("import-type-pdf").click();
    await expect(app.getByTestId("import-source-form")).toBeVisible();
    await expect(app.getByTestId("statementimport-choose")).toBeVisible();
    await expect(app.getByTestId("statementimport-run")).toBeVisible();
    // Back to the type chooser hides the form and re-shows the grid.
    await app.getByTestId("import-back-types").click();
    await expect(app.getByTestId("import-type-picker")).toBeVisible();
    await expect(app.getByTestId("statementimport-choose")).toHaveCount(0);
  });

  test("statement-text Parse advances to review, and footer Save imports the rows", async ({ app }) => {
    await openImport(app);
    await app.getByTestId("import-type-stmt").click();
    const ta = app.locator(`${MODAL} textarea[placeholder^="Posting Date"]`);
    await expect(ta).toBeVisible();
    await ta.fill("Posting Date,Description,Debit,Credit\n2026-07-01,REGRESSION STMT IMPORT,,64.00\n");
    // Deterministic Parse (no AI) yields a draft and advances to Stage 2 review.
    await app.locator(MODAL).getByRole("button", { name: "Parse statement" }).click();
    await expect(app.getByTestId("flip-save")).toBeVisible({ timeout: 20_000 });
    await expect(app.locator(MODAL)).toContainText("REGRESSION STMT IMPORT");
    // "Add more data" returns to Stage 1 (keeping the draft) → the review shortcut appears.
    await app.getByTestId("import-back-btn").click();
    await expect(app.getByTestId("import-review-btn")).toBeVisible();
    await app.getByTestId("import-review-btn").click();
    // Footer Save commits the reviewed draft and closes the modal.
    await app.getByTestId("flip-save").click();
    await expect(app.locator(MODAL)).toHaveCount(0, { timeout: 20_000 });
    // The imported row is now in the ledger.
    await app.locator('input[placeholder="Search description, payee, or tag"]').fill("REGRESSION STMT IMPORT");
    await expect(app.locator("#main")).toContainText("REGRESSION STMT IMPORT", { timeout: 20_000 });
  });

  test("the CSV tile imports pasted rows directly (lossless path, no review)", async ({ app }) => {
    await openImport(app);
    await app.getByTestId("import-type-csv").click();
    const ta = app.locator(`${MODAL} textarea[placeholder^="date,payee"]`);
    await expect(ta).toBeVisible();
    await ta.fill("date,desc,amount\n2026-07-03,REGRESSION CSV IMPORT,-9.99\n");
    // Footer Save commits the ready CSV directly and closes.
    await app.getByTestId("flip-save").click();
    await expect(app.locator(MODAL)).toHaveCount(0, { timeout: 20_000 });
    await app.locator('input[placeholder="Search description, payee, or tag"]').fill("REGRESSION CSV IMPORT");
    await expect(app.locator("#main")).toContainText("REGRESSION CSV IMPORT", { timeout: 20_000 });
  });
});

test.describe("review duplicates", () => {
  const MODAL = '[role="dialog"][aria-label="Review duplicates"]';

  test("the duplicates button opens the review modal over the ledger", async ({ app }) => {
    await nav(app, "/transactions");
    const btn = app.getByTestId("txn-dupes-btn");
    await expect(btn).toBeVisible();
    await btn.click();
    await app.waitForTimeout(650); // past the FlipPanel flip
    const modal = app.locator(MODAL);
    await expect(modal).toBeVisible();
    // The review UI: a duplicate group with proper Merge + Delete buttons.
    await expect(modal.getByTestId("dup-merge-btn").first()).toBeVisible();
    await expect(modal.getByTestId("dup-delete-btn").first()).toBeVisible();
    // The ledger stays mounted behind the modal (not an in-place takeover).
    await expect(app.locator('[data-testid="txn-table"], table').first()).toBeVisible();
    // The old in-place duplicates tile is gone.
    await expect(app.locator('[data-testid="txn-duplicates"], #txn-duplicates')).toHaveCount(0);
    // The Close footer dismisses.
    await modal.locator(".set-btn.close").click();
    await expect(app.locator(MODAL)).toHaveCount(0);
  });

  test("merging a duplicate group resolves it, leaving the empty state", async ({ app }) => {
    await nav(app, "/transactions");
    await app.getByTestId("txn-dupes-btn").click();
    await app.waitForTimeout(650);
    const modal = app.locator(MODAL);
    await expect(modal.getByTestId("dup-merge-btn").first()).toBeVisible();
    await modal.getByTestId("dup-merge-btn").first().click();
    // Confirm the destructive merge in the shared confirm dialog.
    await app.locator("#cf-dialog-confirm").click();
    // The resolved group drops off; the still-open modal shows the empty state.
    await expect(modal).toContainText("No duplicate transactions found", { timeout: 20_000 });
  });
});

test.describe("pager scroll-to-top", () => {
  // Delta between the ledger anchor's top and the scroll container's top, in px.
  // ~0 means the table is pinned to the top of the viewport (i.e. we scrolled to it).
  const anchorDelta = (app) =>
    app.evaluate(() => {
      const anchor = document.getElementById("txn-ledger-anchor");
      const sc = document.querySelector("main.cf-scroll");
      if (!anchor || !sc) return 99999;
      return Math.round(anchor.getBoundingClientRect().top - sc.getBoundingClientRect().top);
    });

  test("clicking Next from the bottom jumps the ledger back to the top", async ({ app }) => {
    await nav(app, "/transactions");
    await expect(app.locator('#main[data-route="/transactions"]').first()).toBeVisible();
    const scroller = app.locator("main.cf-scroll");
    // Multi-page seed → the pager's Next is present and enabled on page 1.
    const nextBtn = app.locator('button[aria-label="Next page"]').last();
    await expect(nextBtn).toBeEnabled();

    // Strand the user at the very bottom of the page, so the ledger's top is scrolled
    // far above the viewport (a large negative delta).
    await scroller.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    expect(await scroller.evaluate((el) => el.scrollTop), "scrolled well down").toBeGreaterThan(200);
    expect(await anchorDelta(app), "ledger top scrolled above the viewport").toBeLessThan(-100);

    // Paging forward scrolls the ledger anchor back to the top of the scroll container.
    await nextBtn.click();
    await expect
      .poll(() => anchorDelta(app), { timeout: 5000 })
      .toBeLessThan(60);
    // And it settled at the top, not somewhere in the middle.
    expect(Math.abs(await anchorDelta(app)), "ledger pinned to the container top").toBeLessThan(60);
  });
});

test.describe("multi-value filters", () => {
  test("multiple account pills filter OR-within, with per-value chips + a count badge", async ({ app }) => {
    await nav(app, "/transactions");
    // Open the filter panel (the funnel trigger toggles it).
    await app.locator(".filters-trigger").first().click();
    await expect(app.locator(".filter-panel")).toBeVisible();
    // The account group is first — select its first two pills (multi-select).
    const pills = app.locator(".filter-pill");
    await pills.nth(0).click();
    await pills.nth(1).click();
    // Both read as selected; each is a removable per-value chip; the trigger badges 2.
    await expect(app.locator(".filter-pill.on")).toHaveCount(2);
    await expect(app.locator(".filter-chip")).toHaveCount(2);
    await expect(app.locator(".filters-trigger .filter-badge")).toHaveText("2");
    // Removing one chip drops just that value (not the whole dimension).
    await app.locator(".filter-chip .chip-x").first().click();
    await expect(app.locator(".filter-pill.on")).toHaveCount(1);
    await expect(app.locator(".filter-chip")).toHaveCount(1);
  });
});

test.describe("unified search control", () => {
  // The transactions + accounts toolbars share the FilterToolbar search, which was
  // switched to the same .fctrl "control pill" markup the to-do page uses (a leading
  // magnifier, a borderless input, and a clear × that appears only when it holds a
  // query) so every toolbar speaks one control language.
  for (const route of ["/transactions", "/accounts"]) {
    test(`${route} search uses the shared .fctrl pill (magnifier + input + clear)`, async ({ app }) => {
      await nav(app, route);
      const pill = app.locator(".filter-toolbar .fctrl.fctrl-search").first();
      await expect(pill).toBeVisible();
      // A leading magnifier icon and a borderless input inside the pill.
      await expect(pill.locator("svg")).toBeVisible();
      const input = pill.locator("input.fctrl-input");
      await expect(input).toBeVisible();
      // No clear affordance until there's a query.
      await expect(pill.locator(".fctrl-clear")).toHaveCount(0);
      // Typing lights the accent ring and reveals the clear (×).
      await input.fill("coffee");
      await expect(pill).toHaveClass(/is-active/);
      const clear = pill.locator(".fctrl-clear");
      await expect(clear).toBeVisible();
      // Clearing empties the field and hides the affordance again.
      await clear.click();
      await expect(input).toHaveValue("");
      await expect(pill.locator(".fctrl-clear")).toHaveCount(0);
    });
  }
});

test.describe("labeled toolbar buttons", () => {
  test("transactions toolbar actions show visible text labels (not icon-only)", async ({ app }) => {
    await nav(app, "/transactions");
    // The Add action is a labeled .btn-tool with its text visible inline.
    const add = app.getByTestId("txn-add-btn");
    await expect(add).toHaveClass(/btn-tool/);
    await expect(add).not.toHaveClass(/tbar-btn/);
    await expect(add).not.toBeEmpty(); // carries a visible text label, not just a glyph
    const label = (await add.innerText()).trim();
    expect(label.length, "the Add button shows a readable text label").toBeGreaterThan(1);
    // Exactly one icon glyph on the button (single-glyph rule).
    await expect(add.locator("svg")).toHaveCount(1);
    // The Filters trigger is also labeled now (not a bare funnel glyph).
    const filters = app.locator(".filters-trigger").first();
    await expect(filters).toHaveClass(/btn-tool/);
    expect((await filters.innerText()).trim().length, "Filters trigger shows its label").toBeGreaterThan(1);
  });

  test("the toolbar is one left-justified group with the green Add at the right end", async ({ app }) => {
    await nav(app, "/transactions");
    const info = await app.locator(".filter-toolbar").first().evaluate((t) => {
      const kids = [...t.children];
      const tops = new Set(kids.map((k) => Math.round(k.getBoundingClientRect().top)));
      const add = t.querySelector('[data-testid="txn-add-btn"]');
      return { rows: tops.size, addIsLast: kids[kids.length - 1] === add };
    });
    // Single row at the standard desktop width — the primary action doesn't wrap below.
    expect(info.rows, "toolbar is a single row").toBe(1);
    expect(info.addIsLast, "the green Add is the last (rightmost) control in the group").toBe(true);
  });

  test("the least-used utilities live in a labeled ⋯ More overflow", async ({ app }) => {
    await nav(app, "/transactions");
    const more = app.getByTestId("txn-more-btn");
    await expect(more).toHaveClass(/btn-tool/);
    await expect(more).toContainText("More"); // labeled, not a bare glyph
    // Export CSV / Columns are not inline — they surface only when the menu opens.
    await expect(app.getByTestId("txn-export-btn")).toBeHidden();
    await more.click();
    await expect(app.getByTestId("txn-export-btn")).toBeVisible();
    await expect(app.getByTestId("txn-columns-btn")).toBeVisible();
  });
});
