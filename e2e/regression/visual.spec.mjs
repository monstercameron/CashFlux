// visual.spec.mjs — pixel-level visual regression on a CURATED set of stable
// pages, in both themes. Deliberately narrow: full-page pixel diffing of dynamic
// surfaces (dashboards with live charts, relative dates, the smart strip) is the
// classic flaky trap, and those are already covered by invariants + interactions.
// Here we lock the layout/typography/theming of content-stable pages, with the
// clock frozen (so any date-derived copy is deterministic) and dynamic regions
// masked. Baselines are generated in the Playwright Linux container so they match
// CI exactly — see e2e/README.md.
//
// Regenerate (in the Linux container): npm run visual:update  (see README)
import { test, expect, boot, setTheme, nav } from "./fixtures.mjs";

// Content-stable surfaces: marketing/informational pages with no live data.
const VISUAL_ROUTES = ["/about", "/plans"];

// Regions that can still vary frame-to-frame even on a "stable" page.
const MASK = (page) => [page.locator(".smart-strip"), page.locator('[data-testid^="smart-"]')];

test.describe("visual regression", () => {
  // Pixel baselines are only trustworthy in the environment they were captured in.
  // These are committed as Windows (-win32) baselines generated natively on the
  // dev box, so this is a LOCAL Windows gate: it skips off-Windows and skips in CI
  // (font/DPI rendering differs machine-to-machine, which full-page pixel diffs
  // can't tolerate). Regenerate on Windows with `npm run visual:update`.
  test.skip(
    process.platform !== "win32" || !!process.env.CI,
    "visual is a local Windows gate (baselines are -win32; skipped in CI)",
  );
  for (const mode of ["dark", "light"]) {
    for (const route of VISUAL_ROUTES) {
      test(`${route} @ ${mode}`, async ({ page }) => {
        // boot() pins the clock (FIXED_NOW) so date-derived copy is deterministic.
        await boot(page);
        if (mode === "light") await setTheme(page, "light");
        await nav(page, route);
        await expect(page).toHaveScreenshot(`${route.replace(/\//g, "_").replace(/^_/, "")}-${mode}.png`, {
          fullPage: true,
          animations: "disabled",
          mask: MASK(page),
          maxDiffPixelRatio: 0.02,
        });
      });
    }
  }
});
