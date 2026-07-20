// debt.spec.mjs — regressions for the interactive /debt coaching redesign: the
// Watch-outs alert tile (debtcoach rules), the strategy tuner that drives the whole
// page, and the teaching accordion. The seeded sample dataset carries a mix of
// debts (including a card whose minimum can't outrun its interest and cards with
// non-trivial utilization), so the coaching surfaces have real signal to show.
import { test, expect, nav } from "./fixtures.mjs";

// tunerInterest reads the tuner readout's total-interest stat (the 3rd stat:
// debt-free date, time to clear, total interest) as a raw string.
async function tunerInterest(app) {
  return app.locator('#sec-tuner .debt-stat-value').nth(2).innerText();
}

test.describe("debt: Watch-outs", () => {
  test("renders ranked, severity-railed alerts with why-it-matters copy", async ({ app }) => {
    await nav(app, "/debt");
    const alerts = app.locator('[data-testid^="debt-alert-"]');
    const n = await alerts.count();
    expect(n).toBeGreaterThan(0);

    // Every alert carries a title and an explanation, and a severity class the CSS
    // rail keys off.
    for (let i = 0; i < n; i++) {
      const a = alerts.nth(i);
      await expect(a.locator(".debt-alert-title")).not.toBeEmpty();
      await expect(a.locator(".debt-alert-text")).not.toBeEmpty();
      const cls = await a.getAttribute("class");
      expect(cls).toMatch(/debt-alert-(critical|watch|info)/);
    }

    // Alerts are ordered most-urgent-first: no lower-severity alert precedes a
    // higher-severity one. Map the class to a rank and assert non-increasing.
    const rank = { critical: 3, watch: 2, info: 1 };
    const sev = async (i) => {
      const cls = await alerts.nth(i).getAttribute("class");
      const m = cls.match(/debt-alert-(critical|watch|info)/);
      return rank[m[1]];
    };
    for (let i = 1; i < n; i++) {
      expect(await sev(i - 1)).toBeGreaterThanOrEqual(await sev(i));
    }
  });
});

test.describe("debt: strategy tuner", () => {
  test("method picker reflects and persists the chosen strategy", async ({ app }) => {
    await nav(app, "/debt");
    const snow = app.getByTestId("debt-tuner-snowball");
    const aval = app.getByTestId("debt-tuner-avalanche");

    // Pick snowball; it becomes the pressed segment and avalanche un-presses.
    await snow.click();
    await expect(snow).toHaveAttribute("aria-pressed", "true");
    await expect(aval).toHaveAttribute("aria-pressed", "false");

    // The choice is durable: leave the page and return, snowball is still pressed.
    await nav(app, "/accounts");
    await nav(app, "/debt");
    await expect(app.getByTestId("debt-tuner-snowball")).toHaveAttribute("aria-pressed", "true");
  });

  test("adding an extra payment surfaces a concrete time+interest saving", async ({ app }) => {
    await nav(app, "/debt");
    // At $0 extra there's nothing to beat, so the muted "add an extra" hint shows and
    // the impact callout is absent.
    await expect(app.getByTestId("debt-tuner-impact")).toHaveCount(0);

    // Bump the extra a few times; the impact callout appears and quantifies the win.
    for (let i = 0; i < 4; i++) await app.getByTestId("debt-extra-inc").click();
    const impact = app.getByTestId("debt-tuner-impact");
    await expect(impact).toBeVisible();
    await expect(impact).toContainText(/sooner/i);
    await expect(impact).toContainText(/interest/i);

    // Clear resets the extra back to nothing and the callout disappears again.
    await app.getByTestId("debt-extra-clear").click();
    await expect(app.getByTestId("debt-tuner-impact")).toHaveCount(0);
  });

  test("switching method recomputes the plan (avalanche costs no more interest)", async ({ app }) => {
    await nav(app, "/debt");
    // Give the plan an extra so both methods clear and the totals are comparable.
    for (let i = 0; i < 4; i++) await app.getByTestId("debt-extra-inc").click();

    await app.getByTestId("debt-tuner-avalanche").click();
    await expect(app.getByTestId("debt-tuner-avalanche")).toHaveAttribute("aria-pressed", "true");
    const avalText = await tunerInterest(app);

    await app.getByTestId("debt-tuner-snowball").click();
    await expect(app.getByTestId("debt-tuner-snowball")).toHaveAttribute("aria-pressed", "true");
    const snowText = await tunerInterest(app);

    // The readout is money; parse to a number. Avalanche (highest-APR-first) can
    // never cost MORE interest than snowball on the same debts + extra.
    const num = (s) => Number(s.replace(/[^0-9.]/g, ""));
    expect(num(avalText)).toBeLessThanOrEqual(num(snowText));
    // And the two strategies genuinely differ on this multi-debt seed — proof the
    // page recomputed, not just re-labelled.
    expect(snowText).not.toBe(avalText);
  });
});

test.describe("debt: teaching accordion", () => {
  test("shows the five topics and expands on click", async ({ app }) => {
    await nav(app, "/debt");
    for (const id of ["methods", "trap", "utilization", "order", "consolidate"]) {
      await expect(app.getByTestId(`debt-learn-${id}`)).toBeVisible();
    }
    // The first card is open by default; a closed one opens when its summary is clicked.
    const trap = app.getByTestId("debt-learn-trap");
    expect(await trap.evaluate((el) => el.open)).toBe(false);
    await trap.locator("summary").click();
    expect(await trap.evaluate((el) => el.open)).toBe(true);
  });
});

test.describe("debt: jump navigation", () => {
  test("offers the new sections", async ({ app }) => {
    await nav(app, "/debt");
    await expect(app.getByTestId("debt-jump-sec-watchouts")).toBeVisible();
    await expect(app.getByTestId("debt-jump-sec-tuner")).toBeVisible();
    await expect(app.getByTestId("debt-jump-sec-learn")).toBeVisible();
  });
});
