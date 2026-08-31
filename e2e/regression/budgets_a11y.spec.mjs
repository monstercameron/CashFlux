// budgets_a11y.spec.mjs — the accessibility contract of the /budgets surface.
//
// A note on why this is an e2e test and not a browser-console check. Focus
// events are NOT dispatched by a document that does not hold window focus: a
// synthetic .focus() there moves activeElement and fires nothing. Measuring the
// keyboard path in a background pane therefore reports "focus does nothing"
// whether the code works or not, which is exactly the false negative that sent
// this investigation the wrong way twice (2026-08-31). Playwright drives a
// focused page, so it is the only place these assertions mean anything.
import { test, expect } from "@playwright/test";
import { boot, nav } from "./fixtures.mjs";

async function openBudgets(page) {
  await nav(page, "/budgets");
  await expect(page.getByTestId("budgets-status-strip").or(page.locator(".bflex-hero")))
    .toBeVisible({ timeout: 45_000 });
}

const MARKER_BTN = ".budget-marker-wrap button:visible, .budget-crow-notes-wrap button:visible";

// The markers live on the COMPACT row, not the card, so every test here has to
// switch density first. Without this the locators matched nothing and all three
// marker tests skipped themselves green — a guard that always skips is not a
// guard (2026-08-31). The toggle names its destination, so data-density is what
// reports the current view.
async function compactMode(page) {
  const density = page.getByTestId("budgets-density");
  await density.scrollIntoViewIfNeeded();
  if ((await density.getAttribute("data-density")) !== "compact") await density.click();
  await expect(page.locator(".budget-crow").first()).toBeVisible({ timeout: 20_000 });
}

test.describe("budgets · accessibility", () => {
  test("keyboard focus previews a marker the same way hover does, and leaving closes it", async ({ page }) => {
    await boot(page);
    await openBudgets(page);

    await compactMode(page);
    // Asserted, not skipped: the seeded household has rolling-over, goal-funded and
    // note-bearing rows, so zero markers means they stopped rendering.
    const marker = page.locator(MARKER_BTN).first();
    await expect(marker, "the compact rows carry no markers at all").toBeVisible({ timeout: 20_000 });
    await marker.scrollIntoViewIfNeeded();

    const pop = page.getByTestId("smart-tip-pop");
    await expect(pop).toHaveCount(0);

    await marker.focus();
    // The popover is deliberately delayed by hover intent, so this waits rather
    // than asserting immediately — an instant assertion here would fail on
    // correct code.
    await expect(pop).toHaveCount(1, { timeout: 5_000 });
    await expect(marker).toHaveAttribute("aria-expanded", "true");

    // Leaving the marker closes it. Focus goes to something outside the wrapper.
    await page.evaluate(() => {
      const away = [...document.querySelectorAll("a[href], button")]
        .find((b) => b.offsetParent && !b.closest(".budget-marker-wrap"));
      away?.focus();
    });
    await expect(pop).toHaveCount(0, { timeout: 5_000 });
  });

  test("moving between a marker's two shapes does not flicker its popover shut", async ({ page }) => {
    // The pill and the glyph are one marker with one open state. Binding focus on
    // each button instead of on the wrapper would make a move between them read as
    // a blur, closing the popover the move was meant to keep open.
    await boot(page);
    await openBudgets(page);

    await compactMode(page);
    const wrap = page.locator(".budget-marker-wrap").first();
    await expect(wrap).toBeAttached({ timeout: 20_000 });
    const btns = wrap.locator("button");
    // A pill AND a glyph is the two-shape case this test exists for. Goal markers
    // are glyph-only by design, so a one-shape marker is a real skip, not a silent
    // one — but the FIRST wrapper having one shape would mean the pill stopped
    // rendering, so the count is reported when it skips.
    const shapes = await btns.count();
    test.skip(shapes < 2, `first marker has ${shapes} shape(s); needs the pill+glyph pair`);

    await btns.first().focus();
    const pop = page.getByTestId("smart-tip-pop");
    await expect(pop).toHaveCount(1, { timeout: 5_000 });
    await btns.nth(1).focus();
    // Give a wrongly-scoped listener time to close it before asserting it did not.
    await page.waitForTimeout(400);
    await expect(pop, "focus moved within one marker and closed its popover").toHaveCount(1);
  });

  test("the pointer can travel from a marker onto its popover without it vanishing", async ({ page }) => {
    // WCAG 2.2 SC 1.4.13 (Content on Hover or Focus), the HOVERABLE clause. The
    // popover used to close on mouseleave, so the pointer could never reach the
    // text — which on a rollover marker is the arithmetic behind the period's cap,
    // not a decorative label. Anyone using magnification, or simply reading slowly,
    // lost it on the way. Leaving now arms a close that the popover itself cancels.
    await boot(page);
    await openBudgets(page);
    await compactMode(page);

    const marker = page.locator(MARKER_BTN).first();
    await expect(marker).toBeVisible({ timeout: 20_000 });
    await marker.scrollIntoViewIfNeeded();

    const pop = page.getByTestId("smart-tip-pop");
    // The popover opens on a hover-INTENT delay, and hover only arms it when the
    // pointer actually moves onto the marker — a pointer already resting there
    // dispatches no mouseenter, and the preceding step can leave it anywhere. So
    // the pointer is parked away first and the pair is retried, rather than hovering
    // once and trusting it landed (this flaked in a multi-spec run, 2026-08-31).
    await expect(async () => {
      await page.mouse.move(5, 5);
      await marker.hover();
      await expect(pop).toBeVisible({ timeout: 2_000 });
    }).toPass({ timeout: 20_000 });

    await pop.hover();
    // Long enough for a close armed by the departure from the marker to have run.
    await page.waitForTimeout(600);
    await expect(pop, "the popover closed while the pointer was on it").toBeVisible();

    // ...and it still closes on a real departure. A grace window that never
    // expires is a popover you cannot get rid of, which is the worse bug.
    await page.mouse.move(5, 5);
    await expect(pop).toBeHidden({ timeout: 5_000 });
  });

  test("a marker is named by what it is, and described by what it means", async ({ page }) => {
    // The glyph used to take the whole sentence as its aria-label. With a
    // described-by node carrying the same sentence that makes a screen reader read
    // the explanation twice before ever saying what the control is.
    await boot(page);
    await openBudgets(page);

    await compactMode(page);
    const markers = page.locator(".budget-marker-wrap button");
    await expect(markers.first()).toBeAttached({ timeout: 20_000 });
    const n = await markers.count();

    for (let i = 0; i < n; i++) {
      const btn = markers.nth(i);
      const descID = await btn.getAttribute("aria-describedby");
      expect(descID, `marker ${i} has no description`).toBeTruthy();
      const desc = (await page.locator(`#${descID}`).innerText()).trim();
      expect(desc.length, `marker ${i}'s description is empty`).toBeGreaterThan(0);

      const name = ((await btn.getAttribute("aria-label")) || (await btn.innerText())).trim();
      expect(name.length, `marker ${i} has no accessible name`).toBeGreaterThan(0);
      expect(name, `marker ${i} is named with its whole description`).not.toBe(desc);
      // A name is a label, not a paragraph.
      expect(name.length, `marker ${i}'s name is a sentence: ${name}`).toBeLessThan(60);
    }
  });

  test("every interactive control on the surface has an accessible name", async ({ page }) => {
    await boot(page);
    await openBudgets(page);

    const unnamed = await page.evaluate(() => {
      const out = [];
      const sel = "button, a[href], input:not([type=hidden]), select, textarea, [role=button]";
      for (const el of document.querySelectorAll(sel)) {
        if (el.closest("[aria-hidden=true]") || el.getAttribute("aria-hidden") === "true") continue;
        if (!el.offsetParent && getComputedStyle(el).position !== "fixed") continue;
        const labelled = el.getAttribute("aria-labelledby");
        const byRef = labelled
          ? labelled.split(/\s+/).map((id) => document.getElementById(id)?.textContent || "").join(" ")
          : "";
        const name = [
          el.getAttribute("aria-label"), byRef, el.innerText,
          el.getAttribute("title"), el.getAttribute("placeholder"),
          el.labels?.[0]?.textContent,
        ].map((s) => (s || "").trim()).find(Boolean);
        if (!name) out.push(el.className || el.tagName);
      }
      return out;
    });
    expect(unnamed, `unnamed controls: ${unnamed.join(" | ")}`).toEqual([]);
  });

  test("each collapsible section reports whether it is open", async ({ page }) => {
    // The folds are the only way to reach recurring, the year planner and savings.
    // A toggle that does not report its state leaves a screen-reader user unable to
    // tell a collapsed section from an empty one.
    await boot(page);
    await openBudgets(page);

    const folds = page.locator(".budget-fold-toggle:visible, .budget-annualgrid-toggle:visible");
    await expect(folds.first()).toBeVisible({ timeout: 20_000 });
    const n = await folds.count();

    for (let i = 0; i < n; i++) {
      const t = folds.nth(i);
      const before = await t.getAttribute("aria-expanded");
      expect(["true", "false"], `fold ${i} does not report a state`).toContain(before);
      const want = before === "true" ? "false" : "true";
      await t.scrollIntoViewIfNeeded();
      // The click is retried rather than asserted once. Opening a fold above this
      // one reflows the page mid-click, so a click can land on nothing while the
      // section is still settling — that is a moving target, not a broken toggle,
      // and asserting once turns it into a flake (seen 2026-08-31).
      await expect(async () => {
        if ((await t.getAttribute("aria-expanded")) === before) await t.click();
        await expect(t).toHaveAttribute("aria-expanded", want, { timeout: 2_000 });
      }, `fold ${i} did not flip aria-expanded`).toPass({ timeout: 20_000 });
    }
  });
});
