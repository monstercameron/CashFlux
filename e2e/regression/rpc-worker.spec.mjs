import { test, expect, boot, nav } from "./fixtures.mjs";

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const BACKEND = `http://127.0.0.1:${RPC_PORT}/`;
const APP_PORT = process.env.E2E_PORT || "8099";
const APP = `http://127.0.0.1:${APP_PORT}`;
const ACCOUNT_NAME = "Worker round-trip account";

async function connectToBackend(app, { expectReload = false } = {}) {
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();

  const toggle = app.locator('[role="switch"]').first();
  if ((await toggle.getAttribute("aria-checked")) !== "true") {
    await toggle.click();
  }
  await expect(toggle).toHaveAttribute("aria-checked", "true");

  await app.locator(".cloud-tab button", { hasText: "Remote" }).click();
  await app.locator('[data-testid="sync-server-url"]').fill(BACKEND);
  await app.locator('[data-testid="sync-advanced-token-toggle"]').click();
  await app.locator('[data-testid="sync-server-token"]').fill("e2e-worker-token");

  const reload = expectReload
    ? app.waitForEvent("load", { timeout: 30_000 })
    : null;
  await app.locator('[data-testid="sync-now"]').click();
  if (reload) {
    await reload;
    await expect(app.locator("#app")).toBeAttached();
    await expect(app.locator("#main")).toBeVisible({ timeout: 45_000 });
    await expect
      .poll(() => app.evaluate(() => document.documentElement.getAttribute("data-services-worker")))
      .toBe("ready");
    return;
  }

  // An old "Synced" label can predate this click. Observe this mutation enter
  // and leave the real queue before accepting the result.
  const pulse = app.locator('[data-testid="sync-pulse"]');
  await expect(pulse).toHaveAttribute("data-sync-state", "syncing", { timeout: 30_000 });
  await expect(pulse).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });
}

test.describe("GWC 5 services worker", () => {
  test.skip(!!process.env.E2E_BASE_URL, "the external visual-test server has no hermetic RPC backend");

  test("stores a workspace through one worker and pulls it into a second client", async ({
    app,
    browser,
  }) => {
    test.setTimeout(120_000);
    await expect
      .poll(() => app.evaluate(() => document.documentElement.getAttribute("data-services-worker")))
      .toBe("ready");

    await expect
      .poll(() => app.workers().filter((worker) => worker.url().endsWith("/services-worker.js")).length)
      .toBe(1);

    // The seeded demo is intentionally protected from upload. Wait for its
    // deferred banner, then follow the real empty-workspace flow and wait for
    // the resulting reload before creating the mutation this test will trace.
    const startFresh = app.locator('[data-testid="sample-start-fresh"]');
    await expect(startFresh).toBeVisible();
    await Promise.all([
      app.waitForEvent("load"),
      startFresh.click(),
    ]);
    await app.waitForFunction(
      () => document.documentElement.getAttribute("data-app-ready") === "true",
      null,
      { timeout: 45_000 },
    );
    await expect(app.locator('[data-testid="sample-data-banner"]')).toHaveCount(0);

    await nav(app, "/accounts");
    await app.getByTestId("accounts-add").click();
    const form = app.locator('[data-testid="account-add-form"]');
    await expect(form).toBeVisible();
    await form.locator('input[type="text"]').first().fill(ACCOUNT_NAME);
    await form.locator('input[type="number"]').first().fill("1234.56");
    await app.locator('button[form="account-add-form"]').click();
    await expect(app.locator("#main")).toContainText(ACCOUNT_NAME);

    await connectToBackend(app);

    // A second isolated browser profile is the server-persistence oracle. It
    // starts on the sample, connects with no shared device storage, pulls the
    // server snapshot, reloads, and must render the exact account client one
    // created. This cannot pass on an empty queue or a UI-only "Synced" label.
    const secondContext = await browser.newContext({
      baseURL: APP,
      reducedMotion: "reduce",
    });
    const second = await secondContext.newPage();
    try {
      await boot(second);
      await expect
        .poll(() => second.evaluate(() => document.documentElement.getAttribute("data-services-worker")))
        .toBe("ready");
      await expect
        .poll(() => second.workers().filter((worker) => worker.url().endsWith("/services-worker.js")).length)
        .toBe(1);

      await connectToBackend(second, { expectReload: true });
      await nav(second, "/accounts");
      await expect(second.locator("#main")).toContainText(ACCOUNT_NAME, { timeout: 30_000 });
      await expect(second.locator('[data-testid="sample-data-banner"]')).toHaveCount(0);
    } finally {
      await secondContext.close();
    }
  });
});
