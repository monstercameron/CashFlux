// budgets_zerobased_journey.spec.mjs — C598.
//
// The other budgets specs check controls. This one checks the JOURNEY: a person
// switches the household to zero-based budgeting, reads what income the plan is
// built on versus what has arrived, creates a budget for a category that does
// not exist yet, tunes limits one at a time and in bulk, drills into a budget's
// spending, and comes back to the same period they started in.
//
// It is one continuous scenario on purpose. Every step passed in isolation before
// this file existed; what nobody had checked was whether they COMPOSE — whether
// a budget created against a new category is still there after a refresh
// (C584 said it was not, for every budget, for four seconds), whether a
// drill-through returns the transactions its own total counted (C585 said no),
// whether the period survives a route change (C588's acceptance criterion),
// whether a bulk adjustment does what its preview promised (C592).
//
// It also records the SHAPE of the journey — route changes, refreshes survived,
// and how many times state had to be rebuilt by hand — because the ticket asks
// for those to be measured, and a number that drifts upward is the regression.
import { test, expect } from "@playwright/test";
import { boot, nav, settle, openVia } from "./fixtures.mjs";

const SETTINGS_MENU = ".bud-set-menu:not(.hidden-menu)";
const NEW_BUDGET = "Pet care";

// reloadOnBudgets is the step this journey repeats after every mutation: leave,
// come back, and hard-refresh. It is the exact gesture C584 lost data to — the
// dataset autosaved on a 4-second ticker and IndexedDB writes cannot be flushed
// on pagehide — so a journey that never reloads would have passed throughout.
async function reloadOnBudgets(page) {
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 45_000 },
  );
  // A reload lands on the URL it left, so the app usually mounts /budgets by
  // itself. Waiting for the route to appear is the reliable signal; pushing the
  // SAME path and firing popstate does not re-render, which is what made a
  // nav() here time out looking for a route the app was already on.
  await page.waitForSelector("#main[data-route]", { timeout: 45_000 });
  await go(page, "/budgets", "Budgets");
}

// dataGen reads the app's own "the dataset is written" stamp. Every successful
// autosave stamps a fresh generation into `cashflux:dataset:gen` (it exists as the
// cross-tab entitlement marker, but it moves on exactly the event this journey
// needs to observe), so it is the honest signal that a mutation has reached the
// store rather than merely reached the screen.
async function dataGen(page) {
  return page.evaluate(() => localStorage.getItem("cashflux:dataset:gen"));
}

// savedSince blocks until the dataset has been written since `before`.
//
// The autosave is a debounced background write, so a refresh fired the instant a
// control closes can beat it and take the pre-edit bytes with it — which is how
// this journey first came back from a reload holding the OLD limit. Waiting on
// the app's own signal, rather than sleeping a guessed number of milliseconds,
// keeps the refresh honest: it proves the write landed instead of hiding a race
// behind a timeout that happens to be long enough today.
async function savedSince(page, before) {
  await expect
    .poll(() => dataGen(page), {
      timeout: 15_000,
      message: "the app never reported the dataset written",
    })
    .not.toBe(before);
}

// go navigates the way a user does — by clicking the rail — and only falls back
// to a synthetic popstate if no link is reachable.
//
// This journey cannot use the shared nav() helper throughout: that fires a
// popstate, and after ANY in-app navigation the router stops responding to one
// (C610), so a step that follows a drill-through or a modal's own Navigate would
// wait forever for a route the app never mounts. Clicking the rail is both the
// honest gesture and the one that works.
async function go(page, route, linkName) {
  const current = await page.getAttribute("#main", "data-route").catch(() => null);
  if (current === route) return;
  const link = page.getByRole("link", { name: linkName }).first();
  if (await link.count()) {
    await link.click({ force: true });
  } else {
    await nav(page, route);
  }
  await page.waitForSelector(`#main[data-route="${route}"]`, { timeout: 45_000 });
}

async function openSettings(page) {
  await page.getByTestId("budgets-settings").scrollIntoViewIfNeeded();
  await openVia(page, page.getByTestId("budgets-settings"), page.locator(SETTINGS_MENU));
}

// closeSettings dismisses the popover. Its backdrop covers the page while open,
// so leaving it up makes the NEXT step's click land on a transparent div — which
// is not a bug in the app, but is a bug in a journey that forgets to close it.
async function closeSettings(page) {
  await page.keyboard.press("Escape");
  await expect(page.locator(SETTINGS_MENU)).toHaveCount(0);
}

test.describe("C598 — the zero-based month, end to end", () => {
  // Not tagged @prod: the deploy gate has to finish fast and be believed, and
  // this is a long multi-mutation scenario. It runs in the full lane.
  test("set the method → read the income → create → assign → adjust → drill → return", async ({ page }) => {
    // The ticket asks for a refresh at every mutation, and each one is a full
    // ~10s wasm boot. That is the cost of proving the writes are durable, so the
    // budget is raised deliberately rather than the refreshes being dropped.
    test.slow();
    const routeChanges = [];
    const refreshes = [];
    let lostState = 0;
    const started = Date.now();

    await boot(page);
    await go(page, "/budgets", "Budgets");
    routeChanges.push("/budgets");

    const period = (await page.getByTestId("period-pill").innerText()).trim();
    expect(period, "the journey needs a period to return to").toBeTruthy();

    // ---- 1. SWITCH THE HOUSEHOLD TO ZERO-BASED -----------------------------
    // The control has to say what it governs before it is used: it and the
    // per-budget picker were both called "method" (C590).
    await openSettings(page);
    await expect(page.locator(SETTINGS_MENU)).toContainText(/whole household/i);
    await page.locator(SETTINGS_MENU).getByTestId("budgets-method").selectOption("zero-based");
    // It reports what actually moved, rather than "saved".
    await expect(page.locator("body")).toContainText(/household method is now/i);
    await closeSettings(page);

    // ---- 2. READ THE INCOME: EXPECTED VERSUS RECEIVED ----------------------
    // The plan's denominator and the money in hand are different numbers, and the
    // page has to keep them apart (C587).
    await expect(page.getByTestId("budgets-income-actual")).toContainText(/of the .* your plan expects/i);
    const funded = page.getByTestId("budgets-funded-callout");
    if (await funded.count()) {
      await expect(funded).toContainText(/isn't funded yet/i);
      await expect(funded).toContainText(/has actually arrived/i);
    }

    // ---- 3. CREATE A BUDGET FOR A CATEGORY THAT DOES NOT EXIST -------------
    const genBeforeCreate = await dataGen(page);
    await page.getByTestId("budgets-add").click();
    await expect(page.getByTestId("budget-add-form")).toBeVisible();
    await page.locator("#budget-add").fill(NEW_BUDGET);
    await page.getByTestId("budget-create-cat").click();
    // A brand-new category has no history, and the form says so instead of
    // borrowing another category's average (C586).
    await expect(page.getByTestId("budget-suggest-none")).toContainText(/no history yet/i);
    await page.locator('[data-testid="budget-add-form"] input[type="number"]').first().fill("120");
    await page.locator('[data-testid="budget-add-form"] button[type="submit"]').click();
    await expect(page.getByTestId("budget-add-form")).toHaveCount(0);
    await expect(page.locator("#main")).toContainText(NEW_BUDGET);

    // …and it is still there after a refresh. This is the C584 gesture.
    await savedSince(page, genBeforeCreate);
    await reloadOnBudgets(page);
    refreshes.push("after create");
    await expect(page.locator("#main"), "a new budget must survive a refresh").toContainText(NEW_BUDGET);
    expect((await page.getByTestId("period-pill").innerText()).trim(), "the period must survive a refresh").toBe(period);

    // The category it created is real, and visible where categories live.
    await go(page, "/categories", "Categories");
    routeChanges.push("/categories");
    await expect(page.locator("#main")).toContainText(NEW_BUDGET);
    await go(page, "/budgets", "Budgets");
    routeChanges.push("/budgets");
    if ((await page.getByTestId("period-pill").innerText()).trim() !== period) lostState++;

    // ---- 4. EDIT ONE LIMIT IN PLACE ---------------------------------------
    // The inline limit editor belongs to the CARD layout, and the page seeds the
    // compact list for a household past six budgets — so the journey has to ask
    // for cards rather than assume them, or this step silently does nothing.
    const density = page.getByTestId("budgets-density");
    await density.scrollIntoViewIfNeeded();
    if ((await density.getAttribute("data-density")) === "compact") await density.click();
    await expect(page.locator(".budget-grid .budget").first()).toBeVisible();

    const cardId = await page
      .locator('[data-testid^="budget-card-"]')
      .first()
      .getAttribute("data-testid")
      .then((t) => t.replace("budget-card-", ""));
    const limitBtn = page.getByTestId(`budget-limit-btn-${cardId}`);
    await expect(limitBtn, "the card layout offers an inline limit editor").toBeVisible();
    {
      const genBeforeLimit = await dataGen(page);
      await limitBtn.click();
      const input = page.getByTestId(`budget-limit-input-${cardId}`);
      await expect(input).toBeVisible();
      await input.fill("321");
      await page.getByTestId(`budget-limit-save-${cardId}`).click();
      await expect(input).toHaveCount(0);
      await savedSince(page, genBeforeLimit);

      await reloadOnBudgets(page);
      refreshes.push("after inline limit edit");
      // Read the BASE limit back through the editor rather than off the card.
      // A budget that rolls over shows its EFFECTIVE cap there — "$242.50 /
      // $205.20" on a $450 limit carrying $244.80 of debt — so asserting the
      // typed figure appears on the card would be asserting the wrong number.
      await page.getByTestId(`budget-limit-btn-${cardId}`).click();
      await expect(
        page.getByTestId(`budget-limit-input-${cardId}`),
        "an inline limit edit must survive a refresh",
      ).toHaveValue("321.00");
      await page.getByTestId(`budget-limit-cancel-${cardId}`).click();
    }

    // ---- 5. ADJUST EVERY BUDGET AT ONCE, ON A PREVIEW ---------------------
    // The bulk tool has to show the consequence before it commits (C592), and the
    // write has to match what it showed.
    const genBeforeAdjust = await dataGen(page);
    await openSettings(page);
    await openVia(page, page.getByTestId("budgets-adjust-all"), page.getByTestId("adjustall-form"));
    await page.getByTestId("adjustall-pct").fill("5");
    await expect(page.getByTestId("adjustall-count")).toContainText(/will change by 5%/i);
    const firstRow = page.locator('[data-testid^="adjustall-row-"]').first();
    const rowId = (await firstRow.getAttribute("data-testid")).replace("adjustall-row-", "");
    const promised = (await firstRow.innerText()).trim().split(/\s+/).pop();
    await page.getByTestId("adjustall-apply").click();
    await expect(page.getByTestId("adjustall-form")).toHaveCount(0);
    await expect(
      page.getByTestId(`budget-card-${rowId}`),
      "the write must match the preview exactly",
    ).toContainText(promised);

    await savedSince(page, genBeforeAdjust);
    await reloadOnBudgets(page);
    refreshes.push("after bulk adjust");
    await expect(page.getByTestId(`budget-card-${rowId}`)).toContainText(promised);

    // ---- 6. DRILL INTO A BUDGET'S SPENDING --------------------------------
    // The ledger it opens must contain the transactions its own total counted —
    // the contradiction C585 was filed for.
    const spender = page.locator('[data-testid^="budget-view-txns-"]').first();
    const spenderId = (await spender.getAttribute("data-testid")).replace("budget-view-txns-", "");
    const spentText = await page.getByTestId(`budget-card-${spenderId}`).innerText();
    const spent = (spentText.match(/\$[\d,]+\.\d{2}/) || [])[0];
    await spender.scrollIntoViewIfNeeded();
    await spender.click();
    await page.waitForSelector('#main[data-route="/transactions"]', { timeout: 25_000 });
    routeChanges.push("/transactions");

    // The ledger says which budget it came from, so the scope is not an anonymous
    // pile of chips.
    await expect(page.locator("#main")).toContainText(/From the .* budget/i);
    // And it is not empty under a non-zero total.
    if (spent && spent !== "$0.00") {
      await expect(page.locator("#main"), "a budget with spend must not open an empty ledger").not.toContainText(
        "No matching transactions",
      );
    }

    // ---- 7. RETURN TO THE SAME PERIOD -------------------------------------
    // Back via the rail link, the way a user can. Deliberately NOT a synthetic
    // popstate: after an in-app navigation — which the drill above is — the
    // router ignores one entirely, so the browser Back button changes the URL
    // and leaves the screen behind. That is filed as C610 and is not this
    // journey's subject; the journey uses a path that works, and C610 owns the
    // one that does not.
    await go(page, "/budgets", "Budgets");
    routeChanges.push("/budgets");
    await settle(page);
    const back = (await page.getByTestId("period-pill").innerText()).trim();
    if (back !== period) lostState++;
    expect(back, "the journey must end where it started").toBe(period);
    // The budget created at the start is still here at the end.
    await expect(page.locator("#main")).toContainText(NEW_BUDGET);

    // ---- SHAPE ------------------------------------------------------------
    // Recorded, and bounded. The numbers are the ticket's "measured completion
    // time and action count"; a rise in either is the regression this catches.
    const seconds = Math.round((Date.now() - started) / 1000);
    // eslint-disable-next-line no-console
    console.log(
      `C598 journey: ${routeChanges.length} route changes (${routeChanges.join(" → ")}), ` +
        `${refreshes.length} refreshes survived (${refreshes.join(", ")}), ` +
        `${lostState} pieces of lost state, ${seconds}s`,
    );
    expect(lostState, "state must survive every route change and refresh").toBe(0);
    expect(routeChanges.length, "a monthly budgeting pass should not need many route changes").toBeLessThanOrEqual(8);
  });
});
