// rail_search.spec.mjs — the sidebar's destination filter.
//
// The rail holds around thirty destinations across four groups, and the tools,
// system and my-pages sections all default to COLLAPSED. So the property worth
// testing is not "typing narrows a list" — it is that a destination behind a
// closed section becomes reachable WITHOUT opening it. A test that only filtered
// the nine always-visible primary items would pass while the feature did nothing
// for the twenty that needed it.
import { test, expect } from "@playwright/test";
import { boot, nav } from "./fixtures.mjs";

const BOX = "railsearch-input";
const RESULTS = ".rail-nav a";

async function openApp(page) {
  await boot(page);
  await nav(page, "/budgets");
  await expect(page.getByTestId(BOX)).toBeVisible({ timeout: 45_000 });
}

// typeQuery types for real. locator.fill() sets the DOM value and dispatches an
// input event, and the field then READS "bud" while the rail stays unfiltered:
// the framework's OnInput never runs, so the Go state behind the list never
// changes. Asserting on the input's value would have passed against a feature
// doing nothing. pressSequentially produces the key events the handler needs.
// (Measured 2026-08-31: fill → 13 unfiltered links; pressSequentially → 1.)
async function typeQuery(page, q) {
  const box = page.getByTestId(BOX);
  await box.click();
  await box.press("ControlOrMeta+a");
  await box.press("Backspace");
  if (q !== "") await box.pressSequentially(q, { delay: 30 });
  await expect(box).toHaveValue(q);
}

async function resultLabels(page) {
  return page.evaluate((sel) =>
    [...document.querySelectorAll(sel)].map((a) => a.innerText.trim().split("\n")[0]),
    RESULTS);
}

test.describe("sidebar · destination filter", () => {
  test("a destination inside a collapsed section is reachable without opening it", async ({ page }) => {
    await openApp(page);

    // Precondition, asserted rather than assumed: the section really is closed, so
    // the match below cannot be coming from an already-expanded rail.
    const collapsedHeaders = await page.locator('.rail-nav [aria-expanded="false"]').count();
    expect(collapsedHeaders, "no collapsed sections — this test proves nothing").toBeGreaterThan(0);

    await typeQuery(page, "worth");
    const hits = await resultLabels(page);
    expect(hits, `"worth" did not reach Net worth: ${hits.join(", ")}`).toContain("Net worth");

    // And it is a real link that navigates.
    await page.locator(RESULTS).first().click();
    await expect(page.locator("#main")).toHaveAttribute("data-route", /networth|net-worth/, { timeout: 20_000 });
  });

  test("results are ranked, so a short query puts the obvious answer first", async ({ page }) => {
    // In menu order "bud" hits the annual budget grid before Budgets itself, which
    // makes the reader scan past what they asked for. Prefix beats substring.
    await openApp(page);
    await typeQuery(page, "bud");
    const hits = await resultLabels(page);
    expect(hits.length, "no matches for \"bud\"").toBeGreaterThan(0);
    expect(hits[0]).toBe("Budgets");
  });

  test("Enter goes to the top match and leaves the rail as it was", async ({ page }) => {
    await openApp(page);
    await typeQuery(page, "goal");
    await page.getByTestId(BOX).press("Enter");

    await expect(page.locator("#main")).toHaveAttribute("data-route", "/goals", { timeout: 20_000 });
    // The filter clears itself on navigation: leaving a stale query behind would
    // show a filtered rail on a page the user has already arrived at.
    await expect(page.getByTestId(BOX)).toHaveValue("");
    await expect(page.locator('.rail-nav [aria-expanded]').first()).toBeVisible();
  });

  test("Escape and the clear button both restore the whole menu", async ({ page }) => {
    await openApp(page);
    const full = (await resultLabels(page)).length;
    expect(full).toBeGreaterThan(5);

    await typeQuery(page, "bud");
    expect((await resultLabels(page)).length).toBeLessThan(full);
    await page.getByTestId(BOX).press("Escape");
    await expect(page.getByTestId(BOX)).toHaveValue("");
    expect(await resultLabels(page)).toHaveLength(full);

    await typeQuery(page, "bud");
    await page.getByTestId("railsearch-clear").click();
    await expect(page.getByTestId(BOX)).toHaveValue("");
    expect(await resultLabels(page)).toHaveLength(full);
  });

  test("a query that matches nothing says so, and says what to do", async ({ page }) => {
    await openApp(page);
    await typeQuery(page, "zzzqqq");
    const empty = page.getByTestId("railsearch-empty");
    await expect(empty).toBeVisible();
    await expect(empty).toContainText("zzzqqq");
    expect(await resultLabels(page)).toHaveLength(0);
  });

  test("the filter announces its result count, and hides with the collapsed rail", async ({ page }) => {
    // The list silently changing length tells a screen-reader user nothing, so the
    // count is a live region. And the collapsed rail is 58px of icons — a text
    // field there would be unreadable, so it is not rendered at all.
    await openApp(page);
    await typeQuery(page, "bud");
    const count = page.getByTestId("railsearch-count");
    await expect(count).toHaveAttribute("role", "status");
    await expect(count).toContainText(/\d+ destination/);

    await page.getByTestId("rail-collapse-btn").click();
    await expect(page.locator(".rail.collapsed")).toBeVisible({ timeout: 10_000 });
    await expect(page.getByTestId(BOX)).toHaveCount(0);

    await page.getByTestId("rail-collapse-btn").click();
    await expect(page.getByTestId(BOX)).toBeVisible({ timeout: 10_000 });
  });
});
