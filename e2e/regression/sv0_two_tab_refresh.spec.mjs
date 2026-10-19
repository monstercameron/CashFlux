import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, unlockIfNeeded } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("two tabs refreshing the same expired access token retain a working session", async ({ app, request }) => {
  test.setTimeout(180_000);
  const username = `sv0-refresh-${crypto.randomUUID().slice(0, 8)}`;
  const password = `CashFlux-R-${crypto.randomUUID().slice(0, 8)}-Pass!`;
  await createPasswordAccount(request, server.origin, username, password);
  await configureRemote(app, server.origin);
  await login(app, username, password);
  await waitForSync(app);
  const other = await app.context().newPage();
  await other.goto("/");
  await other.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await unlockIfNeeded(other);

  // An access-token expiry is deterministic: mutate the stored token in both
  // tabs to an already-expired JWT while retaining the shared refresh token.
  // The app's public store bridge writes this auth record into the shared
  // IndexedDB store. Expiring it this way preserves the rotating refresh token.
  const authKey = "cashflux:auth:access-token";
  const expired = "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjF9.invalid";
  for (const page of [app, other]) {
    const current = await page.evaluate((key) => window.cashfluxStoreGet?.(key) || "", authKey);
    expect(current).not.toBe("");
    await page.evaluate(({ key, value }) => window.cashfluxStoreSet?.(key, value), { key: authKey, value: expired });
    await expect.poll(() => page.evaluate((key) => window.cashfluxStoreGet?.(key) || "", authKey)).toBe(expired);
  }
  await Promise.all([app.getByTestId("sync-now").click(), other.getByTestId("sync-now").click()]);
  for (const page of [app, other]) {
    await expect(page.getByRole("button", { name: "Sign out", exact: true })).toBeVisible({ timeout: 30_000 });
    await waitForSync(page);
  }
  await other.close();
});
