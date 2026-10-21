import { test, expect, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, signOut, waitForSync, addUniqueTransaction, accountSnapshots, accessToken, E2E_PASSCODE, unlockIfNeeded } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

async function unlockAfterSignOut(page) {
  const gate = page.locator("#cf-applock-gate");
  await expect(gate).toBeVisible({ timeout: 45_000 });
  await page.locator("#cf-applock-input").fill(E2E_PASSCODE);
  await gate.getByRole("button", { name: "Unlock" }).click();
  await expect(gate).toBeHidden({ timeout: 45_000 });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
}

test("signing out of A and into B cannot route A's queued data to B", async ({ app, request }) => {
  test.setTimeout(420_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const a = { username: `sv0-a-${suffix}`, password: `CashFlux-A-${suffix}-Pass!` };
  const b = { username: `sv0-b-${suffix}`, password: `CashFlux-B-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, a.username, a.password);
  await createPasswordAccount(request, server.origin, b.username, b.password);
  await app.clock.setFixedTime(new Date());
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible({ timeout: 30_000 });
  await Promise.all([app.waitForEvent("load", { timeout: 20_000 }), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await nav(app, "/accounts");
  await app.getByTestId("accounts-add").click();
  const accountForm = app.getByTestId("account-add-form");
  await expect(accountForm).toBeVisible();
  await accountForm.locator('input[type="text"]').first().fill(`Account switch ${suffix}`);
  await accountForm.locator('input[type="number"]').first().fill("10");
  await app.locator('button[form="account-add-form"]').click();
  await expect(accountForm).toBeHidden({ timeout: 20_000 });
  let localGeneration = "";
  let settledSaves = 0;
  await expect.poll(async () => {
    const generation = await app.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset:gen") || "");
    const failed = await app.getByText("Changes aren't saved yet").isVisible().catch(() => false);
    settledSaves = generation && generation === localGeneration && !failed ? settledSaves + 1 : 0;
    localGeneration = generation;
    return settledSaves >= 4;
  }, { timeout: 30_000, intervals: [500] }).toBe(true);
  await configureRemote(app, server.origin);
  await login(app, a.username, a.password);
  await waitForSync(app);
  const aToken = await accessToken(app);
  expect(aToken).not.toBe("");
  const beforeAdd = await accountSnapshots(request, server.origin, aToken);

  const marker = `SV0-ACCOUNT-A-${suffix}`;
  await addUniqueTransaction(app, marker);
  await waitForSync(app);
  await expect(app.locator('tr[data-testid^="txn-row-"]').filter({ hasText: marker })).toBeVisible();
  await expect.poll(async () => {
    const snapshots = await accountSnapshots(request, server.origin, aToken);
    return snapshots.some((snapshot) => !beforeAdd.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 45_000 }).toBe(true);
  const aCopy = await accountSnapshots(request, server.origin, aToken);
  expect(aCopy.length, "A's server copy should be present after encrypted sync").toBeGreaterThan(0);
  expect(aCopy.some((snapshot) => snapshot.envelope.includes(marker)), "the server must not see A's plaintext transaction").toBe(false);

  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  await signOut(app);
  await unlockAfterSignOut(app);
  await configureRemote(app, server.origin);
  await login(app, b.username, b.password, true);
  await app.reload();
  await expect.poll(async () => {
    await unlockIfNeeded(app);
    return app.getByTestId("foreign-dataset-gate").isVisible();
  }, { timeout: 60_000, intervals: [500, 1000] }).toBe(true);
  await expect(app.getByText(marker), "B must not see A's local transaction after identity reload").toHaveCount(0);
  const bCopy = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(bCopy, "B's server account must not inherit A's workspace").toHaveLength(0);
  expect(bCopy.some((snapshot) => snapshot.envelope.includes(marker)), "the server must not see A's plaintext transaction in B's scope").toBe(false);

  await signOut(app);
  await unlockAfterSignOut(app);
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  await app.getByTestId("cloud-advanced").click();
  await app.getByTestId("cloud-change-connection").click();
  const confirmChange = app.locator("#cf-dialog-confirm");
  if (await confirmChange.isVisible()) await confirmChange.click();
  await configureRemote(app, server.origin);
  await login(app, a.username, a.password);
  await waitForSync(app);
  await nav(app, "/transactions");
  await expect(app.locator('tr[data-testid^="txn-row-"]').filter({ hasText: marker }), "A's server copy must retain A's transaction").toBeVisible();
  const restoredACopy = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(restoredACopy.length, "A's encrypted server copy must remain available").toBeGreaterThan(0);
  expect(restoredACopy.some((snapshot) => snapshot.envelope.includes(marker)), "A's server copy must remain encrypted").toBe(false);
});
