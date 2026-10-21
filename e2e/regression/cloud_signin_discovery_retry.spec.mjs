import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("Cloud sign-in recovers discovery and logs into a closed-registration server without clearing local data", async ({ app, request }) => {
  test.setTimeout(180_000);
  const username = `retry-${Date.now()}`;
  const password = "CashFlux-Retry-2026!";
  await createPasswordAccount(request, server.origin, username, password);
  await configureRemote(app, server.origin);
  await expect(app.getByTestId("password-auth-username")).toBeVisible();
  await expect(app.getByTestId("password-auth-password")).toBeVisible();
  await expect(app.getByTestId("pending-device-expand")).toBeVisible();
  const savedDataset = () => app.evaluate(() => {
    const raw = window.cashfluxStoreGet?.("cashflux:dataset") || "";
    if (!raw) return null;
    const stored = JSON.parse(raw);
    return stored.dataset || stored;
  });
  const before = await savedDataset();
  expect(before).not.toBeNull();

  const versionURL = `${server.origin}/v1/version`;
  await app.route(versionURL, (route) => route.abort());
  await app.reload();
  await expect(app.getByTestId("cloud-setup-signin")).toBeVisible({ timeout: 45_000 });
  await expect(app.getByTestId("sync-discovery-error")).toBeVisible({ timeout: 20_000 });
  const retry = app.getByTestId("cloud-signin-test-connection");
  await expect(retry).toBeEnabled();
  await app.unroute(versionURL);
  await retry.click();
  await expect(app.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
  await expect(app.getByTestId("password-auth-card")).toBeVisible({ timeout: 20_000 });
  const after = await savedDataset();
  for (const key of ["accounts", "transactions", "budgets", "goals", "artifacts", "documents"]) {
    expect(after[key], `${key} changed during a public connection retry`).toEqual(before[key]);
  }
  await login(app, username, password);
  await expect(app.getByTestId("settings-clear-invalid-token")).toHaveCount(0);
});
