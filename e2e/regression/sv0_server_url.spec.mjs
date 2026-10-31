import { test, expect, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, unlockIfNeeded } from "./sv0_helpers.mjs";

let serverA;
let serverB;
test.beforeAll(async () => { serverA = await startServer(); serverB = await startServer(); });
test.afterAll(async () => { await serverB?.stop(); await serverA?.stop(); });

test("changing the server URL clears A's token before contacting B", async ({ app, request }) => {
  test.setTimeout(180_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const user = { username: `sv0-url-${suffix}`, password: `CashFlux-U-${suffix}-Pass!` };
  await createPasswordAccount(request, serverA.origin, user.username, user.password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load", { timeout: 20_000 }), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await configureRemote(app, serverA.origin);
  await login(app, user.username, user.password);
  await waitForSync(app);

  const before = await app.evaluate(() => ({
    refresh: window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "",
    access: window.cashfluxStoreGet?.("cashflux:auth:access-token") || "",
  }));
  expect(before.refresh || before.access).not.toBe("");
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  await app.getByTestId("cloud-advanced").click();
  await app.getByTestId("cloud-change-connection").click();
  await expect(app.locator("#cf-dialog-confirm")).toBeVisible();
  await app.locator("#cf-dialog-confirm").click();
  await expect(app.getByTestId("cloud-setup-where")).toBeVisible();
  await app.getByTestId("cloud-server-options").click();
  await app.getByTestId("sync-server-url").fill(`${serverB.origin}/`);
  await app.getByTestId("sync-test").click();
  await expect(app.locator("#cf-dialog-confirm")).toBeVisible();
  const changedHost = app.waitForEvent("load", { timeout: 30_000 });
  await app.locator("#cf-dialog-confirm").click();
  await changedHost;
  await expect.poll(async () => (await app.locator("#cf-applock-gate").isVisible()) || await app.evaluate(() => document.documentElement.getAttribute("data-app-ready") === "true"), { timeout: 20_000 }).toBe(true);
  await unlockIfNeeded(app);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  const after = await app.evaluate(() => ({
    refresh: window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "",
    access: window.cashfluxStoreGet?.("cashflux:auth:access-token") || "",
  }));
  expect(after.refresh, "server A refresh token must be cleared on host change").toBe("");
  expect(after.access, "server A access token must be cleared on host change").toBe("");

  await expect(app.getByTestId("cloud-setup-where")).toBeVisible({ timeout: 15_000 });
  await app.getByTestId("cloud-server-options").click();
  await app.getByTestId("sync-test").click();
  await expect(app.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
  await app.getByTestId("cloud-where-continue").click();

  // The destination is an independent empty server. The next auth gate must be
  // signed out; no A session or queued snapshot may be replayed there.
  await expect(app.getByRole("button", { name: "Sign out", exact: true })).toHaveCount(0);
  await expect(app.getByTestId("cloud-setup-signin")).toBeVisible({ timeout: 20_000 });
  const version = await request.get(`${serverB.origin}/v1/admin/users`, { headers: { Authorization: "Bearer sv0-e2e-admin-token" } });
  expect(version.status()).toBe(200);
  expect((await version.json()).users || []).toHaveLength(0);
});
