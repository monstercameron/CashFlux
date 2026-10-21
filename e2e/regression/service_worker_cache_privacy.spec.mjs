import { test, expect } from "@playwright/test";

test.use({ serviceWorkers: "allow" });

test("service worker does not persist a same-origin API response", async ({ page }) => {
  test.setTimeout(120_000);
  await page.goto("/");
  await page.evaluate(async () => {
    await navigator.serviceWorker.register("/sw.js?build=dev");
    await navigator.serviceWorker.ready;
  });
  await page.reload();
  await page.waitForFunction(() => !!navigator.serviceWorker.controller);

  // The local static test server answers this extensionless path with the SPA
  // document (HTTP 200). The old worker would write that successful response
  // under an API URL. The new worker must leave it outside CacheStorage.
  const result = await page.evaluate(async () => {
    const url = new URL("/v1/account/export", location.href).href;
    const response = await fetch(url);
    let cached = false;
    for (const name of await caches.keys()) {
      if (!name.startsWith("cashflux-")) continue;
      const cache = await caches.open(name);
      if (await cache.match(url)) cached = true;
    }
    return { status: response.status, cached };
  });
  expect(result.status).toBe(200);
  expect(result.cached).toBe(false);
});
