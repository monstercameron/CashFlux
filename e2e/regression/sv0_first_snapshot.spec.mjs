import { test, expect, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accountSnapshots, accessToken } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("first encrypted snapshot reaches a newly created server workspace", async ({ app, request }) => {
  test.setTimeout(180_000);
  await app.clock.setFixedTime(new Date());
  const suffix = crypto.randomUUID().slice(0, 8);
  const name = `First sync ${suffix}`;
  const username = `sv0-first-${suffix}`;
  const password = `CashFlux-First-${suffix}-Pass!`;
  await createPasswordAccount(request, server.origin, username, password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load", { timeout: 20_000 }), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await nav(app, "/accounts");
  await app.getByTestId("accounts-add").click();
  const form = app.getByTestId("account-add-form");
  await expect(form).toBeVisible();
  await form.locator('input[type="text"]').first().fill(name);
  await form.locator('input[type="number"]').first().fill("10");
  await app.locator('button[form="account-add-form"]').click();

  await configureRemote(app, server.origin);
  await expect(app.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
  await login(app, username, password);
  await waitForSync(app);
  const snapshots = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(snapshots).toHaveLength(1);
  expect(snapshots[0].envelope.startsWith("\u0000cf1\u0000")).toBe(true);
  expect(snapshots[0].envelope).not.toContain(name);
});
