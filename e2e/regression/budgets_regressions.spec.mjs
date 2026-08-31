// budgets_regressions.spec.mjs — guards for defects fixed on 2026-08-31.
//
// Every test here corresponds to a bug that SHIPPED and was found by looking at
// the page rather than by reading the code. They are grouped by what made each
// one invisible, because that is the property a guard has to preserve:
//
//   ABSENCE      something stopped being rendered and nothing failed.
//   DISAGREEMENT two places computed the same fact and drifted apart.
//   DEAD FLAG    a field existed, was read, and was never written.
//   WRONG WORD   the number was right and the label described something else.
//
// A test that only re-checks arithmetic would not have caught any of them.
import { test, expect } from "@playwright/test";
import { boot, nav, openVia } from "./fixtures.mjs";

const SETTINGS_MENU = ".bud-set-menu:not(.hidden-menu)";

async function openBudgets(page) {
  await nav(page, "/budgets");
  await expect(page.getByTestId("budgets-status-strip").or(page.locator(".bflex-hero")))
    .toBeVisible({ timeout: 45_000 });
}

async function setMethod(page, value) {
  await page.getByTestId("budgets-settings").scrollIntoViewIfNeeded();
  await openVia(page, page.getByTestId("budgets-settings"), page.locator(SETTINGS_MENU));
  await page.locator(SETTINGS_MENU).getByTestId("budgets-method").selectOption(value);
  // Wait on the SURFACE, not on the toast. "household method is now…" is already
  // on screen from the previous switch, so a text check passes instantly and the
  // next step runs against the old method — which is what made switching race
  // (2026-08-31). data-method is what the page is actually rendering.
  await expect(page.locator(`.bento-budgets[data-method="${value}"]`))
    .toBeVisible({ timeout: 20_000 });
  await page.keyboard.press("Escape");
  await expect(page.locator(SETTINGS_MENU)).toHaveCount(0);
}

test.describe("budgets · regressions (2026-08-31)", () => {
  // ── DISAGREEMENT ──────────────────────────────────────────────────────────
  test("the rail's resolve figure equals the allocation bar's, in every method", async ({ page }) => {
    // overAssigned was a switch covering zero-based and simple only, so envelope
    // and flex printed "X more than you earn" with no way to act on it; and
    // simple's own formula omitted rollover and savings, so it could disagree with
    // the bar directly above it. Both now read one expression. Comparing the two
    // NUMBERS is the guard — a method-by-method presence check would pass even if
    // the figures drifted.
    await boot(page);
    await openBudgets(page);

    for (const method of ["simple", "zero-based", "envelope"]) {
      await setMethod(page, method);
      const relation = page.getByTestId("budgets-alloc-relation");
      if ((await relation.count()) === 0) continue;
      const text = await relation.innerText();
      const overOnBar = text.match(/\$([\d,]+\.\d\d)\s+more than/i);
      if (!overOnBar) continue; // this method's plan fits; nothing to resolve

      const resolve = page.getByTestId("budgets-rail-resolve");
      await expect(resolve, `${method}: bar says over income, rail offers nothing`)
        .toHaveCount(1);
      const onRail = (await resolve.innerText()).match(/\$([\d,]+\.\d\d)/);
      expect(onRail?.[1], `${method}: rail and bar disagree`).toBe(overOnBar[1]);
    }
  });

  // ── ABSENCE ───────────────────────────────────────────────────────────────
  test("zero-based states its savings slot even when nothing is assigned", async ({ page }) => {
    // The legend was gated on Savings > 0, so with nothing set there was no
    // segment, no legend row, and the only trace was a collapsed tile at the
    // bottom of the page. The concept only existed for people who had already
    // found it. An empty slot is information; an omitted one is not.
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "zero-based");

    const legend = page.locator(".zbb-legend");
    await expect(legend).toBeVisible();
    // Named by DIRECTION — a limit is a ceiling, a target is a floor.
    await expect(legend).toContainText(/spending limits/i);
    await expect(legend).toContainText(/savings targets/i);
  });

  test("the collapsed sections carry figures, not slogans", async ({ page }) => {
    // Recurring was 1000px of always-open rows; Plan the year's hint read "See the
    // whole year and plan ahead", which described the section instead of telling
    // you anything. A collapsed row that answers nothing is just a thicker border.
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "zero-based");

    const hints = page.locator(".budget-fold-toggle-hint, .budget-annualgrid-toggle-hint");
    await expect(hints.first()).toBeVisible();
    const all = (await hints.allInnerTexts()).join(" | ");

    // The defect was a hint that DESCRIBED its section — "See the whole year and
    // plan ahead" — which a collapsed row already implies and which answers nothing.
    // Two properties pin that, and neither demands a digit from every hint: the
    // savings fold with nothing assigned reads "Set a monthly amount per account",
    // which is an empty state doing its job, and an earlier version of this test
    // failed it for having no number in it.
    //
    // Matching an imperative was the second attempt and also wrong — the string
    // arrives with a character trim() does not remove, so anchoring on ^set failed
    // against text that plainly starts with "Set". Asserting on what the hints
    // COLLECTIVELY carry avoids depending on the exact bytes of any one of them.
    const hintTexts = await hints.allInnerTexts();
    const withFigures = hintTexts.filter((h) => /\d/.test(h)).length;
    expect(withFigures, `no collapsed section reports a figure: ${all}`).toBeGreaterThan(0);

    // And no hint may go back to restating its own section. These are the shapes
    // that were there before: a description of what the section is for.
    for (const hint of hintTexts) {
      expect(hint, `hint describes its section instead of reporting: ${all}`)
        .not.toMatch(/see the whole year|plan ahead|see your|view your/i);
    }
  });

  // ── DEAD FLAG ─────────────────────────────────────────────────────────────
  test("a recurring cover arrangement marks the row it funds", async ({ page }) => {
    // CoveredAt was only written by appstate.CoverBudget, which the cover-all
    // control does not call — it writes period boosts directly — so the flag never
    // lit from the one place people use, and the CARD's badge was dead in that
    // path too. This checks the standing-arrangement half, which the marker spec's
    // one-time case does not reach.
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "zero-based");

    const markers = page.locator("[data-testid^=budget-glyph-cover-],[data-testid^=budget-marker-pill-cover-]");
    const coverAll = page.getByTestId("budgets-cover-all");
    test.skip((await coverAll.count()) === 0, "nothing is over budget in the seeded period");

    await coverAll.click();
    const source = page.locator("[data-testid^=cover-all-src-]").first();
    await expect(source).toBeVisible({ timeout: 20_000 });
    const value = await source.evaluate((s) => {
      const opt = [...s.options].find((o) => o.value && !o.value.startsWith("__"));
      return opt ? opt.value : "";
    });
    test.skip(!value, "no budget has slack to cover from");
    await source.selectOption(value);
    await page.getByTestId("cover-all-apply").click();
    await expect(markers.first()).toBeVisible({ timeout: 20_000 });

    // And it SURVIVES a reload — the flag is persisted state, not a render-time
    // artifact of the click that produced it.
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForFunction(
      () => document.documentElement.getAttribute("data-app-ready") === "true",
      null, { timeout: 45_000 },
    );
    await expect(markers.first()).toBeVisible({ timeout: 30_000 });
  });

  // ── WRONG WORD ────────────────────────────────────────────────────────────
  test("the density toggle names where a click goes, and says where you are", async ({ page }) => {
    // The label was frozen at "Compact list" because a moving label beside
    // aria-pressed announced "Card view, pressed" over a compact list. The label
    // now names the DESTINATION and aria-pressed is gone; the current view moved
    // to data-density and to the accessible-name suffix. The two must never name
    // the same view, which is the contradiction returning.
    await boot(page);
    await openBudgets(page);

    const toggle = page.getByTestId("budgets-density");
    await toggle.scrollIntoViewIfNeeded();
    // aria-pressed cannot be true of a button named for its destination.
    await expect(toggle).not.toHaveAttribute("aria-pressed", /.+/);

    const before = await toggle.getAttribute("data-density");
    const labelBefore = (await toggle.innerText()).toLowerCase();
    // Showing the compact list ⇒ the button offers cards, and vice versa.
    if (before === "compact") expect(labelBefore).toMatch(/full cards/);
    else expect(labelBefore).toMatch(/compact list/);

    await toggle.click();
    await expect(toggle).not.toHaveAttribute("data-density", before ?? "");
    const labelAfter = (await toggle.innerText()).toLowerCase();
    expect(labelAfter, "the label did not follow the view").not.toBe(labelBefore);
  });

  test("a live period is never described in the past tense", async ({ page }) => {
    // The month-end review is offered in a period's LAST FIVE DAYS as well as
    // after it closes, and every line read "went over" / "went unspent" — money
    // not yet spent, described as money left over. Guarded here as well as in the
    // month-close spec because the tense flows from copy that any string edit can
    // silently revert.
    await boot(page);
    await openBudgets(page);

    const offer = page.getByTestId("budgets-monthclose-offer");
    test.skip((await offer.count()) === 0, "the review is not offered in this period");
    await offer.click();

    const dialog = page.locator("[role=dialog]").filter({ hasText: /Review / });
    await expect(dialog).toBeVisible({ timeout: 20_000 });
    const text = await dialog.innerText();
    expect(text).not.toMatch(/went over|went unspent|ended over budget/i);
    // And step 5 carries FORWARD: it used to write the previous period's top-ups
    // into the period being reviewed, which could never land usefully.
    expect(text).toMatch(/next period/i);
    expect(text).not.toMatch(/last period's top-ups/i);
  });
});
