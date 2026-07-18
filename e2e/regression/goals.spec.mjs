// goals.spec.mjs — regressions for the goals redesign: the decluttered toolbar (sort
// picker, no smart/metrics, single-glyph Add), Edit moved into the ⋯ menu, the top-3
// steps cap + equal card heights, the review-reminder cadence, multi-link accounts/
// budgets, and virtual allocation (earmarking account balances with no transaction).
import { test, expect, nav } from "./fixtures.mjs";

// firstGoalId returns the id of the first rendered goal card (any kind).
async function firstGoalId(app) {
  const card = app.locator('[data-testid^="goal-row-"]').first();
  await card.scrollIntoViewIfNeeded();
  return (await card.getAttribute("data-testid")).replace("goal-row-", "");
}

// firstFinancialGoalId returns the id of the first financial goal card, or null.
async function firstFinancialGoalId(app) {
  const card = app.locator('[data-testid^="goal-row-"][data-kind="financial"]').first();
  if ((await card.count()) === 0) return null;
  await card.scrollIntoViewIfNeeded();
  return (await card.getAttribute("data-testid")).replace("goal-row-", "");
}

// expandGoal reveals a card's full body (UX-06 #71: cards default COMPACT — the
// trajectory line, Edit, and the secondary actions live behind the Details
// control). No-op when the card is already expanded.
async function expandGoal(app, gid) {
  const btn = app.locator(`[data-testid="goal-expand-${gid}"]`);
  if (await btn.count()) {
    await btn.scrollIntoViewIfNeeded();
    await btn.click();
    await app.waitForTimeout(400);
  }
}

test.describe("goals: decluttered toolbar", () => {
  test("has a Sort picker, no smart/metrics controls, and a single-glyph Add button", async ({ app }) => {
    await nav(app, "/goals");
    await expect(app.getByTestId("goals-sort")).toBeVisible();
    // The old smart-hub link and the "Goal metrics" formula toggle are gone from /goals.
    await expect(app.locator('.bento-goals [data-testid="smart-section-action"]')).toHaveCount(0);
    await expect(app.locator('[data-testid="goals-toggle-formulas"]')).toHaveCount(0);
    // The Add-goal button carries exactly ONE glyph (no plus-circle icon AND a "+" text).
    const add = app.getByTestId("goals-add");
    await expect(add).toBeVisible();
    expect(await add.locator("svg").count()).toBe(1);
    await expect(add).not.toContainText("+");
  });

  test("sorting by name orders the active goal cards A→Z", async ({ app }) => {
    await nav(app, "/goals");
    await app.getByTestId("goals-sort").selectOption("name");
    await app.waitForTimeout(300);
    const titles = await app.locator(".goal-list .goal-card .goal-card-title").allInnerTexts();
    expect(titles.length).toBeGreaterThan(1);
    const sorted = [...titles].sort((a, b) => a.localeCompare(b));
    expect(titles).toEqual(sorted);
  });
});

test.describe("goals: life-event templates", () => {
  test("a life-event chip seeds the name and an editable deadline horizon", async ({ app }) => {
    await nav(app, "/goals");
    await app.getByTestId("goals-add").click();
    await app.waitForTimeout(650); // past the FlipPanel flip
    const form = app.getByTestId("goal-add-form");
    await expect(form).toBeVisible();
    // The classic chips are still there; the life-event set joins them.
    await expect(form.getByTestId("goal-tmpl-emergency-fund")).toBeVisible();
    const wedding = form.getByTestId("goal-tmpl-wedding");
    await expect(wedding).toBeVisible();
    await expect(form.getByTestId("goal-tmpl-new-baby")).toBeVisible();
    await expect(form.getByTestId("goal-tmpl-home-down-payment")).toBeVisible();
    await expect(form.getByTestId("goal-tmpl-moving")).toBeVisible();
    // Picking Wedding seeds the name and a target date ~18 months out.
    await wedding.click();
    await expect(form.locator("#goal-add")).toHaveValue("Wedding");
    const dateVal = await form.locator('input[type="date"]').first().inputValue();
    expect(dateVal).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    const months = (new Date(dateVal) - new Date()) / (1000 * 60 * 60 * 24 * 30.44);
    expect(months).toBeGreaterThan(16);
    expect(months).toBeLessThan(20);
  });
});

test.describe("goals: priority + compare", () => {
  test("priority set in Edit shows a chip, sorts first, and feeds the compare table", async ({ app }) => {
    await nav(app, "/goals");
    // Set High priority on the first financial goal via its Edit modal.
    const gid = await firstFinancialGoalId(app);
    expect(gid).toBeTruthy();
    await expandGoal(app, gid); // compact default: Edit lives in the expanded body
    const editBtn = app.locator(`[data-testid="goal-edit-btn-${gid}"]`);
    await editBtn.scrollIntoViewIfNeeded();
    await editBtn.click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    const prio = dialog.getByTestId("goal-edit-priority");
    await prio.scrollIntoViewIfNeeded();
    await prio.selectOption("1");
    await dialog.locator('button[type="submit"]').click();
    await app.waitForTimeout(650);
    // The card now wears the High-priority chip (in the expanded head; re-expand
    // in case the save re-rendered the card back to compact).
    await expandGoal(app, gid);
    await expect(app.getByTestId(`goal-priority-${gid}`)).toContainText(/high priority/i);
    // Priority sort puts it first.
    await app.getByTestId("goals-sort").selectOption("priority");
    await app.waitForTimeout(300);
    const firstCard = app.locator('.goal-list [data-testid^="goal-row-"]').first();
    await expect(firstCard).toHaveAttribute("data-testid", `goal-row-${gid}`);
    // Compare: pick two goals, read the side-by-side figures.
    await app.getByTestId("goals-compare-btn").click();
    await app.waitForTimeout(650);
    await expect(app.getByTestId("goal-compare-form")).toBeVisible();
    await app.getByTestId("goal-compare-a").selectOption({ index: 1 });
    await app.getByTestId("goal-compare-b").selectOption({ index: 1 }); // first non-A option
    const table = app.getByTestId("goal-compare-table");
    await expect(table).toBeVisible();
    await expect(table).toContainText("Target");
    await expect(table).toContainText("To go");
    await expect(table).toContainText("Projected landing");
    await expect(table).toContainText("Priority");
    await app.keyboard.press("Escape");
  });
});

test.describe("goals: landing-range scenarios", () => {
  test("a paced financial goal shows Best/Expected/Conservative landing dates", async ({ app }) => {
    await nav(app, "/goals");
    // The trajectory line lives in the expanded card body (UX-06 compact default).
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await expandGoal(app, fid);
    const line = app.locator('[data-testid^="goal-scenarios-"]').first();
    await line.scrollIntoViewIfNeeded();
    await expect(line).toBeVisible();
    const text = await line.innerText();
    // Three labeled points, each a month-year (or the honest 10+ yrs marker).
    expect(text).toMatch(/Best (?:[A-Z][a-z]{2} \d{4}|10\+ yrs)/);
    expect(text).toMatch(/Expected (?:[A-Z][a-z]{2} \d{4}|10\+ yrs)/);
    expect(text).toMatch(/Conservative (?:[A-Z][a-z]{2} \d{4}|10\+ yrs)/);
    // The what-if definition travels in the tooltip.
    await expect(line).toHaveAttribute("title", /25% more/);
  });
});

test.describe("goals: everyday actions inline, kebab keeps only Delete", () => {
  // The contract flipped on 2026-07-16 (Cam: "move the non-delete menu opts
  // outside of the kebab"): Edit and the other everyday actions are inline
  // tool buttons on the card; the ⋯ menu holds only the destructive Delete.
  test("Edit is inline on the card; the kebab holds Delete and nothing else duplicates Set aside", async ({ app }) => {
    await nav(app, "/goals");
    // UX-06 layered on the 07-16 contract: the COMPACT face shows one primary +
    // Details; EXPANDING reveals the everyday actions inline (Edit among them)
    // while the kebab still holds only the destructive Delete.
    const gid = await firstGoalId(app);
    await expandGoal(app, gid);
    await expect(app.locator(`.goal-card-actions [data-testid="goal-edit-btn-${gid}"]`)).toBeVisible();
    const fid = await firstFinancialGoalId(app);
    const target = fid || gid;
    await expandGoal(app, target);
    await app.getByTestId(`goal-menu-btn-${target}`).click();
    // Kebab: Delete present, Edit no longer duplicated there.
    await expect(app.locator(`.add-menu [data-testid="goal-delete-btn-${target}"]`)).toBeVisible();
    await expect(app.locator(`.add-menu [data-testid="goal-edit-btn-${target}"]`)).toHaveCount(0);
    if (fid) {
      // One action, one entry point, one name: no kebab "Allocate funds"
      // duplicate — Set aside lives on the card only.
      await expect(app.locator(`.add-menu [data-testid="goal-allocate-btn-${fid}"]`)).toHaveCount(0);
      await expect(app.locator(`.goal-card-actions [data-testid="goal-setaside-${fid}"]`)).toBeVisible();
    }
  });
});

test.describe("goals: steps capped at top 3", () => {
  test("no card shows more than 3 step rows", async ({ app }) => {
    await nav(app, "/goals");
    const lists = app.locator(".goal-todos-list");
    const n = await lists.count();
    for (let i = 0; i < n; i++) {
      expect(await lists.nth(i).locator(".goal-todo").count(), `card ${i} shows >3 steps`).toBeLessThanOrEqual(3);
    }
  });
});

test.describe("goals: edit form — review cadence + multi-link", () => {
  test("the editor exposes a review-reminder picker and account/budget checklists", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    // Edit is an inline card action in the EXPANDED state (UX-06 compact default).
    await expandGoal(app, fid);
    await app.locator(`.goal-card-actions [data-testid="goal-edit-btn-${fid}"]`).click();
    await app.waitForTimeout(650); // flip
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("goal-edit-review")).toBeVisible();
    await expect(dialog.getByTestId("goal-link-accts")).toBeVisible();
    await expect(dialog.getByTestId("goal-link-budgets")).toBeVisible();
    // At least one account checkbox exists to link.
    await expect(dialog.locator('[data-testid^="goal-link-acct-"]').first()).toBeVisible();
  });
});

test.describe("goals: virtual allocation", () => {
  test("Set aside opens straight to the account list; earmarking shows the saved/set-aside legend", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");

    // The card's primary "Set aside" action IS the entry — no kebab hop, and inside
    // there is no master toggle re-asking for intent: the picker is simply the modal.
    // Expand first: the saved/set-aside legend asserted below renders only in the
    // expanded body (UX-06 compact default), and the expanded state survives the modal.
    await expandGoal(app, fid);
    await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    await expect(dialog.getByTestId("goal-alloc-toggle")).toHaveCount(0);
    await expect(dialog.locator('[data-testid^="goal-alloc-pick-"]').first()).toBeVisible();
    // The free-cash ceiling is stated up front.
    await expect(dialog.getByTestId("goal-alloc-free-total")).toContainText(/free to set aside/i);
    // Select the first account + set an amount.
    await dialog.locator('[data-testid^="goal-alloc-pick-"]').first().check();
    await app.waitForTimeout(200);
    await dialog.locator('input[data-testid^="goal-alloc-acct"]').first().fill("500");
    await expect(dialog.getByTestId("goal-alloc-summary")).toContainText(/covered/i);
    await dialog.getByTestId("goal-alloc-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    // The card now shows the saved/set-aside legend under the bar (the old status
    // badge and "% covered" sentence are gone — the bar + legend carry that state).
    await expect(app.locator(`[data-testid="goal-earmarked-${fid}"]`)).toContainText(/set aside/i);
  });

  test("an amount over the account's free balance blocks the save with a named error", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    await dialog.locator('[data-testid^="goal-alloc-pick-"]').first().check();
    await app.waitForTimeout(200);
    // Absurdly over any seed account's free balance → live row warning + save error.
    await dialog.locator('input[data-testid^="goal-alloc-acct"]').first().fill("99999999");
    await expect(dialog.locator('[data-testid^="goal-alloc-over-"]').first()).toBeVisible();
    await dialog.getByTestId("goal-alloc-save").click();
    // Modal stays open with an error naming the shortfall — never a silent clamp.
    await expect(dialog.locator(".err")).toContainText(/only has .* free/i);
  });

  test("smart split fills per-account amounts that sum to the entered total", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
    await app.waitForTimeout(650);
    const d = app.locator('[role="dialog"]');
        await d.getByTestId("goal-alloc-total").fill("1000");
    await d.getByTestId("goal-alloc-split-prop").click();
    await app.waitForTimeout(250);
    const vals = await d.locator(".goal-alloc-input").evaluateAll((els) => els.map((e) => parseFloat(e.value) || 0));
    const total = vals.reduce((a, b) => a + b, 0);
    // The seed's liquid accounts hold far more than $1,000, so the split sums to exactly it.
    expect(Math.abs(total - 1000)).toBeLessThan(0.05);
  });

  test("the Earmarks tab lists exposure + per-goal earmarks and deletes a row", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    // Create earmarks: split $500 evenly across the liquid accounts (several rows).
    await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
    await app.waitForTimeout(650);
    const d = app.locator('[role="dialog"]');
        await d.getByTestId("goal-alloc-total").fill("500");
    await d.getByTestId("goal-alloc-split-even").click();
    await app.waitForTimeout(200);
    await d.getByTestId("goal-alloc-save").click();
    await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });

    // Switch to the Earmarks tab.
    await app.getByTestId("goals-tab-earmarks").click();
    await app.waitForTimeout(400);
    await expect(app.getByTestId("goals-tab-earmarks")).toHaveClass(/is-active/);
    await expect(app.locator(".ea-exp-list")).toBeVisible();
    await expect(app.locator(`[data-testid="ea-goal-${fid}"]`)).toBeVisible();

    // Delete one earmark row → that goal shows one fewer row.
    const before = await app.locator(`[data-testid="ea-goal-${fid}"] .ea-row`).count();
    expect(before).toBeGreaterThan(1);
    await app.locator(`[data-testid^="ea-del-${fid}-"]`).first().click();
    await app.waitForTimeout(400);
    await expect(app.locator(`[data-testid="ea-goal-${fid}"] .ea-row`)).toHaveCount(before - 1);
  });

  test("the allocate picker offers any non-liability account — cash first, held assets tagged, never debts", async ({ app }) => {
    // Contract change 2026-07-16 (Cam): an earmark is a virtual reservation,
    // so ANY asset qualifies — liquid cash lists first, held assets (401k,
    // property, brokerage) follow with a "held asset" tag. Liabilities never.
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
    await app.waitForTimeout(650);
    const dialog = app.locator('[role="dialog"]');
    const names = await dialog.locator(".goal-alloc-acct").allInnerTexts();
    expect(names.length).toBeGreaterThan(0);
    // Liabilities stay out.
    for (const n of names) {
      expect(n.toLowerCase()).not.toMatch(/credit card|car loan|student loan|mortgage/);
    }
    // Held assets are offered (the seed has a 401(k) + a brokerage + the condo)…
    expect(names.join(" ").toLowerCase()).toMatch(/401|stonks|condo/);
    // …each carrying the held-asset tag, and always AFTER the liquid accounts.
    const tags = await dialog.locator(".goal-alloc-type").allInnerTexts();
    expect(tags.length).toBeGreaterThan(0);
    for (const t of tags) {
      expect(t.toLowerCase()).toContain("held asset");
    }
    const rows = dialog.locator(".goal-alloc-row");
    const rowCount = await rows.count();
    let seenHeld = false;
    for (let i = 0; i < rowCount; i++) {
      const isHeld = (await rows.nth(i).locator(".goal-alloc-type").count()) > 0;
      if (seenHeld) {
        expect(isHeld, `liquid row after a held-asset row at index ${i}`).toBe(true);
      }
      seenHeld = seenHeld || isHeld;
    }
  });
});

// earmarkGoal opens a financial goal's allocate modal, reserves `total` split evenly
// across the liquid accounts, and saves. Used by the earmark-first UI tests below.
async function earmarkGoal(app, fid, total) {
  // Works from either card state: the compact face's primary IS Set aside for an
  // active financial goal, and the expanded footer carries it too.
  await app.locator(`[data-testid="goal-setaside-${fid}"]`).click();
  await app.waitForTimeout(650);
  const d = app.locator('[role="dialog"]');
    await d.getByTestId("goal-alloc-total").fill(String(total));
  await d.getByTestId("goal-alloc-split-even").click();
  await app.waitForTimeout(200);
  await d.getByTestId("goal-alloc-save").click();
  await expect(app.locator('[role="dialog"]')).toHaveCount(0, { timeout: 15000 });
}

test.describe("goals: earmark-first reframe", () => {
  test("Set aside is the primary action; earmarking draws the two-tone bar band", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");

    // "Set aside" (earmark) is the card's primary action; "Log saved" is the secondary
    // — and the primary comes first in the footer (earmark-first ordering). The
    // secondary only shows on the expanded card (UX-06 compact default).
    await expandGoal(app, fid);
    await expect(app.locator(`.goal-card-actions [data-testid="goal-setaside-${fid}"]`)).toBeVisible();
    await expect(app.locator(`.goal-card-actions [data-testid="goal-contribute-${fid}"]`)).toBeVisible();
    const primaryFirst = await app.locator(`.goal-card-actions [data-testid="goal-setaside-${fid}"], .goal-card-actions [data-testid="goal-contribute-${fid}"]`).first().getAttribute("data-testid");
    expect(primaryFirst).toBe(`goal-setaside-${fid}`);

    // After earmarking (the seed goal may already reserve some), the hatched earmark band
    // extends the bar out to coverage, and the coverage line reports the reserved money.
    await earmarkGoal(app, fid, 500);
    await expect(app.locator(`[data-testid="goal-bar-earmark-${fid}"]`)).toBeVisible();
    await expect(app.locator(`[data-testid="goal-earmarked-${fid}"]`)).toContainText(/set aside/i);
  });

  test("the Earmarks tab opens with a money-map reconciliation (in accounts → earmarked → free)", async ({ app }) => {
    await nav(app, "/goals");
    const fid = await firstFinancialGoalId(app);
    test.skip(!fid, "no financial goal in the seed");
    await earmarkGoal(app, fid, 800);

    await app.getByTestId("goals-tab-earmarks").click();
    await app.waitForTimeout(400);
    const map = app.getByTestId("earmarks-moneymap");
    await expect(map).toBeVisible();
    // All three reconciliation figures are present and labelled.
    await expect(map).toContainText(/in accounts/i);
    await expect(map).toContainText(/earmarked/i);
    await expect(map).toContainText(/free to assign/i);
    // The earmarked-share bar rendered.
    await expect(map.locator(".ea-map-bar-fill")).toBeVisible();
  });
});

test.describe("dashboard: Goals-at-a-glance widget", () => {
  test("shows current / missed / completed counts and each opens Goals", async ({ app }) => {
    await nav(app, "/");
    // The tile is below the fold and its body renders on scroll-in (perf deferral).
    const title = app.getByText("Goals at a glance", { exact: true }).first();
    await title.scrollIntoViewIfNeeded();
    await expect(app.getByTestId("goal-states-current")).toBeVisible({ timeout: 10000 });
    await expect(app.getByTestId("goal-states-missed")).toBeVisible();
    await expect(app.getByTestId("goal-states-completed")).toBeVisible();
    // The current count is a positive number in the seed.
    const cur = await app.locator('[data-testid="goal-states-current"] .dgs-n').innerText();
    expect(parseInt(cur, 10)).toBeGreaterThan(0);
    // Clicking a count opens the Goals page.
    await app.getByTestId("goal-states-current").click();
    await expect(app.locator('#main[data-route="/goals"]')).toBeVisible({ timeout: 10000 });
  });
});
