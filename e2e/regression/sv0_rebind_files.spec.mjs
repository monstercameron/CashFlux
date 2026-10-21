import fs from "node:fs/promises";
import { test, expect, nav, boot } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accessToken, accountSnapshots, unlockIfNeeded } from "./sv0_helpers.mjs";

const image = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");
let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

for (const action of ["merge", "replace"]) {
test(`workspace recovery ${action}s an encrypted server copy with a local file`, async ({ app, request, browser }) => {
  test.setTimeout(360_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const filename = `rebind-${suffix}.png`;
  const account = { username: `sv0-rebind-${suffix}`, password: `CashFlux-R-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, account.username, account.password);
  await app.clock.setFixedTime(new Date());
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible({ timeout: 30_000 });
  await Promise.all([app.waitForEvent("load"), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true");

  await nav(app, "/artifacts");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser"),
    app.getByRole("button", { name: /upload image/i }).click(),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: image });
  await expect(app.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();

  await configureRemote(app, server.origin);
  await login(app, account.username, account.password);
  await waitForSync(app);
  await expect.poll(async () => app.evaluate(() => JSON.parse(window.cashfluxStoreGet?.("cashflux:sync-queue") || "[]").length), { timeout: 30_000 }).toBe(0);
  const token = await accessToken(app);
  const before = await accountSnapshots(request, server.origin, token);

  const localName = `local-only-${suffix}.png`;
  await app.context().setOffline(true);
  await nav(app, "/artifacts");
  const [localChooser] = await Promise.all([
    app.waitForEvent("filechooser"),
    app.getByRole("button", { name: /upload image/i }).click(),
  ]);
  await localChooser.setFiles({ name: localName, mimeType: "image/png", buffer: image });
  await expect(app.getByTestId("artifact-name").filter({ hasText: localName })).toBeVisible();
  let previousGeneration = "";
  let settled = 0;
  await expect.poll(async () => {
    const generation = await app.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset:gen") || "");
    settled = generation && generation === previousGeneration ? settled + 1 : 0;
    previousGeneration = generation;
    return settled >= 4;
  }, { timeout: 30_000, intervals: [500] }).toBe(true);

  // Simulate an older, stranded queue entry while keeping the signed-in
  // account and encrypted server snapshot real. This exercises the recovery
  // UI without changing any production account or relying on a second user.
  await app.evaluate(() => {
    const get = window.cashfluxStoreGet;
    const set = window.cashfluxStoreSet;
    const dataset = get("cashflux:dataset");
    const workspace = JSON.parse(get("cashflux:workspaces")).activeId;
    set("cashflux:sync-queue", JSON.stringify([{
      userId: "older-account", workspaceId: workspace, dataset,
      clientUpdatedAt: new Date().toISOString(), hash: "stranded-test-entry",
    }]));
  });
  await app.context().setOffline(false);
  await nav(app, "/settings");
  await app.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  const recoveryActions = ["cloud-start-encrypted-sync", "cloud-first-retry", "sync-now"];
  const actionAttempts = new Map();
  await expect.poll(async () => {
    await unlockIfNeeded(app);
    if (await app.getByTestId("sync-rebind-card").isVisible()) return true;
    for (const id of recoveryActions) {
      const button = app.getByTestId(id);
      if ((actionAttempts.get(id) || 0) < 2 && await button.isVisible() && await button.isEnabled()) {
        actionAttempts.set(id, (actionAttempts.get(id) || 0) + 1);
        await button.click({ timeout: 15_000 });
        break;
      }
    }
    return false;
  }, { timeout: 90_000, intervals: [1000] }).toBe(true);
  await app.getByTestId("rebind-start").click();
  await expect(app.getByTestId("rebind-target")).toBeVisible({ timeout: 30_000 });
  await app.getByTestId("rebind-target").selectOption("default");
  await app.getByTestId("rebind-confirm").click();
  await expect(app.getByTestId("rebind-merge-choice")).toBeVisible({ timeout: 60_000 });
  await app.getByTestId(action === "merge" ? "rebind-merge-keep-both" : "rebind-merge-replace").click();
  await expect(app.getByTestId("sync-rebind-card")).toHaveCount(0, { timeout: 60_000 });
  await expect.poll(async () => {
    const after = await accountSnapshots(request, server.origin, token);
    return after.some((snapshot) => !before.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 45_000 }).toBe(true);

  await nav(app, "/artifacts");
  const name = app.getByTestId("artifact-name").filter({ hasText: filename });
  await expect(name).toBeVisible();
  const row = app.locator(".row").filter({ has: name });
  await row.locator('[data-testid^="artifact-menu-btn-"]').click();
  const [download] = await Promise.all([
    app.waitForEvent("download"),
    row.locator('[data-testid^="artifact-download-"]').click(),
  ]);
  expect(await fs.readFile(await download.path())).toEqual(image);
  const local = app.getByTestId("artifact-name").filter({ hasText: localName });
  await expect(local).toBeVisible();
  const localRow = app.locator(".row").filter({ has: local });
  await localRow.locator('[data-testid^="artifact-menu-btn-"]').click();
  const [localDownload] = await Promise.all([
    app.waitForEvent("download"),
    localRow.locator('[data-testid^="artifact-download-"]').click(),
  ]);
  expect(await fs.readFile(await localDownload.path())).toEqual(image);

  const receivingContext = await browser.newContext({ baseURL: new URL(app.url()).origin, serviceWorkers: "block", reducedMotion: "reduce", acceptDownloads: true });
  try {
    const receiving = await receivingContext.newPage();
    await boot(receiving);
    await receiving.clock.setFixedTime(new Date());
    await configureRemote(receiving, server.origin);
    await login(receiving, account.username, account.password);
    await unlockIfNeeded(receiving);
    await nav(receiving, "/artifacts");
    const received = receiving.getByTestId("artifact-name").filter({ hasText: localName });
    await expect.poll(async () => { await unlockIfNeeded(receiving); return received.isVisible(); }, { timeout: 90_000 }).toBe(true);
    const receivedRow = receiving.locator(".row").filter({ has: received });
    let receivedDownload;
    for (let attempt = 0; attempt < 3 && !receivedDownload; attempt++) {
      try {
        await unlockIfNeeded(receiving);
        await expect(received).toBeVisible({ timeout: 30_000 });
        await receivedRow.locator('[data-testid^="artifact-menu-btn-"]').click({ timeout: 15_000 });
        const downloadButton = receivedRow.locator('[data-testid^="artifact-download-"]');
        await expect(downloadButton).toBeVisible({ timeout: 15_000 });
        [receivedDownload] = await Promise.all([
          receiving.waitForEvent("download", { timeout: 15_000 }),
          downloadButton.click({ timeout: 15_000 }),
        ]);
      } catch (error) {
        if (attempt === 2) throw error;
      }
    }
    expect(await fs.readFile(await receivedDownload.path())).toEqual(image);
  } finally {
    await receivingContext.close();
  }
});
}
