// health.spec.mjs — regressions for the /health analysis redesign: the resilience
// runway, the interactive stress-test tile (pay cut / surprise bill / rate hike),
// and the money-leaks read (recurring load + spending creep). The seeded sample
// dataset carries income, cash, cards, recurring charges, and a multi-year history,
// so every surface has real signal.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("health: resilience runway", () => {
  test("the hero states a runway and the stress tile leads with the buffer", async ({ app }) => {
    await nav(app, "/health");
    await expect(app.getByTestId("health-runway-hero")).toContainText(/resilient for/i);
    await expect(app.getByTestId("stress-runway")).toContainText(/no income/i);
  });
});

test.describe("health: interactive stress tests", () => {
  test("changing the pay-cut shock recomputes the outcome", async ({ app }) => {
    await nav(app, "/health");
    const out = app.getByTestId("stress-drop");
    const before = await out.innerText();
    // Move from the default 20% to 50% — the outcome sentence must change.
    await app.getByTestId("stress-chip-drop-50").click();
    await expect(app.getByTestId("stress-chip-drop-50")).toHaveAttribute("aria-pressed", "true");
    await expect(out).not.toHaveText(before);
    await expect(out).toContainText(/50%/);
  });

  test("a large surprise bill and a rate hike produce concrete figures", async ({ app }) => {
    await nav(app, "/health");
    await app.getByTestId("stress-chip-sur-3").click(); // the $5,000 preset
    await expect(app.getByTestId("stress-surprise")).toContainText(/surprise/i);
    // Rate-hike outcome names a monthly and annual interest figure (cards exist in the seed).
    await app.getByTestId("stress-chip-rate-10").click();
    await expect(app.getByTestId("stress-rate")).toContainText(/interest/i);
  });
});

test.describe("health: money leaks", () => {
  test("shows the recurring load and spending creep", async ({ app }) => {
    await nav(app, "/health");
    const leaks = app.locator("#sec-health-leaks");
    await expect(leaks).toBeVisible();
    await expect(leaks).toContainText(/recurring commitments/i);
    await expect(leaks).toContainText(/\/ mo/i);
    await expect(leaks).toContainText(/spending creep/i);
  });
});
