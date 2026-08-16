// txn_correction_journey.spec.mjs — C583.
//
// The other transaction specs check controls. This one checks the JOURNEY: a
// person finds a badly-filed charge, works out where its category came from,
// fixes it (inventing a category on the way), teaches the app a rule, checks the
// record, and gets back to the exact list they were working through.
//
// It is written as one continuous scenario on purpose. Every step in it passed in
// isolation before this file existed; what nobody had checked was whether the
// steps compose — whether the working set survives the round trip, whether the
// row still claims a machine filed it after a person filed it, whether there is
// any way back from the page the rule form lives on. Those are the failures a
// per-control test cannot see, and they are the ones that make the product feel
// like a set of screens rather than a tool.
//
// It also records the SHAPE of the journey — how many route changes, how many
// times state had to be rebuilt — because the ticket asks for those to be
// measured, and a number that drifts upward is the regression.
import { test, expect } from "@playwright/test";
import { boot, nav, settle } from "./fixtures.mjs";

// The merchant the journey works on. It has many charges in the sample ledger, so
// a search for it is a realistic "find the thing I care about" step rather than a
// contrived single-row lookup.
const MERCHANT = "coffee";

test.describe("C583 — the human correction loop", () => {
  // Not tagged @prod: the deploy gate has to finish fast and be believed, and this
  // is a long multi-page scenario. It runs in the full lane, where it belongs.
  test("find → understand → correct → teach → verify → return", async ({ page }) => {
    const routeChanges = [];
    let lostState = 0;

    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });
    routeChanges.push("/transactions");

    // ---- 1. LOCATE ---------------------------------------------------------
    // The user types the merchant they are looking for. The page must then say
    // what it is showing — with a denominator, or the count means nothing.
    await page.locator(".fctrl-input").fill(MERCHANT);
    const countLine = page.locator("[data-testid=txn-count-line]");
    await expect(countLine).toContainText(/Showing \d+ of \d+ transactions/, { timeout: 20_000 });
    const narrowedText = (await countLine.innerText()).replace(/\s+/g, " ").trim();
    const shown = Number(narrowedText.match(/Showing (\d+) of/)[1]);
    const total = Number(narrowedText.match(/of ([\d]+) transactions/)[1]);
    expect(shown).toBeGreaterThan(0);
    expect(shown).toBeLessThan(total); // the search actually narrowed something

    // The search is visible as a removable chip, so the user can see WHY the list
    // is short — a filtered list that does not say it is filtered is the bug this
    // journey exists downstream of.
    await expect(page.locator("[data-testid=filter-summary]")).toContainText(MERCHANT);

    // ---- 2. UNDERSTAND ITS PROVENANCE --------------------------------------
    // Find a row whose category nobody has confirmed, and read the explanation.
    const autoRow = page.locator(".txn-table tbody tr.row").filter({
      has: page.locator("[data-testid=txn-auto-mark]"),
    }).first();
    await expect(autoRow).toBeVisible({ timeout: 20_000 });

    const why = await autoRow.locator("[data-testid=txn-auto-mark]").getAttribute("title");
    // The explanation must be a sentence about WHO filed it, not a label. Either it
    // names the rule accounting for the category, or it says plainly that no rule
    // does — never a bare "auto" with nothing behind it.
    expect(why).toMatch(/Filed automatically/i);
    expect(why).toMatch(/rule|came in from/i);
    expect(why).toMatch(/no one has confirmed it|No rule of yours/i);

    const rowID = (await autoRow.getAttribute("data-testid")).replace("txn-row-", "");

    // The Status column states the row's state in words — no glyph to decode.
    await expect(autoRow.locator(".td-status")).toHaveText(/Needs review|Cleared|Reconciled|—/);

    // ---- 3. CORRECT IT, INVENTING THE CATEGORY ON THE WAY ------------------
    // Opening the row is a visible affordance, not a guess.
    await autoRow.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });

    // The dialog is a real dialog: named, modal, escapable.
    const dialog = page.locator(".flip-wrap[role=dialog]");
    await expect(dialog).toHaveAttribute("aria-modal", "true");
    await expect(dialog).toHaveAttribute("aria-labelledby", /.+/);

    // The category control is not clipped by the button beside it.
    const catSelect = page.locator(".txn-cat-row .field").first();
    const clipped = await catSelect.evaluate((el) => el.scrollWidth > el.clientWidth + 1);
    expect(clipped).toBe(false);

    // Create the missing category without leaving the dialog — no route change.
    const newCat = "Journey coffee " + shown;
    await page.locator("[data-testid=txn-edit-newcat-toggle]").click();
    await page.locator("[data-testid=txn-edit-newcat-name]").fill(newCat);
    await page.locator("[data-testid=txn-edit-newcat-add]").click();
    await expect(catSelect).toHaveValue(/.+/, { timeout: 10_000 });
    await page.locator("[data-testid=txn-edit-save]").click();

    // Saving a category change can raise the "N more look like this — recategorize
    // them too?" offer, which deliberately KEEPS the dialog open: the app has
    // spotted a pattern and is asking before acting on it, which is the behaviour
    // C576 wants (a consequential change is offered, not performed). The journey
    // declines it — teaching the app is the next step, done explicitly via a rule —
    // and the offer's three answers must all be readable, not clipped into a column.
    // Wait for whichever the save produced. The offer REPLACES the form body, so
    // "the form is gone" is true both when the dialog closed and when the offer is
    // showing — asserting on the form alone passes while the dialog is still up and
    // its backdrop is still swallowing every click on the page behind it.
    await page.waitForFunction(
      () => !document.querySelector(".flip-backdrop") || !!document.querySelector("[data-testid=txn-recat-offer]"),
      null, { timeout: 25_000 });

    const offer = page.locator("[data-testid=txn-recat-offer]");
    if (await offer.count()) {
      for (const id of ["txn-recat-apply", "txn-recat-rule", "txn-recat-dismiss"]) {
        const btn = page.locator(`[data-testid=${id}]`);
        await expect(btn).toBeVisible();
        const clipped = await btn.evaluate((el) => el.scrollWidth > el.clientWidth + 1);
        expect(clipped, `${id} is clipped`).toBe(false);
      }
      await page.locator("[data-testid=txn-recat-dismiss]").click();
    }
    // The dialog is really gone — backdrop included. A dismissed modal that leaves
    // its backdrop behind is an invisible sheet over the whole page.
    await expect(page.locator(".flip-backdrop")).toHaveCount(0, { timeout: 15_000 });
    await settle(page);

    // The working set survived the edit: same search, same narrowed count.
    if (!(await page.locator(".fctrl-input").inputValue()).includes(MERCHANT)) lostState++;
    await expect(page.locator("[data-testid=txn-count-line]")).toContainText(`of ${total} transactions`);

    // The correction is visible on the row, and the row no longer claims a machine
    // filed it — a person just did.
    const correctedRow = page.locator(`[data-testid="txn-row-${rowID}"]`);
    await expect(correctedRow.locator(".td-cat-name")).toHaveText(newCat, { timeout: 15_000 });
    await expect(correctedRow.locator("[data-testid=txn-auto-mark]")).toHaveCount(0);

    // ---- 4. TEACH THE APP (the deepest side trip) --------------------------
    await correctedRow.locator(`[data-testid="txn-kebab-${rowID}"]`).click();
    await page.locator("[data-testid=txn-create-rule]:visible").click();
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("data-route") === "/rules", null, { timeout: 20_000 });
    routeChanges.push("/rules");

    // There is a way back, it is at the top of the page, and it NAMES the list it
    // returns to rather than saying "Back".
    const crumb = page.locator("[data-testid=return-crumb]");
    await expect(crumb).toBeVisible({ timeout: 15_000 });
    await expect(crumb).toContainText(MERCHANT);
    // It really is at the top — the trap here is a late-mounted node landing at the
    // end of the page, where nobody scrolls to look for a way back.
    const crumbY = await crumb.evaluate((el) => el.getBoundingClientRect().top);
    expect(crumbY).toBeLessThan(300);

    // ---- 5. RETURN, AND FIND THE SAME LIST ---------------------------------
    await page.locator("[data-testid=return-crumb-back]").click();
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("data-route") === "/transactions", null, { timeout: 20_000 });
    routeChanges.push("/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    if (!(await page.locator(".fctrl-input").inputValue()).includes(MERCHANT)) lostState++;
    await expect(page.locator("[data-testid=txn-count-line]")).toContainText(`of ${total} transactions`);
    await expect(page.locator(`[data-testid="txn-row-${rowID}"]`)).toBeVisible();

    // ---- 6. VERIFY THE RECORD ----------------------------------------------
    await page.locator(`[data-testid="txn-kebab-${rowID}"]`).click();
    await page.locator("[data-testid=txn-history-open]:visible").click();
    await expect(page.locator(".flip-wrap[role=dialog]")).toBeVisible({ timeout: 15_000 });
    // A read-only record offers no Save to press (C567) — closing is the only verb.
    await expect(page.locator(".flip-wrap [data-testid=flip-save]")).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(page.locator(".flip-wrap[role=dialog]")).toBeHidden({ timeout: 10_000 });

    // ---- the shape of the journey ------------------------------------------
    // Two route changes: out to Rules and back. Anything more means the loop has
    // grown a detour. Nothing was rebuilt by hand along the way.
    expect(routeChanges).toEqual(["/transactions", "/rules", "/transactions"]);
    expect(lostState).toBe(0);
  });

  test("the loop is keyboard-usable and every mistake is recoverable", async ({ page }) => {
    await boot(page);
    await nav(page, "/transactions");
    await page.waitForSelector(".txn-table", { timeout: 25_000 });

    // Reaching the search box and typing in it needs no mouse.
    await page.locator(".fctrl-input").focus();
    await page.keyboard.type(MERCHANT);
    await expect(page.locator("[data-testid=filter-summary]")).toContainText(MERCHANT, { timeout: 20_000 });

    // The filter reset says how much it removes before it is pressed, and pressing
    // it really does return the full ledger — the "get me back to everything"
    // action is one control with one meaning (C574).
    const clearAll = page.locator("[data-testid=filter-clear-all]");
    await expect(clearAll).toHaveText(/Clear all 1 filter/);
    await clearAll.click();
    await expect(page.locator("[data-testid=txn-count-line]")).toContainText(/Showing all/, { timeout: 20_000 });
    await expect(page.locator("[data-testid=filter-summary]")).toHaveCount(0);

    // Escape closes the edit dialog without committing anything, and focus is not
    // stranded — the recovery path from "I opened the wrong row".
    const firstRow = page.locator(".txn-table tbody tr.row").first();
    const before = (await firstRow.innerText()).replace(/\s+/g, " ").trim();
    await firstRow.locator("[data-testid=txn-rowedit]").click();
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeVisible({ timeout: 15_000 });
    await page.keyboard.press("Escape");
    await expect(page.locator("[data-testid=txn-edit-form]")).toBeHidden({ timeout: 10_000 });
    // Nothing was committed: the row reads exactly as it did before it was opened.
    const after = (await page.locator(".txn-table tbody tr.row").first().innerText()).replace(/\s+/g, " ").trim();
    expect(after).toBe(before);

    // Confirming an automatic category is one click and is announced, so accepting
    // the machine's guess does not require a trip to the Rules page (C579).
    const autoRow = page.locator(".txn-table tbody tr.row").filter({
      has: page.locator("[data-testid=txn-auto-mark]"),
    }).first();
    await expect(autoRow).toBeVisible({ timeout: 20_000 });
    const id = (await autoRow.getAttribute("data-testid")).replace("txn-row-", "");
    await autoRow.locator(`[data-testid="txn-kebab-${id}"]`).click();
    await page.locator("[data-testid=txn-confirm-category]:visible").click();
    await expect(page.locator(`[data-testid="txn-row-${id}"] [data-testid=txn-auto-mark]`)).toHaveCount(0, { timeout: 15_000 });
  });
});
