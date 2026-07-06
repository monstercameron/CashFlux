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
  // dev box, so this suite runs on Windows only; elsewhere (e.g. Linux CI) it
  // skips rather than failing on a platform mismatch. Regenerate on Windows with
  // `npm run visual:update` (see e2e/README.md).
  test.skip(process.platform !== "win32", "visual baselines are Windows-native (regenerate on Windows)");
  for (const mode of ["dark", "light"]) {
    for (const route of VISUAL_ROUTES) {
      test(`${route} @ ${mode}`, async ({ page }) => {
        // Freeze time BEFORE boot so seeding + any date copy is deterministic.
        await page.clock.install({ time: new Date("2026-07-01T12:00:00Z") });
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
