// reports_saved.spec.mjs — saved report views: name the current period+scope,
// reopen it from the picker, delete it.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("reports: saved views", () => {
  test("save, apply after changing the period, and delete", async ({ app }) => {
    await nav(app, "/reports");
    const saved = app.getByTestId("reports-saved");
    await expect(saved).toBeVisible();
    // No saved views yet: only the Save affordance shows.
    await expect(app.getByTestId("reports-saved-select")).toHaveCount(0);
    // Save the current view under a name.
    await app.getByTestId("reports-saved-open").click();
    await app.getByTestId("reports-saved-name").fill("July check-in");
    await app.getByTestId("reports-saved-confirm").click();
    await expect(app.locator("body")).toContainText(/Saved "July check-in"/);
    const select = app.getByTestId("reports-saved-select");
    await expect(select).toBeVisible();
    // Note the current period pill, then jump the period backward (top bar).
    const pill = app.getByTestId("period-pill");
    const beforeText = (await pill.innerText()).trim();
    await app.locator('[aria-label="Previous period"]').first().click();
    await expect(pill).not.toContainText(beforeText);
    // Applying the saved view restores the saved period.
    await select.selectOption({ label: "July check-in" });
    await expect(app.locator("body")).toContainText(/Applied "July check-in"/);
    await expect(pill).toContainText(beforeText);
    // Delete it: the picker empties back to save-only.
    await app.getByTestId("reports-saved-delete").click();
    await expect(app.locator("body")).toContainText(/Saved view deleted/);
    await expect(app.getByTestId("reports-saved-select")).toHaveCount(0);
  });
});
