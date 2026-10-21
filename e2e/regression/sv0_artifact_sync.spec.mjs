import { readFile } from "node:fs/promises";
import { test, expect, boot, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accountSnapshots, accessToken, addUniqueTransaction, unlockIfNeeded } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

async function connectWithPassword(page, username, password, expectExistingRemote = false) {
  await configureRemote(page, server.origin);
  try {
    await login(page, username, password);
  } catch (err) {
    console.log("sync login diagnostic", (await page.locator("body").innerText()).slice(-1000));
    throw err;
  }
  if (expectExistingRemote) await waitForRemoteReady(page);
}

async function waitForRemoteReady(page) {
  let previousGeneration = "";
  let stableChecks = 0;
  await expect.poll(async () => {
    await unlockIfNeeded(page);
    const state = await page.evaluate(() => ({
      owner: !!window.cashfluxStoreGet?.("cashflux:dataset-owner"),
      generation: window.cashfluxStoreGet?.("cashflux:dataset:gen") || "",
      ready: document.documentElement.getAttribute("data-app-ready") === "true",
    })).catch(() => ({ owner: false, generation: "", ready: false }));
    const locked = await page.locator("#cf-applock-gate").isVisible().catch(() => true);
    stableChecks = state.owner && state.ready && !locked && state.generation === previousGeneration ? stableChecks + 1 : 0;
    previousGeneration = state.generation;
    return stableChecks >= 4;
  }, { timeout: 120_000, intervals: [500] }).toBe(true);
}

async function waitForVisibleUnlocked(page, locator) {
  await expect.poll(async () => {
    await unlockIfNeeded(page);
    return locator.isVisible();
  }, { timeout: 90_000, intervals: [500, 1000, 2000] }).toBe(true);
}

test("encrypted file and later edits sync across both devices", async ({ app, browser, request }) => {
  test.setTimeout(420_000);
  // Server timestamps use the real clock; fixture time is intentionally pinned
  // months earlier for UI screenshots and would make this edit look stale.
  await app.clock.setFixedTime(new Date());
  const suffix = crypto.randomUUID().slice(0, 8);
  const filename = `private-receipt-${suffix}.png`;
  const pixels = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");
  const user = { username: `sv0-file-${suffix}`, password: `CashFlux-F-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, user.username, user.password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load", { timeout: 20_000 }), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await nav(app, "/accounts");
  await app.getByTestId("accounts-add").click();
  const accountForm = app.getByTestId("account-add-form");
  await expect(accountForm).toBeVisible();
  await accountForm.locator('input[type="text"]').first().fill(`File sync ${suffix}`);
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
  await connectWithPassword(app, user.username, user.password);
  await expect.poll(async () => (await accountSnapshots(request, server.origin, await accessToken(app))).length, { timeout: 90_000 }).toBeGreaterThan(0);
  await waitForSync(app);
  let previousGeneration = "";
  let stableChecks = 0;
  await expect.poll(async () => {
    const state = await app.evaluate(() => ({
      generation: window.cashfluxStoreGet?.("cashflux:dataset:gen") || "",
      status: JSON.parse(window.cashfluxStoreGet?.("cashflux:sync-status") || "{}"),
    }));
    stableChecks = state.status.state === "synced" && !state.status.pending && state.generation === previousGeneration ? stableChecks + 1 : 0;
    previousGeneration = state.generation;
    return stableChecks >= 4;
  }, { timeout: 30_000, intervals: [500] }).toBe(true);
  await expect(app.getByTestId("stale-dataset-banner")).toHaveCount(0);
  const beforeUpload = await accountSnapshots(request, server.origin, await accessToken(app));

  await nav(app, "/artifacts");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByRole("button", { name: /upload image/i }).click({ timeout: 20_000 }),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: pixels });
  const uploadedName = app.getByTestId("artifact-name").filter({ hasText: filename });
  await expect(uploadedName).toBeVisible();
  const uploadedRow = app.locator(".row").filter({ has: uploadedName });
  const menuID = await uploadedRow.locator('[data-testid^="artifact-menu-btn-"]').getAttribute("data-testid");
  const artifactID = menuID.slice("artifact-menu-btn-".length);
  await expect.poll(() => app.evaluate(async (id) => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open("cashflux", 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    try {
      const record = await new Promise((resolve, reject) => {
        const request = db.transaction("artifacts", "readonly").objectStore("artifacts").get(id);
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      return record?.mime || "";
    } finally {
      db.close();
    }
  }, artifactID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
  await expect.poll(async () => {
    const found = await accountSnapshots(request, server.origin, await accessToken(app));
    return found.some((snapshot) => !beforeUpload.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 90_000 }).toBe(true);
  const serverCopies = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(serverCopies.every((snapshot) => !snapshot.envelope.includes(filename)), "server snapshot must not expose the filename").toBe(true);
  const appOrigin = new URL(app.url()).origin;
  await app.close(); // release the first WebAssembly runtime before booting another profile

  const secondContext = await browser.newContext({ baseURL: appOrigin, reducedMotion: "reduce" });
  const second = await secondContext.newPage();
  try {
    await boot(second);
    await second.clock.setFixedTime(new Date());
    await connectWithPassword(second, user.username, user.password, true);
    await unlockIfNeeded(second);
    await nav(second, "/artifacts");
    await waitForVisibleUnlocked(second, second.getByTestId("artifact-name").filter({ hasText: filename }));
    const row = second.locator(".row").filter({ has: second.getByTestId("artifact-name").filter({ hasText: filename }) });
    await row.locator('[data-testid^="artifact-menu-btn-"]').click({ timeout: 20_000 });
    const [download] = await Promise.all([
      second.waitForEvent("download", { timeout: 25_000 }),
      row.locator('[data-testid^="artifact-download-"]').click({ timeout: 20_000 }),
    ]);
    expect(await readFile(await download.path())).toEqual(pixels);
    const beforeReturn = await accountSnapshots(request, server.origin, await accessToken(second));
    const edit = `second-device-edit-${suffix}`;
    await addUniqueTransaction(second, edit);
    try {
      await expect.poll(async () => {
        const snapshots = await accountSnapshots(request, server.origin, await accessToken(second));
        return snapshots.some((snapshot) => !beforeReturn.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
      }, { timeout: 30_000 }).toBe(true);
    } catch (err) {
      console.log("second browser sync state", await second.evaluate(() => {
        const registry = JSON.parse(window.cashfluxStoreGet?.("cashflux:workspaces") || "{}");
        return {
          status: window.cashfluxStoreGet?.("cashflux:sync-status"),
          queue: !!window.cashfluxStoreGet?.("cashflux:sync-queue"),
          owner: !!window.cashfluxStoreGet?.("cashflux:dataset-owner"),
          generation: !!window.cashfluxStoreGet?.("cashflux:dataset:gen"),
          head: !!window.cashfluxStoreGet?.("cashflux:dataset:head"),
          auth: !!window.cashfluxStoreGet?.("cashflux:auth:user-id"),
          meta: window.cashfluxStoreGet?.(`cashflux:sync-meta:${registry.activeId}`)?.slice(0, 200),
          saveBanner: document.querySelector('[data-testid="save-failure-banner"]')?.textContent,
        };
      }));
      throw err;
    }
    await waitForSync(second);
    let stableSyncChecks = 0;
    const cleanState = async () => {
      const state = await second.evaluate(() => {
        const get = window.cashfluxStoreGet;
        const registry = JSON.parse(get?.("cashflux:workspaces") || "{}");
        return {
          status: JSON.parse(get?.("cashflux:sync-status") || "{}"),
          queued: JSON.parse(get?.("cashflux:sync-queue") || "[]").length > 0,
          conflict: !!get?.(`cashflux:sync-conflict:${registry.activeId}`),
        };
      });
      stableSyncChecks = state.status.state === "synced" && !state.queued && !state.conflict ? stableSyncChecks + 1 : 0;
      return stableSyncChecks >= 6;
    };
    try {
      await expect.poll(cleanState, { timeout: 30_000, intervals: [500] }).toBe(true);
    } catch (err) {
      console.log("second browser unsettled sync", await second.evaluate(() => {
        const get = window.cashfluxStoreGet;
        const registry = JSON.parse(get?.("cashflux:workspaces") || "{}");
        const conflict = JSON.parse(get?.(`cashflux:sync-conflict:${registry.activeId}`) || "null");
        const meta = JSON.parse(get?.(`cashflux:sync-meta:${registry.activeId}`) || "null");
        return { status: get?.("cashflux:sync-status"), queue: get?.("cashflux:sync-queue")?.slice(0, 300), conflict: conflict && { baseVersion: conflict.baseVersion, hash: conflict.hash, hasEdit: conflict.dataset?.includes("second-device-edit") }, meta: meta && { version: meta.version, hash: meta.hash } };
      }));
      throw err;
    }
    const returned = await accountSnapshots(request, server.origin, await accessToken(second));
    expect(returned.every((snapshot) => !snapshot.envelope.includes(edit)), "second-device edit must remain encrypted on the server").toBe(true);
    await second.close(); // each profile is independent; only one runtime needs to stay loaded
    const thirdContext = await browser.newContext({ baseURL: appOrigin, reducedMotion: "reduce" });
    try {
      const third = await thirdContext.newPage();
      await boot(third);
      await third.clock.setFixedTime(new Date());
      await connectWithPassword(third, user.username, user.password, true);
      await unlockIfNeeded(third);
      await nav(third, "/transactions");
      await waitForVisibleUnlocked(third, third.getByText(edit, { exact: true }));
    } finally {
      await thirdContext.close();
    }
  } finally {
    await secondContext.close();
  }
});
