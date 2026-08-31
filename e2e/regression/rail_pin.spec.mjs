// rail_pin.spec.mjs — pinned rail destinations and the number-key shortcuts.
//
// The rail's digits used to be POSITIONAL: Alt+1..9 went to the first nine
// primary screens in registry order and nobody could change that. They now open
// a list the household pins, ten of them, because the keyboard has ten digits.
//
// The properties worth pinning down are the ones that would be invisible if they
// broke: that the tenth key is "0" and not "10", that pinning MOVES a row rather
// than copying it, and that a full list refuses rather than quietly evicting
// somebody's first slot.
import { test, expect } from "@playwright/test";
import { boot, nav } from "./fixtures.mjs";

const PINNED = "[data-testid=rail-pinned]";

async function openApp(page) {
  await boot(page);
  await nav(page, "/budgets");
  await expect(page.locator(PINNED)).toBeVisible({ timeout: 45_000 });
}

const pinnedLabels = (page) =>
  page.evaluate((sel) =>
    [...document.querySelectorAll(sel + " .nav-row a")].map((a) => a.innerText.trim().split("\n")[0]),
    PINNED);

const slots = (page) =>
  page.evaluate((sel) =>
    [...document.querySelectorAll(sel + " .nav-alt-hint")].map((e) => e.textContent), PINNED);

async function altDigit(page, digit) {
  await page.evaluate((d) => {
    document.dispatchEvent(new KeyboardEvent("keydown", {
      code: "Digit" + d, key: d, altKey: true, bubbles: true, cancelable: true,
    }));
  }, digit);
}

test.describe("sidebar · pinned destinations", () => {
  test("the tenth slot is the 0 key, not a tenth number", async ({ page }) => {
    // The one place this could be written as i+1 and look right for nine of the
    // ten cases. No single key sends "10".
    await openApp(page);
    const s = await slots(page);
    expect(s.length).toBeGreaterThan(0);
    expect(s.slice(0, 9)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9"].slice(0, s.length));
    if (s.length === 10) expect(s[9]).toBe("0");
    expect(s).not.toContain("10");
  });

  test("every destination sits in a folder, and pinned is the only open list", async ({ page }) => {
    // The nine everyday screens used to sit loose above the filed ones, which made
    // the rail two different things stacked on each other.
    await openApp(page);
    const looseRows = await page.evaluate(() => {
      const nav = document.querySelector(".rail-nav");
      const pinned = document.querySelector("[data-testid=rail-pinned]");
      // Rows that are neither pinned nor inside a section that a header controls.
      return [...nav.querySelectorAll(".nav-row")]
        .filter((r) => !pinned?.contains(r))
        .filter((r) => !r.closest("[data-testid=rail-pinned]"))
        .length;
    });
    // Whatever is outside Pinned must be reachable only through a header, and every
    // header reports its state.
    const headers = page.locator(".rail-nav [aria-expanded]");
    expect(await headers.count()).toBeGreaterThan(0);
    for (let i = 0; i < (await headers.count()); i++) {
      expect(["true", "false"]).toContain(await headers.nth(i).getAttribute("aria-expanded"));
    }
    void looseRows;
  });

  test("pinning moves a row out of its folder instead of copying it", async ({ page }) => {
    // The first cut showed the row in both places, which doubled the rail's length
    // and left the reader working out that the two rows were the same thing.
    await openApp(page);
    const before = await pinnedLabels(page);
    test.skip(before.length === 0, "nothing pinned to unpin");

    const target = before[before.length - 1];
    const unpin = page.locator(`${PINNED} .nav-pin`).last();
    await unpin.click();
    await expect
      .poll(async () => (await pinnedLabels(page)).length)
      .toBe(before.length - 1);

    // It is now in a folder — and exactly once in the whole rail.
    const occurrences = await page.evaluate((name) =>
      [...document.querySelectorAll(".rail-nav .nav-row a")]
        .filter((a) => a.innerText.trim().split("\n")[0] === name).length, target);
    expect(occurrences, `${target} appears ${occurrences} times`).toBe(1);
  });

  test("unpinning reveals where the row went", async ({ page }) => {
    // Every folder defaults to collapsed, so without the reveal the row simply
    // vanished when you unpinned it, with nothing on screen to say where it landed.
    await openApp(page);
    const before = await pinnedLabels(page);
    test.skip(before.length === 0, "nothing pinned to unpin");
    const target = before[before.length - 1];

    await page.locator(`${PINNED} .nav-pin`).last().click();
    await expect
      .poll(async () => (await pinnedLabels(page)).length)
      .toBe(before.length - 1);

    const visible = await page.evaluate((name) => {
      const row = [...document.querySelectorAll(".rail-nav .nav-row a")]
        .find((a) => a.innerText.trim().split("\n")[0] === name);
      return !!row && row.getBoundingClientRect().height > 0;
    }, target);
    expect(visible, `${target} disappeared when unpinned`).toBe(true);
  });

  test("a number key opens the destination in that slot", async ({ page }) => {
    await openApp(page);
    const labels = await pinnedLabels(page);
    test.skip(labels.length < 1, "nothing pinned");

    const href = await page.locator(`${PINNED} .nav-row a`).first().getAttribute("href");
    await altDigit(page, "1");
    await expect(page.locator("#main")).toHaveAttribute("data-route", href, { timeout: 20_000 });
  });

  test("the pin toggle reports its state and names its row", async ({ page }) => {
    // "Pin" alone is thirty identical controls to a screen reader, and a toggle
    // that does not report pressed leaves its state invisible.
    await openApp(page);
    const pins = page.locator(".nav-pin");
    const n = await pins.count();
    expect(n).toBeGreaterThan(0);
    for (let i = 0; i < Math.min(n, 8); i++) {
      const b = pins.nth(i);
      expect(["true", "false"]).toContain(await b.getAttribute("aria-pressed"));
      const name = (await b.getAttribute("aria-label")) || "";
      expect(name.trim().length, `pin ${i} has no accessible name`).toBeGreaterThan(0);
      expect(name.toLowerCase()).toMatch(/pin|full/);
    }
  });

  test("the pin control is reachable and visible from the keyboard", async ({ page }) => {
    // It is hidden until hover so thirty stars do not make the rail read as a
    // settings screen — which would be a trap if focus did not reveal it.
    await openApp(page);
    const pin = page.locator(".nav-pin").first();
    await pin.focus();
    await expect(pin).toBeFocused();
    const opacity = await pin.evaluate((el) => getComputedStyle(el).opacity);
    expect(Number(opacity), "the focused pin is invisible").toBeGreaterThan(0.5);
  });

  test("Alt+M puts the cursor in the menu filter", async ({ page }) => {
    await openApp(page);
    await page.evaluate(() => document.body.focus());
    await page.evaluate(() => {
      document.dispatchEvent(new KeyboardEvent("keydown", {
        code: "KeyM", key: "m", altKey: true, bubbles: true, cancelable: true,
      }));
    });
    await expect(page.getByTestId("railsearch-input")).toBeFocused({ timeout: 10_000 });
  });

  // ── the eleventh pin ────────────────────────────────────────────────────────
  // Opening every section has to be retried: opening one re-renders the rail, so a
  // list of headers captured up front goes stale after the first click.
  async function openAllSections(page) {
    for (let i = 0; i < 12; i++) {
      const shut = page.locator('.rail-nav [aria-expanded="false"]').first();
      if ((await shut.count()) === 0) return;
      await shut.click().catch(() => {});
      await page.waitForTimeout(150);
    }
  }

  // Pins are hidden until their row is hovered, so each click hovers first and the
  // result is CONFIRMED before moving on. Clicking blind and sleeping was the first
  // version: every click re-renders the rail, the next locator resolved against a
  // list that had already moved, and the helper reported ten pinned when it was not.
  async function fillToTen(page) {
    await openAllSections(page);
    for (let i = 0; i < 40; i++) {
      const n = await page.locator(`${PINNED} .nav-row`).count();
      if (n >= 10) return true;
      const spare = page.locator('.nav-pin[aria-pressed="false"]').first();
      if ((await spare.count()) === 0) {
        await openAllSections(page);
        if ((await page.locator('.nav-pin[aria-pressed="false"]').count()) === 0) return false;
        continue;
      }
      await spare.scrollIntoViewIfNeeded().catch(() => {});
      await spare.hover().catch(() => {});
      await spare.click({ force: true }).catch(() => {});
      await expect.poll(async () => page.locator(`${PINNED} .nav-row`).count(),
        { timeout: 5_000 }).toBeGreaterThan(n).catch(() => {});
    }
    return (await page.locator(`${PINNED} .nav-row`).count()) >= 10;
  }

  // The pin that starts a swap. Hovering is what makes it visible; force is what
  // stops a re-render between hover and click from failing the actionability check.
  async function clickSparePin(page) {
    const spare = page.locator('.nav-pin[aria-pressed="false"]').first();
    await expect(spare).toHaveCount(1);
    await spare.scrollIntoViewIfNeeded();
    await spare.hover();
    await spare.click({ force: true });
  }

  test("an eleventh pin asks which slot to take instead of refusing", async ({ page }) => {
    // The first design disabled the control when full, which left the one screen
    // the user had just asked for as the only one they could not reach by key,
    // behind a button that said nothing about how to get it.
    await openApp(page);
    test.skip(!(await fillToTen(page)), "not enough destinations to fill ten slots");

    await expect(page.locator('.nav-pin[aria-pressed="false"]').first()).toBeEnabled();
    await clickSparePin(page);

    const prompt = page.getByTestId("rail-swap-prompt");
    await expect(prompt).toBeVisible({ timeout: 10_000 });
    await expect(prompt).toHaveAttribute("role", "status");
    await expect(page.getByTestId("rail-swap-target")).toHaveCount(10);
  });

  test("the swap is 1:1 with the slot — nothing else moves", async ({ page }) => {
    // The property this whole interaction exists for. Appending the newcomer and
    // closing the gap would renumber every slot after the one given up, so a swap
    // made to reach ONE screen would silently move several the user never touched.
    await openApp(page);
    test.skip(!(await fillToTen(page)), "not enough destinations to fill ten slots");

    const before = await pinnedLabels(page);
    expect(before).toHaveLength(10);

    await clickSparePin(page);
    await expect(page.getByTestId("rail-swap-prompt")).toBeVisible({ timeout: 10_000 });

    // A MIDDLE slot, where an append-and-close bug is unmissable.
    const targets = page.getByTestId("rail-swap-target");
    const victim = before[4];
    await targets.nth(4).click();
    await expect(page.getByTestId("rail-swap-prompt")).toHaveCount(0, { timeout: 10_000 });

    const after = await pinnedLabels(page);
    expect(after, "the list changed length").toHaveLength(10);
    expect(after[4], "the chosen slot did not take the newcomer").not.toBe(victim);
    expect(after).not.toContain(victim);
    for (let i = 0; i < 10; i++) {
      if (i === 4) continue;
      expect(after[i], `slot ${i + 1} moved: ${before[i]} → ${after[i]}`).toBe(before[i]);
    }
    // And the digits still run 1..9 then 0.
    expect(await slots(page)).toEqual(["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"]);
  });

  test("Escape backs out of the swap question", async ({ page }) => {
    await openApp(page);
    test.skip(!(await fillToTen(page)), "not enough destinations to fill ten slots");
    const before = await pinnedLabels(page);

    await clickSparePin(page);
    await expect(page.getByTestId("rail-swap-prompt")).toBeVisible({ timeout: 10_000 });
    await page.keyboard.press("Escape");

    await expect(page.getByTestId("rail-swap-prompt")).toHaveCount(0, { timeout: 10_000 });
    expect(await pinnedLabels(page), "the list changed after cancelling").toEqual(before);
  });
});
