import { test, expect } from "@playwright/test";

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const HOSTED = `http://127.0.0.1:${RPC_PORT}`;
const ADMIN_HEADERS = { Authorization: "Bearer e2e-worker-token" };
const PIN = "CashFlux-Lock-7284";
const WRONG_PIN = "CashFlux-Wrong-5193";

async function waitForApp(page) {
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 60_000 },
  );
}

async function route(page, path) {
  await page.evaluate((next) => {
    history.pushState({}, "", next);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await expect(page.locator("#main")).toHaveAttribute("data-route", path.startsWith("/settings") ? "/settings" : path);
}

async function login(page, username, password) {
  await page.getByRole("radio", { name: "Log in", exact: true }).click();
  await page.getByTestId("password-auth-username").fill(username);
  await page.getByTestId("password-auth-password").fill(password);
  await page.getByTestId("password-auth-submit").click();
}

test.describe("hosted first-boot hydration", () => {
  test("production-style boot never seeds and requires App Lock before decrypting existing data", async ({
    browser,
    request,
  }) => {
    test.setTimeout(240_000);
    const suffix = Date.now().toString(36);
    const username = `hosted-hydration-${suffix}`;
    const password = `CashFlux-Hydration-${suffix}!`;
    const accountName = `Synced encrypted account ${suffix}`;

    const createdResponse = await request.post(`${HOSTED}/v1/admin/users`, {
      headers: ADMIN_HEADERS,
      data: { username, password, role: "member" },
    });
    expect(createdResponse.status()).toBe(200);
    const created = await createdResponse.json();

    const firstContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
    const secondContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
    const first = await firstContext.newPage();
    const second = await secondContext.newPage();

    try {
      await first.goto(`${HOSTED}/accounts`);
      await waitForApp(first);
      await expect(first.getByTestId("hosted-auth-gate")).toBeVisible();
      await expect(first.locator(".cf-shell")).toHaveCount(0);
      await expect(first.getByTestId("sample-data-banner")).toHaveCount(0);
      expect(await first.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset") || "")).toBe("");

      await login(first, username, password);
      await expect(first.getByTestId("hosted-hydration-gate")).toHaveCount(0, { timeout: 30_000 });
      await expect(first.locator(".cf-shell")).toBeVisible();
      await expect(first.getByTestId("sample-data-banner")).toHaveCount(0);

      await first.getByTestId("accounts-add").click();
      const form = first.getByTestId("account-add-form");
      await form.locator('input[type="text"]').first().fill(accountName);
      await form.locator('input[type="number"]').first().fill("2468.13");
      await first.locator('button[form="account-add-form"]').click();
      await expect(first.locator("#main")).toContainText(accountName);

      await route(first, "/settings");
      await first.locator(".set-tab-strip").getByText("Advanced", { exact: true }).click();
      await first.getByText(/Set passcode lock/i).first().click();
      await first.locator("#cf-al-pass").fill(PIN);
      await first.locator("#cf-al-confirm").fill(PIN);
      const beforeMeta = JSON.parse(
        (await first.evaluate(() => window.cashfluxStoreGet?.("cashflux:sync-meta:default") || "{}")),
      );
      await first.locator("#cf-al-ok").click();
      await first.locator("#cf-applock-setup").waitFor({ state: "hidden", timeout: 20_000 });
      await expect.poll(async () => {
        const raw = await first.evaluate(() => window.cashfluxStoreGet?.("cashflux:sync-meta:default") || "{}");
        return JSON.parse(raw).version || 0;
      }, { timeout: 45_000 }).toBeGreaterThan(beforeMeta.version || 0);

      await second.goto(`${HOSTED}/accounts`);
      await waitForApp(second);
      await expect(second.getByTestId("sample-data-banner")).toHaveCount(0);
      expect(await second.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset") || "")).toBe("");
      await login(second, username, password);

      const lockGate = second.getByTestId("hosted-hydration-lock");
      await expect(lockGate).toBeVisible({ timeout: 45_000 });
      await expect(second.locator(".cf-shell")).toHaveCount(0);
      await expect(second.getByTestId("sample-data-banner")).toHaveCount(0);

      await second.getByTestId("hosted-hydration-passcode").fill(WRONG_PIN);
      await second.getByTestId("hosted-hydration-confirm").fill(WRONG_PIN);
      await second.getByTestId("hosted-hydration-enable-lock").click();
      await expect(lockGate).toContainText("did not match", { timeout: 30_000 });
      expect(await second.evaluate(() => window.cashfluxStoreGet?.("cashflux:applock") || "")).toBe("");

      await second.getByTestId("hosted-hydration-passcode").fill(PIN);
      await second.getByTestId("hosted-hydration-confirm").fill(PIN);
      await second.getByTestId("hosted-hydration-enable-lock").click();
      await expect(second.getByTestId("hosted-hydration-gate")).toHaveCount(0, { timeout: 60_000 });
      await expect(second.locator(".cf-shell")).toBeVisible();
      await expect(second.locator("#main")).toContainText(accountName);
      await expect(second.getByTestId("sample-data-banner")).toHaveCount(0);

      const savedLock = await second.evaluate(() => window.cashfluxStoreGet?.("cashflux:applock") || "");
      expect(JSON.parse(savedLock).enabled).toBe(true);
      const savedDataset = await second.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset") || "");
      expect(savedDataset.startsWith("\u0000cf1\u0000")).toBe(true);
    } finally {
      await firstContext.close();
      await secondContext.close();
      await request.delete(`${HOSTED}/v1/admin/users/${encodeURIComponent(created.id)}`, {
        headers: ADMIN_HEADERS,
      });
    }
  });
});
