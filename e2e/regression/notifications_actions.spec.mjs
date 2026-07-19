// notifications_actions.spec.mjs — per-notification snooze horizons (1d/1w/1m)
// and the one-level undo for a dismissed notification.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("notifications: snooze horizons + undo dismiss", () => {
  test("snoozing for a week hides the alert; dismissing offers Undo that restores it", async ({ app }) => {
    await nav(app, "/notifications");
    // Each row now carries ONE primary action plus a ••• overflow that holds the
    // snooze horizons, alert settings, and dismiss. Open the first row's overflow.
    const ovfBtn = app.locator('[data-testid^="notif-ovf-"]').first();
    const sid = (await ovfBtn.getAttribute("data-testid")).replace("notif-ovf-", "");
    await ovfBtn.scrollIntoViewIfNeeded();
    await ovfBtn.click();
    const weekOpt = app.getByTestId(`notif-snooze1w-${sid}`);
    await expect(weekOpt).toBeVisible();
    await expect(app.getByTestId(`notif-snooze1d-${sid}`)).toBeVisible();
    await expect(app.getByTestId(`notif-snooze1m-${sid}`)).toBeVisible();
    await weekOpt.click();
    await expect(app.getByTestId(`notif-${sid}`)).toHaveCount(0);
    // Dismiss the (new) first alert from its overflow menu; the undo bar restores it.
    const ovf2 = app.locator('[data-testid^="notif-ovf-"]').first();
    const did = (await ovf2.getAttribute("data-testid")).replace("notif-ovf-", "");
    await ovf2.scrollIntoViewIfNeeded();
    await ovf2.click();
    await app.getByTestId(`notif-dismiss-${did}`).click();
    await expect(app.getByTestId(`notif-${did}`)).toHaveCount(0);
    const undoBar = app.getByTestId("notif-undo-bar");
    await expect(undoBar).toBeVisible();
    await app.getByTestId("notif-undo-dismiss").click();
    await expect(app.getByTestId(`notif-${did}`)).toBeVisible();
    await expect(app.getByTestId("notif-undo-bar")).toHaveCount(0);
  });
});
