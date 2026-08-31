// budgets_markers.spec.mjs — the compact row's markers (2026-08-31).
//
// The row head carries small badges for things a budget IS: rolling over, part
// funded by a goal, covered by another budget. They collapse to glyphs when the
// words stop fitting, and share one hover→popover system.
//
// Four defects are pinned here, and three of them were SILENT — the row rendered,
// the numbers were right, and something was simply missing or drawn in the wrong
// place:
//
//   1. Cover-all wrote period boosts directly and never stamped Budget.CoveredAt,
//      so the "covered" flag never lit from the only control most people use. The
//      card's badge had been dead in that path; the new row marker inherited it.
//   2. A marker with no worded shape had its glyph hidden above the switch width,
//      so goal-funded rows reported nothing at all on a wide screen.
//   3. The goal pill is 168px against a ~151px name cell and spilled across the
//      progress bar. It is glyph-only now at every width.
//   4. Three markers on one row used three different hover systems (native title,
//      click-only popover, hover popover + a native title racing it).
import { test, expect } from "@playwright/test";
import { boot, nav } from "./fixtures.mjs";

const ROW = ".budget-crow";

async function openBudgets(page) {
  await nav(page, "/budgets");
  await expect(page.locator(ROW).first()).toBeVisible({ timeout: 45_000 });
}

// compactMode forces the row list rather than cards. The toggle names its
// DESTINATION, so its label cannot report state — data-density does.
async function compactMode(page) {
  const density = page.getByTestId("budgets-density");
  await density.scrollIntoViewIfNeeded();
  if ((await density.getAttribute("data-density")) !== "compact") await density.click();
  await expect(page.locator(ROW).first()).toBeVisible();
}

function rowFor(page, name) {
  return page.locator(ROW).filter({ hasText: name }).first();
}

test.describe("budgets · compact row markers", () => {
  test("a goal-funded row always shows its marker, at any width", async ({ page }) => {
    // Defect 2: the glyph was hidden above the pill/glyph switch width while the
    // pill it deferred to did not exist — so the marker vanished entirely on a
    // wide screen. Both widths are checked because the bug lived in exactly one.
    await boot(page);
    await openBudgets(page);
    await compactMode(page);

    const glyph = page.locator("[data-testid^=budget-glyph-goal-]").first();
    test.skip((await glyph.count()) === 0, "no goal-funded budget in the seeded data");

    await expect(glyph).toBeVisible();
    // The words live in the accessible name, so dropping to a symbol loses nothing.
    await expect(glyph).toHaveAttribute("aria-label", /goal/i);

    await page.setViewportSize({ width: 820, height: 900 });
    await expect(glyph).toBeVisible();
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(glyph).toBeVisible();
  });

  test("no marker paints outside the name cell", async ({ page }) => {
    // Defect 3, and the class of bug behind it. A bounding-box check is not enough:
    // a zero-width button still paints its icon, so this compares PAINTED extents
    // against the cell and against the bar beside it.
    await boot(page);
    await openBudgets(page);
    await compactMode(page);

    for (const width of [1440, 1100, 900]) {
      await page.setViewportSize({ width, height: 900 });
      await page.waitForTimeout(250);
      const spills = await page.evaluate(() => {
        const out = [];
        document.querySelectorAll(".budget-crow").forEach((r) => {
          const head = r.querySelector(".budget-crow-head");
          const bar = r.querySelector(".budget-crow-bar");
          if (!head || !bar) return;
          const hb = head.getBoundingClientRect();
          const bb = bar.getBoundingClientRect();
          // The cell must still CLIP. This is a style assertion, not a paint one,
          // and it is honest about that: it catches the clip being removed, which
          // is what lets a marker escape, but it cannot prove nothing is drawn
          // outside. The overlap check below is the paint half.
          if (getComputedStyle(head).overflow !== "hidden") {
            out.push({ row: r.textContent.slice(0, 14), why: "head no longer clips" });
          }
          // A hidden bar reports an all-zero rect, and EVERY head then "overlaps"
          // it at x=0. Below ~660px of content the row drops to four columns and
          // the bar is display:none, so comparing against it there produced ten
          // failures describing a collision that cannot exist (2026-08-31). Only
          // compare against a bar that is actually laid out.
          if (bb.width > 0 && hb.right > bb.left + 0.5) {
            out.push({ row: r.textContent.slice(0, 14), why: "name cell overlaps the bar" });
          }
        });
        return out;
      });
      expect(spills, `markers spilled at ${width}px`).toEqual([]);
    }
  });

  test("every marker previews on hover through one popover, and only one", async ({ page }) => {
    // Defect 4. Hovering must open exactly ONE popover — never a second from a
    // duplicate system, and never a native tooltip racing it.
    await boot(page);
    await openBudgets(page);
    await compactMode(page);

    const markers = page.locator("[data-testid^=budget-glyph-],[data-testid^=budget-marker-pill-]");
    const n = await markers.count();
    test.skip(n === 0, "no markers in the seeded data");

    for (let i = 0; i < Math.min(n, 4); i++) {
      const m = markers.nth(i);
      if (!(await m.isVisible())) continue;
      const which = (await m.getAttribute("data-testid")) || `marker ${i}`;
      // No native tooltip on a marker: it would open alongside the popover, and it
      // is unreachable from the keyboard and absent on touch.
      await expect(m, which).not.toHaveAttribute("title", /.+/);

      // The list lives in a scrolling pane, so hovering can race the scroll that
      // brings the marker into view: the pointer lands where the marker WAS.
      // Settling the position first is what makes this deterministic rather than
      // usually-true — this assertion flaked repeatedly before the scroll was
      // separated from the hover (2026-08-31).
      await m.scrollIntoViewIfNeeded();
      await expect(m).toBeInViewport();
      await m.hover();
      await expect(page.getByTestId("smart-tip-pop"), `${which} opened no popover`)
        .toHaveCount(1, { timeout: 8_000 });
      // Leave, and it must clean up after itself — including before the next marker
      // is hovered, or a lingering popover could cover it.
      await page.mouse.move(0, 0);
      await expect(page.getByTestId("smart-tip-pop"), `${which} left a popover behind`)
        .toHaveCount(0, { timeout: 8_000 });
    }
  });

  test("covering a budget flags it as covered on the row", async ({ page }) => {
    // Defect 1, end to end: cover an overage through the real control and the
    // receiving row must say so. This failed before the fix — the cover applied,
    // the limit moved, and nothing reported that another budget had paid for it.
    await boot(page);
    await openBudgets(page);
    await compactMode(page);

    const coverAll = page.getByTestId("budgets-cover-all");
    test.skip((await coverAll.count()) === 0, "nothing is over budget in the seeded period");
    await expect(page.locator("[data-testid^=budget-glyph-cover-],[data-testid^=budget-marker-pill-cover-]"))
      .toHaveCount(0);

    await coverAll.click();
    const source = page.locator("[data-testid^=cover-all-src-]").first();
    await expect(source).toBeVisible({ timeout: 20_000 });
    // Pick the first real source (skip "leave uncovered" and "next month").
    const value = await source.evaluate((s) => {
      const opt = [...s.options].find((o) => o.value && !o.value.startsWith("__"));
      return opt ? opt.value : "";
    });
    test.skip(!value, "no budget has slack to cover from");
    await source.selectOption(value);
    await page.getByTestId("cover-all-apply").click();

    // The marker appears on the row that RECEIVED the money.
    const cover = page.locator("[data-testid^=budget-glyph-cover-],[data-testid^=budget-marker-pill-cover-]");
    await expect(cover.first()).toBeVisible({ timeout: 20_000 });

    // And it explains itself rather than just marking the row.
    await cover.first().hover();
    const pop = page.getByTestId("smart-tip-pop");
    await expect(pop).toHaveCount(1, { timeout: 5_000 });
    await expect(pop).toContainText(/covered/i);
  });
});
