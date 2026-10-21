import { test, expect, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accountSnapshots, accessToken } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("encrypted image reaches the server without making the source device stale", async ({ app, request }) => {
  test.setTimeout(180_000);
  await app.clock.setFixedTime(new Date());
  const suffix = crypto.randomUUID().slice(0, 8);
  const filename = `private-receipt-${suffix}.png`;
  const pixels = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");
  const username = `sv0-upload-${suffix}`;
  const password = `CashFlux-Upload-${suffix}-Pass!`;
  await createPasswordAccount(request, server.origin, username, password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load"), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await nav(app, "/accounts");
  await app.getByTestId("accounts-add").click();
  const form = app.getByTestId("account-add-form");
  await expect(form).toBeVisible();
  await form.locator('input[type="text"]').first().fill(`File sync ${suffix}`);
  await form.locator('input[type="number"]').first().fill("10");
  await app.locator('button[form="account-add-form"]').click();
  await configureRemote(app, server.origin);
  await login(app, username, password);
  await waitForSync(app);
  await expect.poll(async () => (await accountSnapshots(request, server.origin, await accessToken(app))).length, { timeout: 90_000 }).toBeGreaterThan(0);
  await expect(app.getByTestId("stale-dataset-banner")).toHaveCount(0);
  const before = await accountSnapshots(request, server.origin, await accessToken(app));

  await nav(app, "/artifacts");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByRole("button", { name: /upload image/i }).click({ timeout: 20_000 }),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: pixels });
  await expect(app.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();
  await expect.poll(async () => {
    const now = await accountSnapshots(request, server.origin, await accessToken(app));
    return now.some((snapshot) => !before.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 90_000 }).toBe(true);
  await expect(app.getByTestId("stale-dataset-banner")).toHaveCount(0);
  const snapshots = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(snapshots).toHaveLength(1);
  expect(snapshots[0].envelope.startsWith("\u0000cf1\u0000")).toBe(true);
  expect(snapshots[0].envelope).not.toContain(filename);
});
