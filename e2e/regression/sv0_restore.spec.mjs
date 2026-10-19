import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, addUniqueTransaction, datasetHas, reloadReady } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("encrypted restore commits before reload and later edits survive", async ({ app, request }) => {
  test.setTimeout(240_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const user = { username: `sv0-restore-${suffix}`, password: `CashFlux-B-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, user.username, user.password);
  await configureRemote(app, server.origin);
  await login(app, user.username, user.password);
  await waitForSync(app);

  const baseline = `SV0-BACKUP-${suffix}`;
  await addUniqueTransaction(app, baseline);
  await waitForSync(app);
  await app.keyboard.press("Control+k");
  await expect(app.locator("#cf-cmd-palette")).toBeVisible();
  await app.locator("#cf-cmd-input").fill("encrypted backup");
  await app.getByText(/back up everything, encrypted/i).click();
  const phrase = `CashFlux backup phrase ${suffix}!`;
  const prompt = app.locator("input").last();
  await expect(prompt).toBeVisible();
  await prompt.fill(phrase);
  await prompt.press("Enter");
  const confirm = app.locator("input").last();
  await expect(confirm).toBeVisible();
  await confirm.fill(phrase);
  const [download] = await Promise.all([
    app.waitForEvent("download"),
    app.getByRole("button", { name: /continue|confirm|save/i }).last().click(),
  ]);
  expect(download.suggestedFilename()).toMatch(/\.cfbak$/i);
  const fs = await import("node:fs/promises");
  const backup = await fs.readFile(await download.path());
  expect(backup.length).toBeGreaterThan(32);

  const editBeforeRestore = `SV0-REMOVED-${suffix}`;
  await addUniqueTransaction(app, editBeforeRestore);
  await app.keyboard.press("Control+k");
  await app.locator("#cf-cmd-input").fill("restore");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser"),
    app.getByText(/restore from a backup file/i).click(),
  ]);
  await chooser.setFiles({ name: "cashflux-backup.cfbak", mimeType: "application/octet-stream", buffer: backup });
  const unlock = app.locator("input").last();
  await expect(unlock).toBeVisible();
  await unlock.fill(phrase);
  await unlock.press("Enter");
  await expect(app.locator("#cf-dialog-confirm")).toBeVisible();
  await Promise.all([app.waitForEvent("load"), app.locator("#cf-dialog-confirm").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  expect(await datasetHas(app, baseline)).toBe(true);
  expect(await datasetHas(app, editBeforeRestore)).toBe(false);

  const after = `SV0-AFTER-RESTORE-${suffix}`;
  await addUniqueTransaction(app, after);
  await waitForSync(app);
  await reloadReady(app);
  expect(await datasetHas(app, after), "post-restore edit must still exist after reload").toBe(true);
});
