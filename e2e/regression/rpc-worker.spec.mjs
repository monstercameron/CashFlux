import { test, expect, boot, nav } from "./fixtures.mjs";

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const BACKEND = `http://127.0.0.1:${RPC_PORT}/`;
const APP_PORT = process.env.E2E_PORT || "8099";
const APP = `http://127.0.0.1:${APP_PORT}`;
const ACCOUNT_NAME = "Worker round-trip account";
const SYNC_PASSCODE = "CashFlux-RPC-Worker-7294";

async function connectToBackend(app, { expectReload = false } = {}) {
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();

  const start = app.getByTestId("cloud-setup-start");
  if (await start.isVisible()) await start.click();
  await expect(app.getByTestId("cloud-setup-where")).toBeVisible();
  await app.getByRole("radio", { name: "Your own server", exact: true }).click();
  await app.getByTestId("cloud-address-advanced").click();
  await app.getByTestId("sync-server-url").fill(BACKEND);
  await app.getByTestId("sync-test").click();
  await expect(app.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
  await app.getByTestId("cloud-where-continue").click();
  await app.locator('[data-testid="sync-advanced-token-toggle"]').click();
  await app.locator('[data-testid="sync-server-token"]').fill("e2e-worker-token");
  const authReload = app.waitForEvent("load", { timeout: 30_000 });
  await app.getByTestId("cloud-use-token").click();
  await authReload;
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });

  const lockGate = app.locator("#cf-applock-gate");
  if (await lockGate.isVisible().catch(() => false)) {
    await app.locator("#cf-applock-input").fill(SYNC_PASSCODE);
    await lockGate.getByRole("button", { name: "Unlock" }).click();
    await expect(lockGate).toBeHidden();
  }
  const protect = app.getByTestId("sync-enable-passcode");
  if (await protect.isVisible()) {
    await protect.click();
    await app.locator("#cf-al-pass").fill(SYNC_PASSCODE);
    await app.locator("#cf-al-confirm").fill(SYNC_PASSCODE);
    await app.locator("#cf-al-ok").click();
  }
  await expect(app.getByTestId("cloud-start-encrypted-sync")).toBeVisible({ timeout: 15_000 });
  await app.getByTestId("cloud-start-encrypted-sync").click();

  // A stale status label cannot stand in for the committed encrypted sync.
  const pulse = app.locator('[data-testid="sync-pulse"]');
  await expect(pulse).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });
}

test.describe("GWC 5 services worker", () => {
  test.skip(
    !!process.env.E2E_BASE_URL && !process.env.E2E_LIVE_RPC,
    "the external visual-test server has no hermetic RPC backend",
  );

  test("stores a workspace through one worker and pulls it into a second client", async ({
    app,
    browser,
  }) => {
    test.setTimeout(120_000);
    if (process.env.E2E_LIVE_RPC) {
      // The normal regression clock is pinned to July 1 for deterministic UI
      // fixtures. A persistent live backend can already hold a newer snapshot,
      // so live-mode mutations must use the real current clock for honest LWW.
      await app.clock.setFixedTime(new Date());
    }
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

      // The sample banner mounts after the app-ready marker. Wait for that
      // deferred shell update before interacting with Settings; otherwise it
      // can replace the Cloud pane halfway through a toggle click.
      await expect(second.locator('[data-testid="sample-start-fresh"]')).toBeVisible();
      await connectToBackend(second, { expectReload: true });
      await nav(second, "/accounts");
      await expect(second.locator("#main")).toContainText(ACCOUNT_NAME, { timeout: 30_000 });
      await expect(second.locator('[data-testid="sample-data-banner"]')).toHaveCount(0);
      const sourceDeviceID = await app.evaluate(() => window.cashfluxStoreGet("cashflux:sync-device-id"));
      const secondDeviceID = await second.evaluate(() => window.cashfluxStoreGet("cashflux:sync-device-id"));
      const secondSyncMeta = await second.evaluate(() => window.cashfluxStoreGet("cashflux:sync-meta:default"));
      expect(sourceDeviceID).toBeTruthy();
      // A pull-only client lazily creates its device id when its first watch
      // event arrives; it has not pushed anything yet.
      expect(secondDeviceID).toBeFalsy();
      // This metadata must survive the pull-triggered reload. Without it the
      // next remote update is misclassified as overwriting unsynced local work.
      expect(secondSyncMeta).toBeTruthy();

      // Deletion must follow the same automatic persistence path as creation:
      // local mutation -> four-second autosave -> worker PutWorkspace -> watch
      // event -> second client pull/reload. Do not click "Sync now" anywhere in
      // this half; that would hide a broken automatic flush.
      await nav(app, "/accounts");
      const edit = app.locator('[data-testid^="edit-account-btn-"]').first();
      const accountID = (await edit.getAttribute("data-testid")).replace("edit-account-btn-", "");
      const accountRow = app.getByTestId(`acct-row-${accountID}`);
      await accountRow.locator(".add-wrap > button").click();
      await app.getByTestId(`delete-account-btn-${accountID}`).click();
      await app.locator("#cf-dialog-confirm").click();
      await expect(accountRow).toHaveCount(0);

      // Client two's removal is the persistence oracle for the whole chain:
      // autosave -> accepted worker RPC -> backend snapshot -> watch -> pull.
      await expect(second.locator("#main")).not.toContainText(ACCOUNT_NAME, { timeout: 30_000 });
      await nav(second, "/accounts");
      await expect(second.locator('[data-testid^="acct-row-"]')).toHaveCount(0);
    } finally {
      await secondContext.close();
    }
  });
});
