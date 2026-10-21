import { test, expect } from "@playwright/test";

const HOSTED = `http://127.0.0.1:${process.env.E2E_RPC_PORT || "8198"}`;
const ADMIN_HEADERS = { Authorization: "Bearer e2e-worker-token" };

test("hosted sign-in preserves an unreadable local copy through the recovery choice", async ({ browser, request }) => {
  test.setTimeout(180_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const username = `corrupt-signin-${suffix}`;
  const password = `CashFlux-Corrupt-${suffix}!`;
  const corrupt = `CORRUPT-LOCAL-COPY-${suffix}`;
  const createdResponse = await request.post(`${HOSTED}/v1/admin/users`, {
    headers: ADMIN_HEADERS,
    data: { username, password, role: "member" },
  });
  expect(createdResponse.status()).toBe(200);
  const created = await createdResponse.json();
  const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
  const page = await context.newPage();
  try {
    // Seed the real origin before the app loads, so the IndexedDB write cannot
    // race its boot-time read of the checkpoint.
    await page.route(`${HOSTED}/__seed_corrupt_copy__`, (route) =>
      route.fulfill({ status: 200, contentType: "text/html", body: "<!doctype html><title>seed</title>" }));
    await page.goto(`${HOSTED}/__seed_corrupt_copy__`);
    await page.evaluate((raw) => new Promise((resolve, reject) => {
      const open = indexedDB.open("cashflux-kv", 1);
      open.onupgradeneeded = () => open.result.createObjectStore("kv");
      open.onerror = () => reject(open.error);
      open.onsuccess = () => {
        const db = open.result;
        const tx = db.transaction("kv", "readwrite");
        tx.objectStore("kv").put(raw, "cashflux:dataset");
        tx.oncomplete = () => { db.close(); resolve(); };
        tx.onerror = () => reject(tx.error);
      };
    }), corrupt);
    await page.goto(`${HOSTED}/settings/cloud`);
    await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
    await expect(page.getByTestId("hosted-auth-gate")).toBeVisible();
    await expect(page.locator(".cf-shell")).toHaveCount(0);
    await expect(page.getByTestId("password-auth-username")).toBeVisible();
    await expect(page.getByTestId("password-auth-password")).toBeVisible();
    await expect.poll(() => page.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset") || "")).toBe(corrupt);

    await page.getByRole("radio", { name: "Log in", exact: true }).click();
    await page.getByTestId("password-auth-username").fill(username);
    await page.getByTestId("password-auth-password").fill(password);
    await page.getByTestId("password-auth-submit").click();
    await expect(page.getByTestId("hosted-hydration-empty-account")).toBeVisible({ timeout: 45_000 });
    await expect(page.locator(".cf-shell")).toHaveCount(0);
    await expect.poll(() => page.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset") || "")).toBe(corrupt);
  } finally {
    await context.close();
    await request.delete(`${HOSTED}/v1/admin/users/${encodeURIComponent(created.id)}`, { headers: ADMIN_HEADERS });
  }
});
