import { test, expect } from "@playwright/test";

const ORIGIN = (process.env.CASHFLUX_PRODUCTION_URL || "").replace(/\/+$/, "");
const TOKEN = process.env.CASHFLUX_PRODUCTION_TOKEN || "";
const OWNER_USERNAME = process.env.CASHFLUX_PRODUCTION_OWNER_USERNAME || "";
const OWNER_PASSWORD = process.env.CASHFLUX_PRODUCTION_OWNER_PASSWORD || "";
const ENABLED = Boolean(ORIGIN && TOKEN && OWNER_USERNAME && OWNER_PASSWORD);

const suffix = Date.now().toString(36);
const username = `prod-e2e-${suffix}`;
const password = `CashFlux-Prod-${suffix}-Old!`;
const recoveredPassword = `CashFlux-Prod-${suffix}-New!`;
const approvedUsername = `approved-${suffix}`;
const approvedPassword = `CashFlux-Approved-${suffix}!`;

const adminHeaders = { Authorization: `Bearer ${TOKEN}` };

async function waitForApp(page) {
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 60_000 },
  );
}

async function signIn(page, user, pass) {
  await page.getByRole("radio", { name: "Log in", exact: true }).click();
  await page.getByTestId("password-auth-username").fill(user);
  await page.getByTestId("password-auth-password").fill(pass);
  await page.getByTestId("password-auth-submit").click();
  await expect(page.getByTestId("hosted-auth-gate")).toHaveCount(0, { timeout: 30_000 });
  await expect(page.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", {
    timeout: 30_000,
  });
}

async function findUser(request, identity) {
  const response = await request.get(
    `${ORIGIN}/v1/admin/users?q=${encodeURIComponent(identity)}`,
    { headers: adminHeaders },
  );
  expect(response.status()).toBe(200);
  const body = await response.json();
  return body.users?.find((user) => user.username === identity || user.email === identity);
}

test.describe("opt-in production auth smoke", () => {
  test.skip(!ENABLED, "Set CASHFLUX_PRODUCTION_* to run destructive disposable-account checks.");
  test.describe.configure({ mode: "serial" });

  test("TLS, owner console, CRUD, recovery, approval, sync, and revocation", async ({
    browser,
    request,
  }) => {
    test.setTimeout(300_000);

    const createdIDs = [];
    const ready = await request.get(`${ORIGIN}/readyz`);
    expect(ready.status()).toBe(204);
    const version = await request.get(`${ORIGIN}/v1/version`);
    expect(version.status()).toBe(200);
    expect(await version.json()).toMatchObject({
      hostedApp: true,
      customAuthEnabled: true,
      registrationOpen: false,
    });

    const adminContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
    const admin = await adminContext.newPage();

    try {
      await admin.goto(`${ORIGIN}/console/`);
      await admin.getByTestId("admin-username").fill(OWNER_USERNAME);
      await admin.getByTestId("admin-password").fill(OWNER_PASSWORD);
      await admin.getByTestId("admin-credential-signin").click();
      await expect(admin.getByRole("heading", { name: "Operator Console" })).toBeVisible();
      await expect(admin.getByLabel(`Manage ${OWNER_USERNAME}`)).toBeVisible();

      const createdResponse = await request.post(`${ORIGIN}/v1/admin/users`, {
        headers: adminHeaders,
        data: { username, password, role: "member" },
      });
      expect(createdResponse.status()).toBe(200);
      const created = await createdResponse.json();
      createdIDs.push(created.id);
      expect(created.recoveryCode).toMatch(/^[A-Z2-7]{16}$/);

      const context = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
      const page = await context.newPage();
      await page.goto(`${ORIGIN}/accounts`);
      await waitForApp(page);
      await expect(page.getByTestId("hosted-auth-gate")).toBeVisible();
      await expect(page.locator(".cf-shell")).toHaveCount(0);
      await signIn(page, username, password);

      const oldRefresh = await page.evaluate(
        () => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "",
      );
      expect(oldRefresh).not.toBe("");

      await page.goto(`${ORIGIN}/settings`);
      await waitForApp(page);
      await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
      await page.getByRole("button", { name: "Sign out", exact: true }).click();
      await expect(page.getByTestId("hosted-auth-gate")).toBeVisible({ timeout: 30_000 });

      const revoked = await request.post(`${ORIGIN}/v1/auth/refresh`, {
        headers: {
          Cookie: `cashflux_refresh=${oldRefresh}; cashflux_csrf=production-e2e`,
          "X-CashFlux-CSRF": "production-e2e",
        },
      });
      expect(revoked.status()).toBe(401);

      await page.getByRole("radio", { name: "Forgot password?", exact: true }).click();
      await page.getByTestId("password-auth-username").fill(username);
      await page.getByTestId("password-auth-password").fill(recoveredPassword);
      await page.getByTestId("password-auth-recovery-input").fill(created.recoveryCode);
      await page.getByTestId("password-auth-submit").click();
      const replacement = (await page.getByTestId("password-auth-recovery-code").innerText()).trim();
      expect(replacement).toMatch(/^[A-Z2-7]{16}$/);
      expect(replacement).not.toBe(created.recoveryCode);
      await page.getByTestId("password-auth-recovery-dismiss").click();
      await expect(page.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", {
        timeout: 30_000,
      });
      await context.close();

      const approvalContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce" });
      const approval = await approvalContext.newPage();
      await approval.goto(`${ORIGIN}/accounts`);
      await waitForApp(approval);
      await approval.getByTestId("pending-device-request").click();
      await expect(approval.getByTestId("pending-device-waiting")).toBeVisible({ timeout: 30_000 });

      await admin.getByTestId("admin-pending-refresh").click();
      const pending = admin.getByTestId("admin-pending-device");
      await expect(pending).toHaveCount(1);
      await pending.getByTestId("admin-pending-approve").click();
      await expect(admin.getByTestId("admin-pending-status")).toContainText("Access approved");

      await expect(approval.getByTestId("pending-device-approved")).toBeVisible({ timeout: 30_000 });
      await approval.getByTestId("pending-device-accept").click();
      await approval.getByTestId("pending-device-username").fill(approvedUsername);
      await approval.getByTestId("pending-device-password").fill(approvedPassword);
      await approval.getByTestId("pending-device-setpassword-submit").click();
      await expect(approval.getByTestId("pending-device-recovery-code")).toBeVisible();
      await approval.getByTestId("pending-device-recovery-dismiss").click();
      await expect(approval.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", {
        timeout: 30_000,
      });
      const approved = await findUser(request, approvedUsername);
      expect(approved?.id).toBeTruthy();
      createdIDs.push(approved.id);
      await approvalContext.close();
    } finally {
      for (const identity of [username, approvedUsername]) {
        const user = await findUser(request, identity).catch(() => undefined);
        if (user?.id) {
          createdIDs.push(user.id);
        }
      }
      for (const id of [...new Set(createdIDs)].reverse()) {
        await request.delete(`${ORIGIN}/v1/admin/users/${encodeURIComponent(id)}`, {
          headers: adminHeaders,
        });
      }
      await adminContext.close();
    }
  });
});
