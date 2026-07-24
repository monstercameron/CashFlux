// sync.spec.mjs — guards the /sync page and Settings -> Cloud pane against ever
// again showing a sync failure with no visible reason (TODOS.md C450: "why is
// there no fold or pocket where I can see the sync error"). The hermetic e2e
// server has no real backend behind it, so pointing sync at any address
// deterministically fails to dial — exactly the real-world "backend unavailable"
// class of error this guards, with zero mocking required.
import { test, expect, nav } from "./fixtures.mjs";

// UNREACHABLE is a loopback port nothing listens on in the hermetic test
// environment, so connecting to it fails fast (connection refused) rather than
// timing out — keeping the test quick without relying on a live server.
const UNREACHABLE = "http://127.0.0.1:8199/";

test.describe("sync error visibility", () => {
  test("/sync status card shows the specific failure reason, not just a generic label", async ({ app }) => {
    await nav(app, "/sync");
    await app.locator("[role=switch]").first().click();

    const useDifferent = app.locator('[data-testid="sync-use-different-address"]');
    if (await useDifferent.count() > 0) await useDifferent.click();

    await app.locator('[data-testid="sync-server-url"]').fill(UNREACHABLE);
    const tokenField = app.locator('[data-testid="sync-server-token"]');
    await tokenField.waitFor({ timeout: 10_000 });
    await tokenField.fill("not-a-real-token");
    await app.locator('[data-testid="sync-now"]').click();

    const statusCard = app.locator('[data-testid="sync-status-card"]');
    await expect(statusCard).toContainText(/sync error/i, { timeout: 20_000 });

    const detail = app.locator('[data-testid="sync-status-detail"]');
    await expect(detail).toBeVisible();
    const text = (await detail.innerText()).trim();
    // The regression this guards against: syncStatusLabel() collapsing every
    // failure into the bare literal ("Sync error" / "Reason: pull failed") with
    // no way to tell WHY. A real dial failure's underlying error text is always
    // longer than the bare fallback phrase alone.
    expect(text, "status detail should not be empty").not.toBe("");
    expect(text.length, `status detail should include real error detail, got: ${text}`).toBeGreaterThan("Reason: pull failed".length);
  });

  test("Settings -> Cloud shows the same failure reason", async ({ app }) => {
    await nav(app, "/sync");
    await app.locator("[role=switch]").first().click();
    const useDifferent = app.locator('[data-testid="sync-use-different-address"]');
    if (await useDifferent.count() > 0) await useDifferent.click();
    await app.locator('[data-testid="sync-server-url"]').fill(UNREACHABLE);
    const tokenField = app.locator('[data-testid="sync-server-token"]');
    await tokenField.waitFor({ timeout: 10_000 });
    await tokenField.fill("not-a-real-token");
    await app.locator('[data-testid="sync-now"]').click();
    await expect(app.locator('[data-testid="sync-status-card"]')).toContainText(/sync error/i, { timeout: 20_000 });

    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
    const detail = app.locator('[data-testid="settings-sync-status-detail"]');
    await expect(detail).toBeVisible({ timeout: 10_000 });
    const text = (await detail.innerText()).trim();
    expect(text.length, `Settings status detail should include real error detail, got: ${text}`).toBeGreaterThan("Reason: pull failed".length);
  });
});
