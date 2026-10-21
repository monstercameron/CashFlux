import { readFile } from "node:fs/promises";
import { test, expect, boot, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accountSnapshots, accessToken } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

async function connectWithPassword(page, username, password) {
  await configureRemote(page, server.origin);
  await login(page, username, password);
}

test("encrypted image attachment uploads and restores to a second device", async ({ app, browser, request }) => {
  test.setTimeout(360_000);
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
  await expect(app.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();
  await expect.poll(async () => {
    const found = await accountSnapshots(request, server.origin, await accessToken(app));
    return found.some((snapshot) => !beforeUpload.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 90_000 }).toBe(true);
  const serverCopies = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(serverCopies.every((snapshot) => !snapshot.envelope.includes(filename)), "server snapshot must not expose the filename").toBe(true);

  const secondContext = await browser.newContext({ baseURL: new URL(app.url()).origin, reducedMotion: "reduce" });
  const second = await secondContext.newPage();
  try {
    await boot(second);
    await second.clock.setFixedTime(new Date());
    await connectWithPassword(second, user.username, user.password);
    await nav(second, "/artifacts");
    await expect(second.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible({ timeout: 45_000 });
    const row = second.locator(".row").filter({ has: second.getByTestId("artifact-name").filter({ hasText: filename }) });
    await row.locator('[data-testid^="artifact-menu-btn-"]').click();
    const [download] = await Promise.all([
      second.waitForEvent("download"),
      row.locator('[data-testid^="artifact-download-"]').click(),
    ]);
    expect(await readFile(await download.path())).toEqual(pixels);
  } finally {
    await secondContext.close();
  }
});
