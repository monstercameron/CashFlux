// notifications_actions.spec.mjs — per-notification snooze horizons (1d/1w/1m)
// and the one-level undo for a dismissed notification.
import { test, expect, nav } from "./fixtures.mjs";

test.describe("notifications: snooze horizons + undo dismiss", () => {
  test("snoozing for a week hides the alert; dismissing offers Undo that restores it", async ({ app }) => {
    await nav(app, "/notifications");
    // Snooze got promoted out of the ••• overflow to a dedicated clock control
    // (W1/C369): each row carries the primary action, the clock (snooze horizons),
    // and a ••• that keeps alert settings + dismiss. Open the first row's clock.
    const snzBtn = app.locator('[data-testid^="notif-snooze-"]').first();
    const sid = (await snzBtn.getAttribute("data-testid")).replace("notif-snooze-", "");
    await snzBtn.scrollIntoViewIfNeeded();
    await snzBtn.click();
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
