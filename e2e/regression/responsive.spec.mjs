// responsive.spec.mjs — desktop responsiveness acceptance ratchet.
//
// The July 19 responsiveness pass (styles/breakpoints.go) makes page layout
// respond to CONTENT width — viewport minus the live rail — via the cf-rail-c
// <html> mirror class and the ruleContentMax/Min dual-emission helpers. This
// suite pins the two properties that pass established:
//
//   1. NO SILENT HORIZONTAL CLIPPING: on every first-nine page, at every
//      supported desktop width, in BOTH rail states, no rendered element's
//      right edge extends past the main pane (outside an explicitly scrollable
//      ancestor). The failure mode this guards against is invisible loss —
//      hidden balances, actions, and table columns with no scrollbar to reveal
//      them (the v1.2.3 audit's top defect).
//
//   2. THE RAIL STATE DRIVES LAYOUT: collapsing the sidebar widens the pane by
//      182px and the breakpoint system must actually see it — the <html>
//      mirror class tracks the rail, flips layout regimes live, and survives
//      a reload.
import { test, expect, nav, settle } from "./fixtures.mjs";

const ROUTES = [
  "/",
  "/transactions",
  "/accounts",
  "/budgets",
  "/goals",
  "/todo",
  "/notifications",
  "/assistant",
  "/reports",
];

// The supported desktop matrix (audit §2): common laptop widths through QHD.
// 2560 is checked at 1440-high; the rest at 900/768-class heights.
const WIDTHS = [1024, 1280, 1366, 1440, 1920, 2560];

// overflowOffenders returns descriptors for elements whose right edge passes
// the main pane's, excluding elements inside an overflow-x scrollable ancestor
// (explicit scroll is an allowed, visible escape — silent clipping is not),
// zero-size boxes, fixed-position overlays, and aria-hidden décor.
async function overflowOffenders(page) {
  return page.evaluate(() => {
    const main = document.querySelector("#main");
    if (!main) return ["no #main"];
    const limit = main.getBoundingClientRect().right + 1;
    const out = [];
    for (const el of main.querySelectorAll("*")) {
      if (el.closest("[aria-hidden='true']")) continue;
      const cs = getComputedStyle(el);
      if (cs.display === "none" || cs.visibility === "hidden" || cs.position === "fixed") continue;
      const r = el.getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      if (r.right - limit <= 4) continue;
      let anc = el.parentElement, scrollable = false;
      while (anc && anc !== main) {
        if (/(auto|scroll)/.test(getComputedStyle(anc).overflowX)) { scrollable = true; break; }
        anc = anc.parentElement;
      }
      if (scrollable) continue;
      const cls = (typeof el.className === "string" ? el.className : "").split(/\s+/).slice(0, 3).join(".");
      out.push(`${el.tagName.toLowerCase()}${el.id ? "#" + el.id : ""}${cls ? "." + cls : ""} +${Math.round(r.right - limit)}px`);
      if (out.length >= 5) break;
    }
    return out;
  });
}

async function setRail(page, collapsed) {
  const is = await page.evaluate(() => document.querySelector("aside.rail")?.classList.contains("collapsed") ?? false);
  if (is !== collapsed) {
    await page.locator('[data-testid="rail-collapse-btn"]').click();
    await expect
      .poll(() => page.evaluate(() => document.querySelector("aside.rail")?.classList.contains("collapsed")))
      .toBe(collapsed);
  }
}

for (const collapsed of [false, true]) {
  test(`no silent horizontal clipping on the first nine pages (rail ${collapsed ? "collapsed" : "expanded"})`, async ({ app: page }) => {
    test.setTimeout(240_000);
    await setRail(page, collapsed);
    const failures = [];
    for (const width of WIDTHS) {
      await page.setViewportSize({ width, height: width >= 2560 ? 1440 : 900 });
      for (const route of ROUTES) {
        await nav(page, route);
        await settle(page);
        const offenders = await overflowOffenders(page);
        if (offenders.length) failures.push(`${route} @ ${width}px: ${offenders.join(", ")}`);
      }
    }
    expect(failures, failures.join("\n")).toEqual([]);
  });
}

test("the rail state drives layout regimes and survives reload", async ({ app: page }) => {
  // 1180px: expanded pane = 940px (< contentGrid4 → 2-col bento), collapsed
  // pane = 1122px (≥ contentGrid4 → 4-col bento). The regime must follow the
  // toggle live, and the persisted state must reproduce it after a reload.
  await page.setViewportSize({ width: 1180, height: 900 });
  await setRail(page, false);
  // Null-safe: .bento mounts a beat after data-app-ready (below-fold deferral),
  // so a not-yet-rendered grid reports cols:0 and the expect.poll simply retries.
  const state = () =>
    page.evaluate(() => {
      const bento = document.querySelector(".bento");
      return {
        mirror: document.documentElement.classList.contains("cf-rail-c"),
        cols: bento ? getComputedStyle(bento).gridTemplateColumns.split(" ").length : 0,
      };
    });
  await expect.poll(async () => (await state()).cols).toBe(2);
  expect((await state()).mirror).toBe(false);

  await setRail(page, true);
  await expect.poll(async () => (await state()).cols).toBe(4);
  expect((await state()).mirror).toBe(true);

  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await expect.poll(async () => (await state()).mirror, { timeout: 15_000 }).toBe(true);
  await expect.poll(async () => (await state()).cols).toBe(4);

  // Leave the app expanded (the persisted default) so later tests in the
  // worker's storage state aren't surprised.
  await setRail(page, false);
});
