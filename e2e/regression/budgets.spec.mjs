// budgets.spec.mjs — regressions for the budgets card footer: only the everyday money
// moves are inline (Cover/Top-up, Transactions) and everything configurational lives in
// the ⋯ overflow (Edit budget / Edit tracking / Notes / Formulas, with the danger group —
// Remove recurring, Delete — at its bottom); the enhanced top-up (this-month vs permanent
// + fund-from-budgets); the notes modal; the copyable formulas modal; the sort picker; and
// the 50/30/20 template living inside the Add-budget modal.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("budgets: release unused funds", () => {
  test("the kebab's Release lowers this period's cap to what's spent", async ({ app }) => {
    await nav(app, "/budgets");
    // Find a budget whose kebab offers Release (remaining > 0).
    const bid = (await app.locator('[data-testid^="budget-release-btn-"]').first().getAttribute("data-testid"))
      .replace("budget-release-btn-", "");
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-release-btn").click();
    // Confirm the release; the undoable toast names the amount and budget.
    await app.locator("#cf-dialog-confirm").click();
    await expect(app.locator("body")).toContainText(/Released .* from /);
    // The row's Release action is gone — nothing left to release this period.
    await openBudgetMenu(app, bid);
    await expect(app.getByTestId(`budget-release-btn-${bid}`)).toHaveCount(0);
    await app.keyboard.press("Escape");
  });
});

// firstBudgetId returns the id of the first budget row (derived from its kebab button —
// the one per-card action that is always visible).
async function firstBudgetId(app) {
  const kebab = app.locator('[data-testid^="budget-kebab-"]').first();
  await kebab.scrollIntoViewIfNeeded();
  return (await kebab.getAttribute("data-testid")).replace("budget-kebab-", "");
}

// useCardDensity switches to the full-card layout. The sample household has more
// than six budgets, so the page SEEDS the compact list — the card footer these
// assertions describe is not what a fresh context opens on.
async function useCardDensity(app) {
  const density = app.getByTestId("budgets-density");
  await density.scrollIntoViewIfNeeded();
  if ((await density.getAttribute("aria-pressed")) === "true") await density.click();
  await expect(app.locator(".budget-grid .budget").first()).toBeVisible();
}

// menuItem locates one entry inside a budget row's open ⋯ overflow.
function menuItem(app, bid, testid) {
  return app.locator(`.add-menu [data-testid="${testid}-${bid}"]`);
}

// openBudgetMenu opens a card's ⋯ overflow so a menu item can be clicked.
//
// The generous timeout is not padding. The menu is positioned by AnchorPopover,
// which measures and repositions inside requestAnimationFrame, and rAF in this
// headless environment is throttled to roughly a frame every two seconds. The
// menu therefore becomes visible seconds after the click that opened it, which
// is comfortably past Playwright's 5s default — the source of the intermittent
// "resolved to <button …> - unexpected value hidden" failures across this file.
// Waiting on the real signal is correct; the item is in the DOM the whole time,
// so polling for its presence rather than its visibility proves nothing.
async function openBudgetMenu(app, bid) {
  await app.locator(`[data-testid="budget-kebab-${bid}"]`).scrollIntoViewIfNeeded();
  await app.locator(`[data-testid="budget-kebab-${bid}"]`).click();
  await expect(app.locator(`.add-menu [data-testid="edit-budget-btn-${bid}"]`)).toBeVisible({ timeout: 30000 });
}

test.describe("budgets: slim footer + kebab", () => {
  test("only money moves are inline; config + Delete live in the ⋯ menu", async ({ app }) => {
    await nav(app, "/budgets");
    await useCardDensity(app);
    const bid = await firstBudgetId(app);
    // Inline: the Transactions drill and the kebab (plus Cover/Top-up, which depend on
    // the budget's over/under state, so they aren't asserted per-card here).
    await expect(app.locator(`.budget-actions [data-testid="budget-view-txns-${bid}"]`)).toBeVisible();
    await expect(app.locator(`.budget-actions [data-testid="budget-kebab-${bid}"]`)).toBeVisible();
    // The configurational actions are NOT visible footer buttons anymore…
    for (const t of ["edit-budget-btn", "edit-budget-cats-btn", "budget-notes-btn", "budget-formulas-btn", "delete-budget-btn"]) {
      await expect(app.locator(`[data-testid="${t}-${bid}"]`)).not.toBeVisible();
    }
    // …they appear in the ⋯ menu, with the destructive Delete at its bottom
    // (standing directive: delete stays in the kebab).
    await app.locator(`[data-testid="budget-kebab-${bid}"]`).click();
    for (const t of ["edit-budget-btn", "edit-budget-cats-btn", "budget-notes-btn", "budget-formulas-btn", "delete-budget-btn"]) {
      await expect(app.locator(`.add-menu [data-testid="${t}-${bid}"]`)).toBeVisible();
    }
  });
});

test.describe("budgets: density toggle", () => {
  test("compact list renders one-line rows, keeps actions in the kebab, and persists", async ({ app }) => {
    await nav(app, "/budgets");
    // The page seeds compact for a household past six budgets, so start from the
    // card layout deliberately rather than assuming which one a fresh context
    // opens on.
    await useCardDensity(app);
    const toggle = app.getByTestId("budgets-density");
    await expect(toggle).toHaveAttribute("aria-pressed", "false");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
    // Cards become compact ledger rows; the card grid is gone.
    await expect(app.locator(".budget-clist .budget-crow").first()).toBeVisible();
    await expect(app.locator(".budget-grid")).toHaveCount(0);
    // The compact row keeps a one-click, LABELLED path to the ledger in its own
    // action cell (C591: the budget name is a heading again, so the drill has to
    // be a control that says what it is), and everything configurational stays in
    // the ⋯ menu.
    const bid = await firstBudgetId(app);
    const drill = app.locator(`.budget-crow-actions [data-testid="budget-view-txns-${bid}"]`);
    await expect(drill).toBeVisible();
    await expect(drill).toHaveAttribute("aria-label", /see the transactions in/i);
    await expect(app.locator(`[data-testid="budget-card-${bid}"] button.budget-crow-name`)).toHaveCount(0);
    await app.locator(`[data-testid="budget-kebab-${bid}"]`).click();
    await expect(app.locator(`.add-menu [data-testid="edit-budget-btn-${bid}"]`)).toBeVisible();
    await app.keyboard.press("Escape");
    // The choice persists across a reload (localStorage-backed).
    await app.reload();
    await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 45000 });
    await nav(app, "/budgets");
    await expect(app.locator(".budget-clist .budget-crow").first()).toBeVisible();
    // Restore the default so later specs see the card layout.
    await app.getByTestId("budgets-density").click();
    await expect(app.locator(".budget-grid .budget").first()).toBeVisible();
  });
});

test.describe("budgets: enhanced top-up", () => {
  test("top-up offers this-month vs permanent + a fund-from-budgets checklist", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = (await app.locator('[data-testid^="budget-topup-btn-"]').first().getAttribute("data-testid"))
      .replace("budget-topup-btn-", "");
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-topup-btn").click();
    await app.waitForTimeout(650); // flip
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("topup-dur-month")).toBeVisible();
    await expect(dialog.getByTestId("topup-dur-perm")).toBeVisible();
    // Default is "this month" — the hint says so.
    await expect(dialog.getByTestId("topup-dur-hint")).toContainText(/this period only/i);
    // Expand the funding checklist — it lists budgets with room to give.
    await dialog.getByTestId("topup-cover-toggle").click();
    await expect(dialog.locator('[data-testid^="topup-src-"]').first()).toBeVisible();
  });
});

test.describe("budgets: notes modal", () => {
  test("adding a note via the kebab modal shows a readable notes line on the card", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstBudgetId(app);
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-notes-btn").click();
    await app.waitForTimeout(650);
    const note = "Trim this once the baby-gear splurge settles — revisit in Q4.";
    await app.locator('[role="dialog"] textarea').first().fill(note);
    await app.getByTestId("budget-notes-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });
    // The row carries a note marker afterwards. In COMPACT density — the default,
    // and what this suite runs in — the marker is the icon beside the budget name
    // and the note text rides on its tooltip; the clamped text preview belongs to
    // the card layout. Asserting the title rather than the text content is what
    // compact actually renders, and it still proves the saved text reached the row.
    const line = app.locator(`[data-testid="budget-notes-${bid}"]`);
    await expect(line).toBeVisible();
    await expect(line).toHaveAttribute("title", note);
    // Clicking the marker reopens the full note in the flip modal (the old inline
    // aria-expanded expander was retired with the side-panel design).
    await line.click();
    await app.waitForTimeout(650);
    await expect(app.locator('[role="dialog"] textarea').first()).toHaveValue(note);
  });

  // C545 proper: the note was reported as unsavable. It saved and persisted the
  // whole time — the row simply showed no trace of it in the default density, so
  // the write was indistinguishable from a failure. This pins BOTH halves: the
  // note survives a reload, and the row says so.
  test("a saved note survives a reload and the row still shows it", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstBudgetId(app);
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-notes-btn").click();
    await app.waitForTimeout(650);
    const note = "Renews in March — cancel before it auto-bills.";
    await app.locator('[role="dialog"] textarea').first().fill(note);
    await app.getByTestId("budget-notes-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    await app.reload({ waitUntil: "networkidle" });
    await nav(app, "/budgets");
    const marker = app.locator(`[data-testid="budget-notes-${bid}"]`);
    await expect(marker).toBeVisible({ timeout: 20000 });
    await expect(marker).toHaveAttribute("title", note);
    // ...and the stored text is the whole sentence, not a truncated one. A
    // multi-line field that rewrote its own content on every keystroke used to
    // drop characters mid-word, so the note saved as a fragment of what was typed.
    await marker.click();
    await app.waitForTimeout(650);
    await expect(app.locator('[role="dialog"] textarea').first()).toHaveValue(note);
  });

  // C614: the editor names the action, and names it by state. "Notes" was the only
  // noun in a menu of verb phrases, and it read identically whether it was about to
  // create a note or overwrite one.
  test("the editor says whether it will add a note or edit one", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstBudgetId(app);

    await openBudgetMenu(app, bid);
    const item = menuItem(app, bid, "budget-notes-btn");
    await expect(item).toHaveText("Add a note");
    await item.click();
    await app.waitForTimeout(650);
    await expect(app.locator('[role="dialog"]')).toContainText("Add a note");
    // Nothing to remove yet, so no remove control is offered.
    await expect(app.getByTestId("budget-notes-remove")).toHaveCount(0);

    await app.locator('[role="dialog"] textarea').first().fill("Review this in the autumn.");
    await app.getByTestId("budget-notes-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    await openBudgetMenu(app, bid);
    await expect(menuItem(app, bid, "budget-notes-btn")).toHaveText("Edit note");
    await menuItem(app, bid, "budget-notes-btn").click();
    await app.waitForTimeout(650);
    await expect(app.locator('[role="dialog"]')).toContainText("Edit note");
    await expect(app.getByTestId("budget-notes-remove")).toBeVisible();
  });

  // Removing a note used to be reachable only by clearing the field and pressing
  // Save — a destructive gesture with no name. It is now an explicit, confirmed act.
  test("a note can be removed explicitly, and the removal confirms first", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstBudgetId(app);
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-notes-btn").click();
    await app.waitForTimeout(650);
    await app.locator('[role="dialog"] textarea').first().fill("This note is about to go.");
    await app.getByTestId("budget-notes-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });
    await expect(app.locator(`[data-testid="budget-notes-${bid}"]`)).toBeVisible();

    await app.locator(`[data-testid="budget-notes-${bid}"]`).click();
    await app.waitForTimeout(650);
    await app.getByTestId("budget-notes-remove").click();
    await app.waitForTimeout(500);
    // It asks before discarding text it cannot give back.
    const confirm = app.getByRole("button", { name: /^(remove|confirm|yes|ok)$/i }).last();
    await confirm.click();
    await app.waitForTimeout(1200);

    await expect(app.locator(`[data-testid="budget-notes-${bid}"]`)).toHaveCount(0);
    await app.reload({ waitUntil: "networkidle" });
    await nav(app, "/budgets");
    await expect(app.locator(`[data-testid="budget-notes-${bid}"]`)).toHaveCount(0);
  });
});

test.describe("budgets: formulas modal", () => {
  test("shows the budget's variables with copy buttons", async ({ app }) => {
    await nav(app, "/budgets");
    const bid = await firstBudgetId(app);
    await openBudgetMenu(app, bid);
    await menuItem(app, bid, "budget-formulas-btn").click();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("budget-formulas")).toBeVisible();
    // The five per-budget variables, each with a copy button.
    await expect(app.locator('[data-testid^="budget-formula-copy-"]')).toHaveCount(5);
    await expect(app.locator('[data-testid^="budget-formula-name-"]').first()).toContainText(/^budget_.*_limit$/);
  });
});

test.describe("budgets: sort + add template", () => {
  // Reads every rendered budget card and derives its overage ($ over the limit) and its
  // distance from the limit line in percentage points (|% used − 100|), from the values
  // the card actually shows — so we assert the REAL on-screen order, not the intent.
  async function derivedOrder(app) {
    return app.evaluate(() =>
      [...document.querySelectorAll(".budget-grid .budget")].map((c) => {
        const amt = (c.querySelector(".budget-amount")?.textContent || "").trim();
        const pct = parseInt(c.querySelector(".budget-pct")?.textContent || "0", 10) || 0;
        const toC = (s) => Math.round(parseFloat((s || "").replace(/[^0-9.\-]/g, "")) * 100) || 0;
        const [spent, limit] = amt.split("/").map(toC);
        return { name: (c.querySelector(".row-desc")?.textContent || "").trim(), over: Math.max(0, spent - limit), dist: Math.abs(pct - 100) };
      }),
    );
  }

  test("over budget sorts by severity (overage ↓); close-to-limit by |% − 100| (↑)", async ({ app }) => {
    await nav(app, "/budgets");
    // Over budget → overage strictly non-increasing down the list (worst overspend first).
    await app.getByTestId("budgets-sort").selectOption("overage");
    await app.waitForTimeout(400);
    const ov = await derivedOrder(app);
    expect(ov.length).toBeGreaterThan(2);
    for (let i = 1; i < ov.length; i++) {
      expect(ov[i - 1].over, `overage not descending at ${i} (${ov[i - 1].name}→${ov[i].name})`).toBeGreaterThanOrEqual(ov[i].over);
    }
    // Close to the limit → |% used − 100| non-decreasing (a 99%/101% budget beats a 200%
    // one, and 0%-used budgets — 100 points away — rank LAST, not first).
    await app.getByTestId("budgets-sort").selectOption("near");
    await app.waitForTimeout(400);
    const nr = await derivedOrder(app);
    for (let i = 1; i < nr.length; i++) {
      expect(nr[i - 1].dist, `|%-100| not ascending at ${i} (${nr[i - 1].name}→${nr[i].name})`).toBeLessThanOrEqual(nr[i].dist);
    }
    // A 0%-used budget must not be first under "close to the limit".
    expect(nr[0].dist).toBeLessThan(100);
  });

  test("the 50/30/20 template lives inside the Add-budget modal", async ({ app }) => {
    await nav(app, "/budgets");
    await app.getByTestId("budgets-add").click();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("budget-add-tmpl")).toBeVisible();
    await expect(app.getByTestId("budgets-template-503020")).toBeVisible();
  });
});


test.describe("budgets: card density", () => {
  // C595: the card stacked every caption it had — tracking, tags, committed,
  // set-aside, target, coverage, owner, method, custom fields, pace, carry,
  // metrics — between the bar and the actions, so "how much is left?" competed
  // with a dozen explanations of how it was arrived at.
  test("a card leads with the headline and folds its explanations away", async ({ app }) => {
    await nav(app, "/budgets");
    // The sample household defaults to compact; this ticket is about the CARD.
    const density = app.getByTestId("budgets-density");
    await density.scrollIntoViewIfNeeded();
    if ((await density.getAttribute("aria-pressed")) === "true") await density.click();
    await expect(app.locator(".budget-grid .budget").first()).toBeVisible();

    const bid = await firstBudgetId(app);
    const card = app.getByTestId(`budget-card-${bid}`);
    await card.scrollIntoViewIfNeeded();
    const collapsed = (await card.boundingBox()).height;

    // The primary summary is present without opening anything.
    await expect(card).toContainText(/(Over budget|On track|Near|At risk)/i);
    // The explanatory block is behind one labelled, keyboard-reachable control.
    const toggle = app.getByTestId(`budget-details-${bid}`);
    await expect(toggle).toBeVisible();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(toggle).toHaveAttribute("aria-label", /details for/i);
    await expect(app.getByTestId(`budget-details-body-${bid}`)).toHaveCount(0);

    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await expect(app.getByTestId(`budget-details-body-${bid}`)).toBeVisible();
    const expanded = (await card.boundingBox()).height;
    expect(expanded, "the disclosure must actually be saving vertical space").toBeGreaterThan(collapsed);

    // Every action stays out of the disclosure — it hides explanations, not
    // things the user can do.
    await expect(app.getByTestId(`budget-view-txns-${bid}`)).toBeVisible();
    await expect(app.getByTestId(`budget-kebab-${bid}`)).toBeVisible();

    // Restore the default density for later specs.
    await density.click();
  });
});
