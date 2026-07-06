// smoke.spec.mjs — the stable floor: every routed page loads with substantive,
// on-topic content and zero genuine console/page errors, in both themes. This is
// "the app is not broken on any route", independent of visual polish. The deeper
// per-page interaction checks live in interactions.spec.mjs.
import { test, expect, ROUTES, nav, mainText, setTheme } from "./fixtures.mjs";

test.describe("all-routes smoke", () => {
  // One comprehensive sweep over every route — deliberately a single test (one
  // boot/seed) rather than 46 re-booting tests. It does real work, so it gets a
  // generous budget (CI runners are slower than local).
  test("every route renders substantive, on-topic content with no app errors", async ({ app, errors }) => {
    test.setTimeout(300_000);
    for (const [route, rx] of ROUTES) {
      await nav(app, route);
      const body = (await mainText(app)).trim();
      expect(body.length, `${route}: body empty / too short`).toBeGreaterThan(40);
      expect(body, `${route}: body missing expected anchor ${rx}`).toMatch(rx);
    }
    expect(errors, `console/page errors during route sweep:\n${errors.join("\n")}`).toEqual([]);
  });

  test("light theme recolors every sampled route without error", async ({ app, errors }) => {
    await setTheme(app, "light");
    for (const route of ["/", "/transactions", "/reports", "/settings", "/subscriptions", "/p/priya-business"]) {
      await nav(app, route);
      await expect
        .poll(() => app.evaluate(() => document.documentElement.getAttribute("data-theme")))
        .toBe("light");
    }
    await setTheme(app, "dark");
    expect(errors, `errors during light-theme sweep:\n${errors.join("\n")}`).toEqual([]);
  });
});
