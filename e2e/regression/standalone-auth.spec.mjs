import { test, expect, nav } from "./fixtures.mjs";

const RPC_PORT = process.env.E2E_RPC_PORT || "8198";
const BACKEND = `http://127.0.0.1:${RPC_PORT}`;
const APP_ORIGIN = process.env.E2E_BASE_URL || `http://127.0.0.1:${process.env.E2E_PORT || "8099"}`;
const USERNAME = "auth-e2e-user";
const RENAMED_USERNAME = "auth-e2e-renamed";
const OLD_PASSWORD = "CashFlux-E2E-Old-7294";
const NEW_PASSWORD = "CashFlux-E2E-New-8427";
const APPROVED_USERNAME = "approved-e2e-user";
const APPROVED_PASSWORD = "CashFlux-Approved-3917";
const APPROVED_NEW_PASSWORD = "CashFlux-Approved-New-6248";
const HOSTED_USERNAME = "hosted-e2e-user";
const HOSTED_PASSWORD = "CashFlux-Hosted-5732";
const HOSTED_NEW_PASSWORD = "CashFlux-Hosted-New-6843";

async function openCloud(page) {
  await nav(page, "/settings");
  await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  const remote = page.locator(".cloud-tab button", { hasText: "Remote" });
  for (let attempt = 0; attempt < 3 && !(await remote.isVisible()); attempt += 1) {
    const toggle = page.locator('[role="switch"]').first();
    if ((await toggle.getAttribute("aria-checked")) !== "true") {
      await toggle.press("Space", { timeout: 10_000 });
    }
    await expect(remote).toBeVisible({ timeout: 5_000 }).catch(() => {});
  }
  await expect(remote).toBeVisible();
  await remote.click({ force: true, timeout: 10_000 });
  await page.getByTestId("sync-server-url").fill(`${BACKEND}/`);
  await expect(page.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
}

async function openPasswordForm(page) {
  const expand = page.getByTestId("password-auth-expand");
  const card = page.getByTestId("password-auth-card");
  if (!(await expand.or(card).isVisible())) {
    await page.getByTestId("sync-advanced-token-toggle").click();
  }
  await expect(expand.or(card)).toBeVisible();
  if (await expand.isVisible()) {
    await expand.click();
  }
  await expect(card).toBeVisible();
}

async function rediscoverBackend(page) {
  // Re-selecting Remote deliberately restarts discovery with the persisted
  // address. Clearing/refilling the field would launch an empty-address probe
  // that can race and overwrite the successful result.
  await page.locator(".cloud-tab button", { hasText: "Remote" }).click({ force: true, timeout: 10_000 });
  await expect(page.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
}

async function choosePasswordMode(page, name) {
  const option = page.getByRole("radio", { name, exact: true });
  let lastError;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    try {
      await expect(option).toBeVisible({ timeout: 5_000 });
      if ((await option.getAttribute("aria-checked")) !== "true") {
        await option.click({ force: true, timeout: 5_000 });
        await expect(option).toHaveAttribute("aria-checked", "true", { timeout: 2_000 });
      }
      return;
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}

async function fillStable(locator, value) {
  let lastError;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    try {
      await locator.fill(value, { force: true, timeout: 5_000 });
      await expect(locator).toHaveValue(value, { timeout: 2_000 });
      return;
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}

async function submitPassword(page) {
  const submit = page.getByTestId("password-auth-submit");
  const outcome = page
    .getByRole("button", { name: "Sign out", exact: true })
    .or(page.getByTestId("password-auth-recovery-code"))
    .or(page.locator(".toast-err"));
  let lastError;
  for (let attempt = 0; attempt < 5; attempt += 1) {
    try {
      await submit.dispatchEvent("click", undefined, { timeout: 5_000 });
      await expect(outcome).toBeVisible({ timeout: 5_000 });
      return;
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}

async function passwordLogin(page, username, password) {
  await choosePasswordMode(page, "Log in");
  await fillStable(page.getByTestId("password-auth-username"), username);
  await fillStable(page.getByTestId("password-auth-password"), password);
  await submitPassword(page);
}

async function passwordSignOut(page) {
  const button = page.getByRole("button", { name: "Sign out", exact: true });
  await button.click();
  await expect(button).toHaveCount(0);
}

async function expectAuthFailure(page) {
  const toast = page.locator(".toast-err");
  await expect(toast).toBeVisible({ timeout: 20_000 });
  await expect(toast.locator(".toast-msg")).not.toHaveText("");
  await toast.getByRole("button", { name: "Dismiss" }).click({ force: true });
  await expect(toast).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Sign out", exact: true })).toHaveCount(0);
}

test.describe("standalone basic-auth lifecycle", () => {
  test.describe.configure({ mode: "serial" });

  test("operator CRUD, login, recovery, sync, suspension, and deletion work end to end", async ({
    app,
    browser,
    request,
  }) => {
    test.setTimeout(360_000);

    const admin = await app.context().newPage();
    const browserErrors = [];
    for (const page of [app, admin]) {
      page.on("pageerror", (error) => browserErrors.push(`page: ${error.message}`));
      page.on("console", (message) => {
        if (message.type() === "error" && !message.text().startsWith("Failed to load resource")) {
          browserErrors.push(`console: ${message.text()}`);
        }
      });
    }
    try {
      await admin.goto(`${BACKEND}/console/`);
      await expect(admin.getByTestId("admin-credential-signin")).toBeVisible();
      await admin.getByTestId("admin-break-glass-toggle").click();
      await admin.getByTestId("admin-break-glass-token").fill("e2e-worker-token");
      await admin.getByTestId("admin-break-glass-signin").click();
      await expect(admin.getByRole("heading", { name: "Operator Console" })).toBeVisible();

      await admin.getByRole("button", { name: "Create a user account" }).click();
      await admin.getByLabel("Username").fill(USERNAME);
      await admin.getByLabel("Temporary password").fill(OLD_PASSWORD);
      await admin.getByLabel("Role").selectOption("viewer");
      await admin.getByRole("button", { name: "Create account" }).click();
      const initialRecovery = (await admin.getByLabel("One-time recovery code").innerText()).trim();
      expect(initialRecovery).toMatch(/^[A-Z2-7]{16}$/);
      await admin.getByRole("button", { name: "I saved the recovery code" }).click();

      // A fresh backend has exactly this one user. Open it, then prove both
      // editable identity fields persist before the user signs into CashFlux.
      await expect(admin.getByRole("columnheader", { name: "Account" })).toBeVisible();
      await expect(admin.getByLabel(`Manage ${USERNAME}`)).toBeVisible();
      await admin.getByLabel(`Manage ${USERNAME}`).click();
      await admin.locator("#username-input").fill(RENAMED_USERNAME);
      await admin.locator("#role-input").selectOption("member");
      await admin.getByRole("button", { name: "Save account" }).click();
      await expect(admin.locator(".status-banner")).toContainText("Account updated");
      await expect(admin.locator(".detail-card")).toContainText(RENAMED_USERNAME);
      await expect(admin.locator(".detail-card")).toContainText("member");

      // Promote this account, prove the normal owner credential session can
      // operate the console, then return to the static break-glass path for
      // the destructive lifecycle checks below.
      await admin.locator("#role-input").selectOption("owner");
      await admin.getByRole("button", { name: "Save account" }).click();
      await expect.poll(async () => {
        const response = await request.get(`${BACKEND}/v1/admin/users?q=${encodeURIComponent(RENAMED_USERNAME)}`, {
          headers: { Authorization: "Bearer e2e-worker-token" },
        });
        const body = await response.json();
        return body.users?.[0]?.role || "";
      }).toBe("owner");
      await admin.getByRole("button", { name: "Back to console" }).click();
      await admin.getByRole("button", { name: "Sign out and return to home" }).click();
      await expect(admin.getByTestId("admin-credential-signin")).toBeVisible();
      await admin.getByTestId("admin-username").fill(RENAMED_USERNAME);
      await admin.getByTestId("admin-password").fill(OLD_PASSWORD);
      await admin.getByTestId("admin-credential-signin").click();
      await expect(admin.getByRole("heading", { name: "Operator Console" })).toBeVisible();
      await admin.getByRole("button", { name: "Sign out and return to home" }).click();
      await admin.getByTestId("admin-break-glass-toggle").click();
      await admin.getByTestId("admin-break-glass-token").fill("e2e-worker-token");
      await admin.getByTestId("admin-break-glass-signin").click();
      await expect(admin.getByRole("heading", { name: "Operator Console" })).toBeVisible();
      await admin.getByLabel(`Manage ${RENAMED_USERNAME}`).click();
      await expect(admin.getByRole("button", { name: "Suspend account" })).toBeVisible();

      // Remove the protected demo first so the new account has a real local
      // dataset to seed. The account is intentionally empty on the backend.
      const startFresh = app.getByTestId("sample-start-fresh");
      await expect(startFresh).toBeVisible();
      if (await startFresh.count()) {
        await Promise.all([
          app.waitForNavigation({ waitUntil: "domcontentloaded", timeout: 45_000 }),
          startFresh.click(),
        ]);
        await app.waitForFunction(
          () => document.documentElement.getAttribute("data-app-ready") === "true",
          null,
          { timeout: 45_000 },
        );
        await expect(app.getByTestId("sample-data-banner")).toHaveCount(0);
      }
      await openCloud(app);
      await openPasswordForm(app);
      await passwordLogin(app, RENAMED_USERNAME, OLD_PASSWORD);
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();

      // A missing workspace is the expected first-sync seed case, not an
      // error. Require the real queue to settle and reject the old symptom.
      const pulse = app.getByTestId("sync-pulse");
      await expect(pulse).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });
      await expect(app.getByTestId("sync-status-card")).not.toContainText("workspace not found");

      // Keep a second device signed in while this device recovers the account.
      // ResetPassword must revoke that independently-issued refresh family.
      const otherContext = await browser.newContext({
        reducedMotion: "reduce",
        serviceWorkers: "block",
      });
      const other = await otherContext.newPage();
      await other.goto(APP_ORIGIN);
      await other.waitForFunction(
        () => document.documentElement.getAttribute("data-app-ready") === "true",
        null,
        { timeout: 45_000 },
      );
      await openCloud(other);
      await openPasswordForm(other);
      await passwordLogin(other, RENAMED_USERNAME, OLD_PASSWORD);
      const oldDeviceRefresh = await other.evaluate(
        () => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "",
      );
      expect(oldDeviceRefresh).not.toBe("");
      await otherContext.close();

      await passwordSignOut(app);
      await rediscoverBackend(app);
      await openPasswordForm(app);
      await choosePasswordMode(app, "Forgot password?");
      await fillStable(app.getByTestId("password-auth-username"), RENAMED_USERNAME);
      await fillStable(app.getByTestId("password-auth-password"), NEW_PASSWORD);
      await fillStable(app.getByTestId("password-auth-recovery-input"), initialRecovery);
      await submitPassword(app);
      const replacementRecovery = (await app.getByTestId("password-auth-recovery-code").innerText()).trim();
      expect(replacementRecovery).toMatch(/^[A-Z2-7]{16}$/);
      expect(replacementRecovery).not.toBe(initialRecovery);
      await app.getByTestId("password-auth-recovery-dismiss").click();

      const revokedSession = await request.post(`${BACKEND}/v1/auth/refresh`, {
        headers: {
          Cookie: `cashflux_refresh=${oldDeviceRefresh}; cashflux_csrf=e2e-csrf`,
          "X-CashFlux-CSRF": "e2e-csrf",
        },
      });
      expect(revokedSession.status()).toBe(401);

      await passwordSignOut(app);
      await rediscoverBackend(app);
      await openPasswordForm(app);
      await passwordLogin(app, RENAMED_USERNAME, OLD_PASSWORD);
      await expectAuthFailure(app);

      // The consumed code also fails. Returning to login with the new
      // password proves recovery left the account usable.
      await choosePasswordMode(app, "Forgot password?");
      await fillStable(app.getByTestId("password-auth-password"), "CashFlux-E2E-Other-5186");
      await fillStable(app.getByTestId("password-auth-recovery-input"), initialRecovery);
      await submitPassword(app);
      await expectAuthFailure(app);
      await passwordLogin(app, RENAMED_USERNAME, NEW_PASSWORD);
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();

      await passwordSignOut(app);
      await rediscoverBackend(app);
      await admin.getByRole("button", { name: "Suspend account" }).click();
      await expect(admin.locator(".status-banner")).toContainText("Account suspended");
      await openPasswordForm(app);
      await passwordLogin(app, RENAMED_USERNAME, NEW_PASSWORD);
      await expectAuthFailure(app);

      await admin.getByRole("button", { name: "Reinstate account" }).click();
      await expect(admin.locator(".status-banner")).toContainText("Account reinstated");
      await passwordLogin(app, RENAMED_USERNAME, NEW_PASSWORD);
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
      await passwordSignOut(app);
      await rediscoverBackend(app);

      await admin.getByRole("button", { name: "Delete this account" }).click();
      await admin.getByRole("button", { name: "Yes, delete" }).click();
      await expect(admin.getByRole("heading", { name: "Operator Console" })).toBeVisible();
      await expect(admin.locator("tr.user-row")).toHaveCount(0);
      await openPasswordForm(app);
      await passwordLogin(app, RENAMED_USERNAME, NEW_PASSWORD);
      await expectAuthFailure(app);

      // Closed registration is the full server's safe default. The password
      // fallback still supports login/recovery for existing accounts, but it
      // must not advertise a direct Create account path.
      await expect(app.getByRole("radio", { name: "Create account", exact: true })).toHaveCount(0);

      // A first-time client explicitly requests access. Merely opening Cloud
      // did not create a request; this click is the consent boundary.
      await app.getByTestId("pending-device-request").click();
      await expect(app.getByTestId("pending-device-waiting")).toBeVisible({ timeout: 20_000 });
      await admin.getByTestId("admin-pending-refresh").click();
      const requestRow = admin.getByTestId("admin-pending-device");
      await expect(requestRow).toHaveCount(1);
      await requestRow.getByTestId("admin-pending-approve").click();
      await expect(admin.getByTestId("admin-pending-status")).toContainText("Access approved");

      await expect(app.getByTestId("pending-device-approved")).toBeVisible({ timeout: 20_000 });
      await app.getByTestId("pending-device-accept").click();
      await expect(app.getByTestId("pending-device-setpassword")).toBeVisible({ timeout: 20_000 });
      await fillStable(app.getByTestId("pending-device-username"), APPROVED_USERNAME);
      await fillStable(app.getByTestId("pending-device-password"), APPROVED_PASSWORD);
      await app.getByTestId("pending-device-setpassword-submit").click();
      const approvedRecovery = (await app.getByTestId("pending-device-recovery-code").innerText()).trim();
      expect(approvedRecovery).toMatch(/^[A-Z2-7]{16}$/);

      // Until the recovery code is acknowledged, the provisional session is
      // memory-only and neither login state nor sync may start.
      expect(await app.evaluate(() => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "")).toBe("");
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toHaveCount(0);
      await app.getByTestId("pending-device-recovery-dismiss").click();
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
      await expect(app.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });

      // The approved account is now a normal recoverable account.
      await passwordSignOut(app);
      await rediscoverBackend(app);
      await openPasswordForm(app);
      await choosePasswordMode(app, "Forgot password?");
      await fillStable(app.getByTestId("password-auth-username"), APPROVED_USERNAME);
      await fillStable(app.getByTestId("password-auth-password"), APPROVED_NEW_PASSWORD);
      await fillStable(app.getByTestId("password-auth-recovery-input"), approvedRecovery);
      await submitPassword(app);
      await expect(app.getByTestId("password-auth-recovery-code")).toBeVisible();
      await app.getByTestId("password-auth-recovery-dismiss").click();
      await passwordSignOut(app);
      await rediscoverBackend(app);
      await openPasswordForm(app);
      await passwordLogin(app, APPROVED_USERNAME, APPROVED_NEW_PASSWORD);
      await expect(app.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();

      // Rejection creates no second account or session.
      const rejectedContext = await browser.newContext({
        reducedMotion: "reduce",
        serviceWorkers: "block",
      });
      const rejected = await rejectedContext.newPage();
      await rejected.goto(APP_ORIGIN);
      await rejected.waitForFunction(
        () => document.documentElement.getAttribute("data-app-ready") === "true",
        null,
        { timeout: 45_000 },
      );
      await openCloud(rejected);
      await rejected.getByTestId("pending-device-request").click();
      await expect(rejected.getByTestId("pending-device-waiting")).toBeVisible({ timeout: 20_000 });
      await admin.getByTestId("admin-pending-refresh").click();
      await expect(admin.getByTestId("admin-pending-device")).toHaveCount(1);
      await admin.getByTestId("admin-pending-reject").click();
      await expect(rejected.getByTestId("pending-device-rejected")).toBeVisible({ timeout: 20_000 });
      expect(await rejected.evaluate(() => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "")).toBe("");
      await rejectedContext.close();

      // The approved account is the only survivor; remove it so the lifecycle
      // ends with the disposable server empty again.
      await admin.getByRole("button", { name: "Refresh console data" }).click();
      await expect(admin.locator("tr.user-row")).toHaveCount(1);
      await admin.locator("tr.user-row").first().click();
      await admin.getByRole("button", { name: "Delete this account" }).click();
      await admin.getByRole("button", { name: "Yes, delete" }).click();
      await expect(admin.locator("tr.user-row")).toHaveCount(0);
      expect(browserErrors).toEqual([]);
    } finally {
      await admin.close();
    }
  });

  test("server-hosted CashFlux gates every route and revokes password sessions on sign-out", async ({
    browser,
    request,
  }) => {
    test.setTimeout(180_000);

    const createdResponse = await request.post(`${BACKEND}/v1/admin/users`, {
      headers: { Authorization: "Bearer e2e-worker-token" },
      data: { username: HOSTED_USERNAME, password: HOSTED_PASSWORD, role: "member" },
    });
    expect(createdResponse.status()).toBe(200);
    const created = await createdResponse.json();
    expect(created.recoveryCode).toMatch(/^[A-Z2-7]{16}$/);

    const context = await browser.newContext({
      reducedMotion: "reduce",
      serviceWorkers: "block",
    });
    const page = await context.newPage();
    const browserErrors = [];
    page.on("pageerror", (error) => browserErrors.push(`page: ${error.message}`));
    page.on("console", (message) => {
      if (message.type() === "error" && !message.text().startsWith("Failed to load resource")) {
        browserErrors.push(`console: ${message.text()}`);
      }
    });

    try {
      await page.goto(BACKEND);
      await page.waitForFunction(
        () => document.documentElement.getAttribute("data-app-ready") === "true",
        null,
        { timeout: 45_000 },
      );
      await expect(page.getByTestId("hosted-auth-gate")).toBeVisible();
      await expect(page.locator(".cf-shell")).toHaveCount(0);
      await expect(page.getByTestId("sync-pulse")).toHaveCount(0);
      expect(await page.evaluate(() => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "")).toBe("");

      await choosePasswordMode(page, "Log in");
      await fillStable(page.getByTestId("password-auth-username"), HOSTED_USERNAME);
      await fillStable(page.getByTestId("password-auth-password"), HOSTED_PASSWORD);
      await page.getByTestId("password-auth-submit").click();
      await expect(page.getByTestId("hosted-auth-gate")).toHaveCount(0, { timeout: 30_000 });
      await expect(page.locator("#main")).toBeVisible();
      await expect(page.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });

      const signedOutRefresh = await page.evaluate(
        () => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "",
      );
      expect(signedOutRefresh).not.toBe("");

      await nav(page, "/settings");
      await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
      await expect(page.locator(".cloud-tab")).toBeVisible();
      await page.getByRole("button", { name: "Sign out", exact: true }).click();
      await expect(page.getByTestId("hosted-auth-gate")).toBeVisible({ timeout: 30_000 });
      await expect(page.locator(".cf-shell")).toHaveCount(0);

      const revokedSession = await request.post(`${BACKEND}/v1/auth/refresh`, {
        headers: {
          Cookie: `cashflux_refresh=${signedOutRefresh}; cashflux_csrf=e2e-csrf`,
          "X-CashFlux-CSRF": "e2e-csrf",
        },
      });
      expect(revokedSession.status()).toBe(401);

      await choosePasswordMode(page, "Forgot password?");
      await fillStable(page.getByTestId("password-auth-username"), HOSTED_USERNAME);
      await fillStable(page.getByTestId("password-auth-password"), HOSTED_NEW_PASSWORD);
      await fillStable(page.getByTestId("password-auth-recovery-input"), created.recoveryCode);
      await page.getByTestId("password-auth-submit").click();
      const replacement = (await page.getByTestId("password-auth-recovery-code").innerText()).trim();
      expect(replacement).toMatch(/^[A-Z2-7]{16}$/);
      expect(replacement).not.toBe(created.recoveryCode);
      await page.getByTestId("password-auth-recovery-dismiss").click();
      await expect(page.getByTestId("hosted-auth-gate")).toHaveCount(0, { timeout: 30_000 });
      await expect(page.locator("#main")).toBeVisible();
      await expect(page.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", { timeout: 30_000 });

      const firstTimeContext = await browser.newContext({
        reducedMotion: "reduce",
        serviceWorkers: "block",
      });
      const firstTime = await firstTimeContext.newPage();
      await firstTime.goto(`${BACKEND}/accounts`);
      await firstTime.waitForFunction(
        () => document.documentElement.getAttribute("data-app-ready") === "true",
        null,
        { timeout: 45_000 },
      );
      await expect(firstTime.getByTestId("hosted-auth-gate")).toBeVisible();
      await expect(firstTime.locator(".cf-shell")).toHaveCount(0);
      await expect(firstTime.getByTestId("pending-device-request")).toBeVisible();
      expect(await firstTime.evaluate(() => window.cashfluxStoreGet?.("cashflux:auth:refresh-token") || "")).toBe("");
      await firstTimeContext.close();

      expect(browserErrors).toEqual([]);
    } finally {
      await context.close();
      const deleted = await request.delete(`${BACKEND}/v1/admin/users/${encodeURIComponent(created.id)}`, {
        headers: { Authorization: "Bearer e2e-worker-token" },
      });
      expect(deleted.status()).toBe(200);
    }
  });
});
