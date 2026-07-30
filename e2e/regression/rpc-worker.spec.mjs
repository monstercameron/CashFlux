import { test, expect, nav } from "./fixtures.mjs";

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const BACKEND = `http://127.0.0.1:${RPC_PORT}/`;

test.describe("GWC 5 services worker", () => {
  test.skip(!!process.env.E2E_BASE_URL, "the external visual-test server has no hermetic RPC backend");

  test("boots one dedicated worker and completes a real workspace sync", async ({ app }) => {
    await expect
      .poll(() => app.evaluate(() => document.documentElement.getAttribute("data-services-worker")))
      .toBe("ready");

    await expect
      .poll(() => app.workers().filter((worker) => worker.url().endsWith("/services-worker.js")).length)
      .toBe(1);

    // The seeded demo is intentionally protected from upload. Start with the
    // product's real empty-workspace flow so Sync now must perform a successful
    // PutWorkspace/GetWorkspace round trip against the hermetic GoGRPCBridge.
    await app.locator('[data-testid="sample-start-fresh"]').click();
    await expect(app.locator('[data-testid="sample-data-banner"]')).toHaveCount(0);

    await nav(app, "/settings");
    await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
    await app.locator("[role=switch]").first().click();

    const useDifferent = app.locator('[data-testid="sync-use-different-address"]');
    if (await useDifferent.count()) await useDifferent.click();
    await app.locator('[data-testid="sync-server-url"]').fill(BACKEND);

    await app.locator('[data-testid="sync-advanced-token-toggle"]').click();
    await app.locator('[data-testid="sync-server-token"]').fill("e2e-worker-token");
    await app.locator('[data-testid="sync-now"]').click();

    const status = app.locator('[data-testid="sync-status-card"]');
    await expect(status).toContainText(/synced/i, { timeout: 30_000 });
    await expect(status).not.toContainText(/sync error/i);
    await expect
      .poll(() => app.evaluate(() => document.documentElement.getAttribute("data-services-worker")))
      .toBe("ready");
  });
});
