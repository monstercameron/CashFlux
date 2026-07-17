// notifications_actions.spec.mjs — per-notification snooze horizons (1d/1w/1m)
// and the one-level undo for a dismissed notification.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("notifications: snooze horizons + undo dismiss", () => {
  test("snoozing for a week hides the alert; dismissing offers Undo that restores it", async ({ app }) => {
    await nav(app, "/notifications");
    // Snooze the first alert for a week via the new horizon menu.
    const snoozeBtn = app.locator('[data-testid^="notif-snooze-"]').first();
    const sid = (await snoozeBtn.getAttribute("data-testid")).replace("notif-snooze-", "");
    await snoozeBtn.scrollIntoViewIfNeeded();
    await snoozeBtn.click();
    const weekOpt = app.getByTestId(`notif-snooze1w-${sid}`);
    await expect(weekOpt).toBeVisible();
    await expect(app.getByTestId(`notif-snooze1d-${sid}`)).toBeVisible();
    await expect(app.getByTestId(`notif-snooze1m-${sid}`)).toBeVisible();
    await weekOpt.click();
    await expect(app.getByTestId(`notif-${sid}`)).toHaveCount(0);
    // Dismiss the (new) first alert; the undo bar restores it.
    const dismissBtn = app.locator('[data-testid^="notif-dismiss-"]').first();
    const did = (await dismissBtn.getAttribute("data-testid")).replace("notif-dismiss-", "");
    await dismissBtn.click();
    await expect(app.getByTestId(`notif-${did}`)).toHaveCount(0);
    const undoBar = app.getByTestId("notif-undo-bar");
    await expect(undoBar).toBeVisible();
    await app.getByTestId("notif-undo-dismiss").click();
    await expect(app.getByTestId(`notif-${did}`)).toBeVisible();
    await expect(app.getByTestId("notif-undo-bar")).toHaveCount(0);
  });
});
