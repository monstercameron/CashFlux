import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, reloadReady } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("an encrypted synced workspace reopens after App Lock unlock", async ({ app, request }) => {
  test.setTimeout(180_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const username = `sv0-reload-${suffix}`;
  const password = `CashFlux-Reload-${suffix}-Pass!`;
  await createPasswordAccount(request, server.origin, username, password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load"), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await configureRemote(app, server.origin);
  await login(app, username, password);
  await waitForSync(app);
  await reloadReady(app);
  await expect(app.locator(".cf-shell")).toBeVisible();
});
