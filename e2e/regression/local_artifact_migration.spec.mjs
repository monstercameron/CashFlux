import { test, expect, nav } from "./fixtures.mjs";

async function artifactRecord(page, id) {
  return page.evaluate(async (key) => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open("cashflux", 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    try {
      return await new Promise((resolve, reject) => {
        const request = db.transaction("artifacts", "readonly").objectStore("artifacts").get(key);
        request.onsuccess = () => resolve(request.result ? { mime: request.result.mime, data: Array.from(request.result.data) } : null);
        request.onerror = () => reject(request.error);
      });
    } finally {
      db.close();
    }
  }, id);
}

async function protectedStoredValue(page, key) {
  return page.evaluate(async (recordKey) => {
    for (const { name } of await indexedDB.databases()) {
      if (!name?.startsWith("cashflux-kv")) continue;
      const db = await new Promise((resolve, reject) => {
        const request = indexedDB.open(name);
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      try {
        if (!db.objectStoreNames.contains("kv")) continue;
        const value = await new Promise((resolve, reject) => {
          const request = db.transaction("kv", "readonly").objectStore("kv").get(recordKey);
          request.onsuccess = () => resolve(request.result);
          request.onerror = () => reject(request.error);
        });
        if (value !== undefined) return value;
      } finally {
        db.close();
      }
    }
    return null;
  }, key);
}

test("protected boot migrates a legacy file in an inactive workspace", async ({ app }) => {
  test.setTimeout(120_000);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible();
  await Promise.all([app.waitForEvent("load"), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true");
  const originalWorkspaceID = await app.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")).activeId);
  const pixels = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");
  const filename = `legacy-inactive-${crypto.randomUUID().slice(0, 8)}.png`;
  await nav(app, "/artifacts");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser"),
    app.getByRole("button", { name: /upload image/i }).click(),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: pixels });
  await expect(app.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();
  const row = app.locator(".row").filter({ has: app.getByTestId("artifact-name").filter({ hasText: filename }) });
  const menuID = await row.locator('[data-testid^="artifact-menu-btn-"]').getAttribute("data-testid");
  const id = menuID.slice("artifact-menu-btn-".length);

  await app.keyboard.press("Control+k");
  await expect(app.locator("#cf-cmd-palette")).toBeVisible();
  await app.locator("#cf-cmd-input").fill("new workspace");
  await app.getByText(/new workspace/i).last().click();
  const prompt = app.getByRole("dialog").getByRole("textbox", { name: "Name the new workspace" });
  await expect(prompt).toBeVisible();
  await prompt.fill("Migration destination");
  await Promise.all([app.waitForEvent("load"), prompt.press("Enter")]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true");

  // Model an old external file while its owning workspace is inactive: its
  // portable snapshot holds metadata, and the separate IDB record holds bytes.
  const bundleKey = `cashflux:ws-data:${originalWorkspaceID}`;
  await expect.poll(() => app.evaluate((key) => !!window.cashfluxStoreGet(key), bundleKey)).toBe(true);
  const previousBundleRecord = await protectedStoredValue(app, bundleKey);
  await app.evaluate(({ key, artifactID }) => {
    const bundle = JSON.parse(window.cashfluxStoreGet(key));
    const dataset = JSON.parse(bundle["cashflux:dataset"]);
    const artifact = dataset.artifacts.find((entry) => entry.id === artifactID);
    artifact.bytes = null;
    bundle["cashflux:dataset"] = JSON.stringify(dataset);
    window.cashfluxStoreSet(key, JSON.stringify(bundle));
  }, { key: bundleKey, artifactID: id });
  await expect.poll(() => protectedStoredValue(app, bundleKey), { timeout: 20_000 }).not.toBe(previousBundleRecord);
  await app.evaluate(async ({ key, bytes }) => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open("cashflux", 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    try {
      await new Promise((resolve, reject) => {
        const tx = db.transaction("artifacts", "readwrite");
        tx.objectStore("artifacts").put({ mime: "image/png", data: new Uint8Array(bytes) }, key);
        tx.oncomplete = resolve;
        tx.onabort = () => reject(tx.error);
      });
    } finally {
      db.close();
    }
  }, { key: id, bytes: [...pixels] });
  expect((await artifactRecord(app, id)).mime).toBe("image/png");
  await app.reload();
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true");
  await expect.poll(async () => (await artifactRecord(app, id)).mime, { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
  const saved = await artifactRecord(app, id);
  expect(Buffer.from(saved.data)).not.toEqual(pixels);
});
