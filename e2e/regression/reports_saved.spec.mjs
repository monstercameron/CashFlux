// reports_saved.spec.mjs — saved report views: name the current period+scope,
// reopen it from the picker, delete it.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("reports: turn into action", () => {
  test("the actions menu creates a follow-up task and routes to the review inbox", async ({ app }) => {
    await nav(app, "/reports");
    await app.getByTestId("reports-actions-btn").click();
    await app.getByTestId("reports-action-task").click();
    await expect(app.locator("body")).toContainText(/Task added: Follow up on the .* report/);
    // The task exists on /todo.
    await nav(app, "/todo");
    await app.getByTestId("todo-search").fill("Follow up on the");
    await expect(app.locator("#main")).toContainText(/Follow up on the .* report/);
    // Review inbox action opens the inbox overlay.
    await nav(app, "/reports");
    await app.getByTestId("reports-actions-btn").click();
    await app.getByTestId("reports-action-review").click();
    await expect(app.locator('[role="dialog"]')).toBeVisible();
    await app.keyboard.press("Escape");
  });
});

test.describe("reports: life-event annotations", () => {
  test("an event overlapping the report window shows as a chip", async ({ app }) => {
    // Create a life event inside the current (default) report month.
    await nav(app, "/events");
    await app.getByTestId("events-add").click();
    await app.getByTestId("event-name").fill("Portugal trip");
    await app.getByTestId("event-start").fill("2026-07-05");
    await app.getByTestId("event-end").fill("2026-07-10");
    await app.getByTestId("event-save").click();
    // The report annotates the window with it.
    await nav(app, "/reports");
    const chips = app.getByTestId("report-event-chips");
    await chips.scrollIntoViewIfNeeded();
    await expect(chips).toBeVisible();
    await expect(chips).toContainText(/Life events in this period/);
    await expect(chips).toContainText(/Portugal trip/);
    await expect(chips).toContainText(/Jul 5 – Jul 9/); // End is exclusive
    // Manage routes to /events.
    await app.getByTestId("report-events-manage").click();
    await expect(app.locator('#main[data-route="/events"]').first()).toBeVisible();
  });
});

test.describe("reports: snapshots", () => {
  test("Snapshot freezes the aggregates; the picker reopens them read-only", async ({ app }) => {
    await nav(app, "/reports");
    await app.getByTestId("reports-snap-take").click();
    await expect(app.locator("body")).toContainText(/Snapshot of .* saved/);
    // The snapshot auto-selects and renders the frozen panel.
    const panel = app.getByTestId("report-snap-panel");
    await expect(panel).toBeVisible();
    await expect(panel).toContainText(/Frozen view of .* read-only/);
    await expect(panel).toContainText(/Income .*Spending .*Net /);
    // The picker lists it; deleting empties the panel.
    await expect(app.getByTestId("reports-snap-select")).toBeVisible();
    await app.getByTestId("reports-snap-delete").click();
    await expect(app.getByTestId("report-snap-panel")).toHaveCount(0);
  });
});

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
