import { test, expect } from "@playwright/test";
import { startServer, APP_ORIGIN } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("corrupt saved bytes are quarantined without seeding over the original", async ({ browser }) => {
  test.setTimeout(120_000);
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const page = await context.newPage();
  const corrupt = "SV0-CORRUPT-BYTES-KEEP-EXACTLY";
  // The init script opens the real browserstore database before application
  // scripts run. IndexedDB serializes the put ahead of the app's own read.
  await page.addInitScript((raw) => {
    const request = indexedDB.open("cashflux-kv", 1);
    request.onupgradeneeded = () => request.result.createObjectStore("kv");
    request.onsuccess = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains("kv")) {
        db.close();
        return;
      }
      db.transaction("kv", "readwrite").objectStore("kv").put(raw, "cashflux:dataset");
    };
  }, corrupt);
  await page.goto(APP_ORIGIN);
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });

  await expect(page.getByText(/couldn't read your saved data/i)).toBeVisible({ timeout: 20_000 });
  const stored = await page.evaluate(() => new Promise((resolve, reject) => {
    const request = indexedDB.open("cashflux-kv");
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const tx = request.result.transaction("kv", "readonly");
      const get = tx.objectStore("kv").get("cashflux:dataset");
      get.onsuccess = () => resolve(get.result);
      get.onerror = () => reject(get.error);
    };
  }));
  expect(stored).toBe(corrupt);
  const quarantined = await page.evaluate(() => new Promise((resolve, reject) => {
    const request = indexedDB.open("cashflux-kv");
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const tx = request.result.transaction("kv", "readonly");
      const keys = tx.objectStore("kv").getAllKeys();
      keys.onsuccess = () => resolve(keys.result.map(String).filter((key) => key.startsWith("cashflux:quarantine:")));
      keys.onerror = () => reject(keys.error);
    };
  }));
  expect(quarantined.length).toBeGreaterThan(0);
  await context.close();
});
