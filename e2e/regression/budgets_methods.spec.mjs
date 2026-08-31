// budgets_methods.spec.mjs — every budgeting method renders its own surface
// (2026-08-31).
//
// The /budgets surface is four different pages behind one route. Simple and
// envelope share the loader band and the budget list; zero-based replaces the
// band's hero with To Assign and adds a savings tile; flex throws the list away
// entirely for one pooled number plus fixed commitments.
//
// Everything before this file exercised ONE of them. The work that reshaped the
// row, the markers and the collapsible sections was verified in zero-based and
// simple only, so this walks all four and checks the shape each is supposed to
// have — plus the invariants that must hold in every one of them:
//
//   - no row overflows or scrolls inside itself at any width
//   - the name cell still clips, so a marker cannot escape it
//   - the hero's labels describe the figures actually shown
import { test, expect } from "@playwright/test";
import { boot, nav, openVia } from "./fixtures.mjs";

const SETTINGS_MENU = ".bud-set-menu:not(.hidden-menu)";

async function openBudgets(page) {
  await nav(page, "/budgets");
  await expect(page.getByTestId("budgets-status-strip").or(page.locator(".bflex-hero")))
    .toBeVisible({ timeout: 45_000 });
}

// setMethod drives the household method picker the way a person does.
//
// It uses the same openVia + Escape-and-WAIT pattern the zero-based journey
// proved, rather than a hand-rolled click. The first draft here located the
// trigger by its label text and closed the popover without waiting: the menu's
// backdrop covers the page while open, so the next test's click landed on a
// transparent div and four tests failed for a reason that had nothing to do with
// what they were testing (2026-08-31).
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

// rowInvariants are the things that must be true of the compact list in EVERY
// method that has one. They are checked per width because every layout bug found
// in this surface appeared at one width and not another.
async function rowInvariants(page, label) {
  for (const width of [1440, 1100, 900]) {
    await page.setViewportSize({ width, height: 900 });
    await page.waitForTimeout(250);
    const problems = await page.evaluate(() => {
      const out = [];
      document.querySelectorAll(".budget-crow").forEach((r) => {
        if (r.scrollWidth > r.clientWidth + 1) {
          out.push({ row: r.textContent.slice(0, 14), why: "row scrolls inside itself" });
        }
        const head = r.querySelector(".budget-crow-head");
        if (head && getComputedStyle(head).overflow !== "hidden") {
          out.push({ row: r.textContent.slice(0, 14), why: "name cell no longer clips" });
        }
        const bar = r.querySelector(".budget-crow-bar");
        if (head && bar) {
          const hb = head.getBoundingClientRect();
          const bb = bar.getBoundingClientRect();
          // A hidden bar reports an all-zero rect; only compare against a laid-out one.
          if (bb.width > 0 && hb.right > bb.left + 0.5) {
            out.push({ row: r.textContent.slice(0, 14), why: "name cell overlaps the bar" });
          }
        }
      });
      return out;
    });
    expect(problems, `${label} at ${width}px`).toEqual([]);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
}

test.describe("budgets · every method renders its own surface", () => {
  test("simple leads with what was budgeted, not with income", async ({ page }) => {
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "simple");

    const figs = page.locator(".budget-loader-fig");
    await expect(figs).toHaveCount(3);
    // The band's middle figure is the PLAN in simple — the labels and the numbers
    // have to agree, which is the pairing zero-based broke by swapping in income.
    await expect(figs.nth(0)).toContainText(/spent/i);
    await expect(figs.nth(1)).toContainText(/budgeted/i);
    await expect(figs.nth(2)).toContainText(/left/i);
    // Simple has no savings tile — that belongs to zero-based alone.
    await expect(page.locator('[data-widget="budget-savings"]')).toHaveCount(0);
    await rowInvariants(page, "simple");
  });

  test("zero-based leads with To Assign and adds the savings tile", async ({ page }) => {
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "zero-based");

    // The hero is the method's one governing number, named by its state.
    const hero = page.getByTestId("budgets-hero-left");
    await expect(hero).toBeVisible();
    await expect(page.getByTestId("budgets-status-strip"))
      .toContainText(/to assign|over-assigned|all assigned/i);
    // Spending rides the allocation bar's axis here rather than a band of its own.
    await expect(page.getByTestId("budgets-spend-rail")).toBeVisible();
    await expect(page.locator('[data-widget="budget-savings"]')).toHaveCount(1);
    await rowInvariants(page, "zero-based");
  });

  test("envelope keeps the band and the list", async ({ page }) => {
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "envelope");

    await expect(page.locator(".budget-loader-fig")).toHaveCount(3);
    await expect(page.locator(".budget-crow").first()).toBeVisible();
    // Envelope is not zero-based: no To Assign hero, no savings tile.
    await expect(page.getByTestId("budgets-spend-rail")).toHaveCount(0);
    await expect(page.locator('[data-widget="budget-savings"]')).toHaveCount(0);
    await rowInvariants(page, "envelope");
  });

  test("flex replaces the list with one pooled number", async ({ page }) => {
    await boot(page);
    await openBudgets(page);
    await setMethod(page, "flex");

    // The whole surface changes: no per-budget rows, no summary band.
    await expect(page.locator(".budget-crow")).toHaveCount(0);
    await expect(page.locator(".budget-loader-fig")).toHaveCount(0);
    // What it puts there instead: the flex number, and the fixed commitments that
    // are deliberately NOT part of the day-to-day pool.
    await expect(page.locator("body")).toContainText(/day-to-day/i);
    await expect(page.locator("body")).toContainText(/fixed commitments/i);
    // The method picker has to remain reachable, or flex is a one-way door.
    await expect(page.getByTestId("budgets-settings")).toBeVisible();
  });

  test("an over-income plan offers the same way out in every method that can be over", async ({ page }) => {
    // The allocation bar reports "X more than you earn" on every method. The rail's
    // Resolve action used to be built from an overAssigned figure computed for
    // zero-based and simple ONLY, so envelope stated the problem and offered nothing
    // to do about it. Both now read the SAME expression the bar does, and this test
    // is what stops the two drifting apart again (2026-08-31).
    await boot(page);
    await openBudgets(page);

    const overIncome = async () =>
      (await page.getByTestId("budgets-alloc-relation").count()) > 0 &&
      /more than (you earn|was earned)/i.test(await page.getByTestId("budgets-alloc-relation").innerText());

    await setMethod(page, "simple");
    test.skip(!(await overIncome()), "seeded plan is not over income");
    await expect(page.getByTestId("budgets-rail-resolve")).toHaveCount(1);

    await setMethod(page, "envelope");
    expect(await overIncome(), "envelope still reports being over income").toBe(true);
    // Envelope used to state the problem and offer nothing: overAssigned was a
    // switch covering zero-based and simple only, so the rail had nothing to act
    // on. Every method now derives it from the SAME expression the allocation bar
    // uses, so a surface can no longer report being over income while withholding
    // the way out (2026-08-31).
    await expect(page.getByTestId("budgets-rail-resolve")).toHaveCount(1);
  });
});
