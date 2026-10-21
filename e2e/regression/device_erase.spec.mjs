import { test, expect } from "@playwright/test";

const tinyPNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");

test("device erase removes real files from two locally created workspaces", async ({ browser }) => {
  test.setTimeout(150_000);
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const page = await context.newPage();
  const ready = () => page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
  const upload = async (filename) => {
    await page.goto("/artifacts");
    await ready();
    const [chooser] = await Promise.all([
      page.waitForEvent("filechooser"),
      page.getByRole("button", { name: /upload image/i }).click(),
    ]);
    await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: tinyPNG });
    await expect(page.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();
  };
  try {
    await page.goto("/");
    await ready();
    await expect(page.getByTestId("sample-start-fresh")).toBeVisible();
    await Promise.all([page.waitForEvent("load"), page.getByTestId("sample-start-fresh").click()]);
    await ready();
    const firstWorkspace = await page.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")).activeId);
    await upload("erase-first-workspace.png");

    await page.keyboard.press("Control+k");
    await page.locator("#cf-cmd-input").fill("new workspace");
    await page.getByText(/new workspace/i).last().click();
    const workspaceName = page.getByRole("dialog").getByRole("textbox", { name: "Name the new workspace" });
    await workspaceName.fill("Second erase workspace");
    await Promise.all([page.waitForEvent("load"), workspaceName.press("Enter")]);
    await ready();
    const secondWorkspace = await page.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")).activeId);
    expect(secondWorkspace).not.toBe(firstWorkspace);
    await expect.poll(() => page.evaluate((key) => !!window.cashfluxStoreGet(key), `cashflux:ws-data:${firstWorkspace}`)).toBe(true);
    await upload("erase-second-workspace.png");

    const before = await page.evaluate(async () => {
      const request = indexedDB.open("cashflux");
      const db = await new Promise((resolve, reject) => {
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      try {
        return await new Promise((resolve, reject) => {
          const count = db.transaction("artifacts", "readonly").objectStore("artifacts").count();
          count.onsuccess = () => resolve(count.result);
          count.onerror = () => reject(count.error);
        });
      } finally { db.close(); }
    });
    expect(before).toBeGreaterThanOrEqual(2);

    await page.goto("/settings");
    await ready();
    await page.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    await page.getByRole("button", { name: "Wipe data" }).click();
    await page.getByRole("button", { name: "Erase everything" }).click();
    await expect(page.getByRole("heading", { name: "This device has been erased" })).toBeVisible({ timeout: 30_000 });
    expect(await page.evaluate(async () => (await indexedDB.databases()).filter(({ name }) => name === "cashflux" || name?.startsWith("cashflux-kv")))).toEqual([]);
  } finally {
    await context.close();
  }
});

test("device erase removes files and queued data while another app tab is open", async ({ browser }) => {
  test.setTimeout(150_000);
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const first = await context.newPage();
  const second = await context.newPage();
  try {
    for (const page of [first, second]) {
      await page.goto("/");
      await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    }

    await first.evaluate(async () => {
      async function put(name, store, key, value) {
        const db = await new Promise((resolve, reject) => {
          const request = indexedDB.open(name, 1);
          request.onupgradeneeded = () => {
            if (!request.result.objectStoreNames.contains(store)) request.result.createObjectStore(store);
          };
          request.onsuccess = () => resolve(request.result);
          request.onerror = () => reject(request.error);
        });
        await new Promise((resolve, reject) => {
          const tx = db.transaction(store, "readwrite");
          tx.objectStore(store).put(value, key);
          tx.oncomplete = resolve;
          tx.onerror = () => reject(tx.error);
        });
        db.close();
      }
      await put("cashflux", "artifacts", "erase-test-file", {
        mime: "text/plain", data: new Uint8Array([83, 89, 78, 67]),
      });
      await put("cashflux-kv", "kv", "cashflux:ws-data:inactive", "synthetic inactive workspace");
      await put("cashflux-kv", "kv", "cashflux:sync-queue", "synthetic queued snapshot");
      for (const name of ["cashflux-kv-v2:other", "cashflux-atrest-keys-v1:other", "cashflux-refresh-v1:other", "cfv2:other"]) {
        await put(name, "records", "private", "synthetic private value");
      }
      localStorage.setItem("cashflux:legacy-record", "synthetic legacy data");
      localStorage.setItem("cf.artifactSalt", "synthetic legacy salt");
      const cache = await caches.open("cashflux-private-legacy");
      await cache.put(new URL("/v1/account/export", location.href), new Response("synthetic private response"));
    });

    await first.goto("/settings");
    await first.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    await first.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    await first.getByRole("button", { name: "Wipe data" }).click();
    await first.getByRole("button", { name: "Erase everything" }).click();

    await expect(second).toHaveURL(/erase-device\.html#wait/, { timeout: 30_000 });
    await expect(first.getByRole("heading", { name: "This device has been erased" })).toBeVisible({ timeout: 30_000 });
    await expect(second.getByRole("heading", { name: "This device has been erased" })).toBeVisible({ timeout: 30_000 });

    const remaining = await first.evaluate(async () => ({
      databases: (await indexedDB.databases()).map((db) => db.name).filter((name) =>
        name === "cashflux" || name === "cashflux-kv" || name?.startsWith("cashflux-kv-v2:") ||
        name?.startsWith("cashflux-atrest-keys-v1:") || name?.startsWith("cashflux-refresh-v1:") ||
        name?.startsWith("cfv2:")),
      caches: (await caches.keys()).filter((name) => name.startsWith("cashflux-")),
      localKeys: Object.keys(localStorage).filter((name) =>
        name.startsWith("cashflux:") || name === "cf.artifactSalt"),
    }));
    expect(remaining).toEqual({ databases: [], caches: [], localKeys: [] });
  } finally {
    await context.close();
  }
});

test("a controlled offline app can still open and complete device erasure", async ({ browser }) => {
  test.setTimeout(120_000);
  const context = await browser.newContext({ serviceWorkers: "allow", reducedMotion: "reduce" });
  const page = await context.newPage();
  try {
    await page.goto("/settings");
    await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    await page.waitForFunction(() => navigator.serviceWorker.controller !== null, null, { timeout: 45_000 });
    await context.setOffline(true);
    await page.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    await page.getByRole("button", { name: "Wipe data" }).click();
    await page.getByRole("button", { name: "Erase everything" }).click();
    await expect(page.getByRole("heading", { name: "This device has been erased" })).toBeVisible({ timeout: 30_000 });
  } finally {
    await context.close();
  }
});

test("a tab holding a database open prevents success until erasure is retried", async ({ browser }) => {
  test.setTimeout(120_000);
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const page = await context.newPage();
  const holder = await context.newPage();
  try {
    await page.goto("/settings");
    await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    await holder.route("**/__hold_database__", (route) =>
      route.fulfill({ status: 200, contentType: "text/html", body: "<!doctype html><title>hold</title>" }));
    await holder.goto("/__hold_database__");
    await holder.evaluate(() => new Promise((resolve, reject) => {
      const request = indexedDB.open("cashflux-kv", 1);
      request.onsuccess = () => { window.holdDatabase = request.result; resolve(); };
      request.onerror = () => reject(request.error);
    }));

    await page.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    await page.getByRole("button", { name: "Wipe data" }).click();
    await page.getByRole("button", { name: "Erase everything" }).click();
    await expect(page.getByRole("heading", { name: "Erasure is unfinished" })).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole("heading", { name: "This device has been erased" })).toHaveCount(0);

    await holder.evaluate(() => window.holdDatabase.close());
    await page.getByRole("button", { name: "Retry erasure" }).click();
    await expect(page.getByRole("heading", { name: "This device has been erased" })).toBeVisible({ timeout: 30_000 });
  } finally {
    await context.close();
  }
});

test("a waiting tab does not mistake marker removal for completed erasure", async ({ browser }) => {
  const context = await browser.newContext({ serviceWorkers: "block" });
  const writer = await context.newPage();
  const waiter = await context.newPage();
  try {
    await writer.goto("/erase-device.html");
    await writer.evaluate(() => {
      localStorage.setItem("cashflux:erase-in-progress", "synthetic pending erase");
      localStorage.setItem("cf-device-erased-v1", "1");
      localStorage.setItem("cashflux:legacy-record", "still present");
    });
    await waiter.goto("/erase-device.html#wait");
    await expect(waiter.getByRole("heading", { name: "Erasure is in progress in another tab" })).toBeVisible();
    await writer.evaluate(() => localStorage.removeItem("cashflux:erase-in-progress"));
    await expect(waiter.getByRole("heading", { name: "Erasure is unfinished" })).toBeVisible();
    await expect(waiter.getByRole("button", { name: "Retry erasure" })).toBeVisible();
  } finally {
    await context.close();
  }
});

test("a direct eraser URL cannot delete data without Settings confirmation", async ({ browser }) => {
  const context = await browser.newContext({ serviceWorkers: "block" });
  const page = await context.newPage();
  try {
    await page.goto("/erase-device.html");
    await page.evaluate(() => localStorage.setItem("cashflux:legacy-record", "keep this record"));
    await page.goto("/erase-device.html#start=guessed-ticket");
    await expect(page.getByRole("heading", { name: "No erasure is running" })).toBeVisible();
    expect(await page.evaluate(() => localStorage.getItem("cashflux:legacy-record"))).toBe("keep this record");
    await page.evaluate(() => localStorage.setItem("cashflux:erase-in-progress", "private-ticket"));
    await page.reload();
    await expect(page.getByRole("heading", { name: "Erasure is in progress in another tab" })).toBeVisible();
    expect(await page.evaluate(() => localStorage.getItem("cashflux:legacy-record"))).toBe("keep this record");
  } finally {
    await context.close();
  }
});
