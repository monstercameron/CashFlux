import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, addUniqueTransaction, datasetHas, reloadReady, accountSnapshots, accessToken, unlockIfNeeded } from "./sv0_helpers.mjs";
import { nav } from "./fixtures.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("a stale W1 tab cannot send its edit into W2", async ({ app, browser, request }) => {
  test.setTimeout(180_000);
  const username = `sv0-ws-${crypto.randomUUID().slice(0, 8)}`;
  const password = `CashFlux-W-${crypto.randomUUID().slice(0, 8)}-Pass!`;
  await createPasswordAccount(request, server.origin, username, password);
  await configureRemote(app, server.origin);
  await login(app, username, password);
  await waitForSync(app);
  const w1ID = await app.evaluate(() => JSON.parse(window.cashfluxStoreGet?.("cashflux:workspaces") || "{}").activeId || "");

  const secondTab = await app.context().newPage();
  await secondTab.goto("/");
  await secondTab.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await unlockIfNeeded(secondTab);
  await expect(secondTab.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();

  // Use the product's workspace command so the second page follows the same
  // create/switch path as a household member, then let its reload finish.
  await secondTab.keyboard.press("Control+k");
  const palette = secondTab.locator("#cf-cmd-palette");
  await expect(palette).toBeVisible();
  await secondTab.locator("#cf-cmd-input").fill("new workspace");
  await secondTab.getByText(/new workspace/i).last().click();
  const name = `SV0-W2-${crypto.randomUUID().slice(0, 6)}`;
  const prompt = secondTab.locator("input").last();
  await expect(prompt).toBeVisible();
  await prompt.fill(name);
  await Promise.all([secondTab.waitForEvent("load"), prompt.press("Enter")]);
  await secondTab.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  const w2ID = await secondTab.evaluate(() => JSON.parse(window.cashfluxStoreGet?.("cashflux:workspaces") || "{}").activeId || "");
  expect(w1ID).not.toBe("");
  expect(w2ID).not.toBe("");
  expect(w2ID).not.toBe(w1ID);

  const marker = `SV0-STALE-W1-${crypto.randomUUID().slice(0, 8)}`;
  await addUniqueTransaction(app, marker);
  await waitForSync(app);
  await reloadReady(secondTab);
  expect(await datasetHas(secondTab, marker), "W2 must not receive an edit authored in the still-open W1 tab").toBe(false);
  const copies = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(copies.some((snapshot) => snapshot.workspaceID === w1ID), "W1's encrypted server copy must remain present").toBe(true);
  expect(copies.some((snapshot) => snapshot.envelope.includes(marker)), "the server must not see the plaintext W1 edit").toBe(false);
  expect(await datasetHas(secondTab, marker), "W2's local data must not receive W1's edit").toBe(false);
  await nav(secondTab, "/settings");
  await expect(secondTab.getByText(name, { exact: true })).toBeVisible();
  await secondTab.close();
});
