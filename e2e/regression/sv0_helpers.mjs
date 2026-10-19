// Helpers for SV0 incident regressions. Every spec owns a fresh full server and
// temporary database so password identities and remote snapshots cannot leak
// between cases.
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import net from "node:net";
import path from "node:path";
import { gunzipSync } from "node:zlib";
import { expect } from "@playwright/test";
import { nav } from "./fixtures.mjs";

export const APP_ORIGIN = process.env.E2E_BASE_URL || `http://127.0.0.1:${process.env.E2E_PORT || "8099"}`;
export const ADMIN_TOKEN = "sv0-e2e-admin-token";
export const E2E_PASSCODE = "CashFlux-SV0-Setup-7294";

async function freePort() {
  const server = net.createServer();
  await new Promise((resolve, reject) => server.listen(0, "127.0.0.1", resolve).once("error", reject));
  const { port } = server.address();
  await new Promise((resolve, reject) => server.close((err) => err ? reject(err) : resolve()));
  return port;
}

export async function startServer() {
  const port = await freePort();
  const origin = `http://127.0.0.1:${port}`;
  const dataDir = mkdtempSync(path.join(import.meta.dirname, ".sv0-server-"));
  const command = process.platform === "win32" ? "go.exe" : "go";
  const child = spawn(command, ["run", "./cmd/cashflux-server"], {
    cwd: path.resolve(import.meta.dirname, "../.."),
    env: {
      ...process.env,
      CASHFLUX_SERVER_ADDR: `127.0.0.1:${port}`,
      CASHFLUX_SERVER_DATA_DIR: dataDir,
      CASHFLUX_SERVER_AUTH_MODE: "token",
      CASHFLUX_SERVER_TOKEN: ADMIN_TOKEN,
      CASHFLUX_SERVER_APP_ORIGIN: APP_ORIGIN,
      CASHFLUX_SERVER_LOG_LEVEL: "error",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let exited;
  let serverOutput = "";
  child.stdout.on("data", (chunk) => { serverOutput += chunk.toString(); });
  child.stderr.on("data", (chunk) => { serverOutput += chunk.toString(); });
  child.once("exit", (code, signal) => { exited = { code, signal }; });
  const stop = async () => {
    if (process.platform === "win32" && child.pid) {
      try { execFileSync("taskkill.exe", ["/PID", String(child.pid), "/T", "/F"], { stdio: "ignore" }); } catch {}
    } else {
      child.kill();
    }
    await new Promise((resolve) => {
      if (exited) return resolve();
      child.once("exit", resolve);
      setTimeout(resolve, 3000).unref();
    });
    rmSync(dataDir, { recursive: true, force: true });
  };

  try {
    await expect.poll(async () => {
      if (exited) throw new Error(`SV0 server exited (${exited.signal || exited.code}): ${serverOutput}`);
      try {
        const response = await fetch(`${origin}/readyz`);
        return response.status === 204;
      } catch {
        return false;
      }
    }, { timeout: 90_000, intervals: [100, 250, 500, 1000] }).toBe(true);
  } catch (err) {
    await stop();
    throw err;
  }
  return { origin, dataDir, stop };
}

export async function createPasswordAccount(request, origin, username, password) {
  const response = await request.post(`${origin}/v1/admin/users`, {
    headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
    data: { username, password, role: "member" },
  });
  expect(response.status(), `create password account ${username}`).toBe(200);
  return response.json();
}

export async function configureRemote(page, origin) {
  await nav(page, "/settings");
  await page.locator(".settings-page .set-tab-strip button", { hasText: "Cloud" }).first().click();
  const start = page.getByTestId("cloud-setup-start");
  if (await start.isVisible()) await start.click();
  await expect(page.getByTestId("cloud-setup-where")).toBeVisible({ timeout: 15_000 });
  await page.getByRole("radio", { name: "Your own server", exact: true }).click();
  await page.getByTestId("cloud-address-advanced").click();
  await page.getByTestId("sync-server-url").fill(`${origin}/`);
  await page.getByTestId("sync-test").click();
  await expect(page.getByTestId("sync-discovery-ok")).toBeVisible({ timeout: 20_000 });
  await page.getByTestId("cloud-where-continue").click();
  await expect(page.getByTestId("cloud-setup-signin")).toBeVisible();
}

export async function openPasswordForm(page) {
  const expand = page.getByTestId("password-auth-expand");
  const card = page.getByTestId("password-auth-card");
  if (!(await expand.or(card).isVisible())) await page.getByTestId("sync-advanced-token-toggle").click();
  if (await expand.isVisible()) await expand.click();
  await expect(card).toBeVisible();
}

export async function login(page, username, password) {
  await unlockIfNeeded(page);
  await openPasswordForm(page);
  const mode = page.getByRole("radio", { name: "Log in", exact: true });
  if ((await mode.getAttribute("aria-checked")) !== "true") await mode.click({ force: true });
  await page.getByTestId("password-auth-username").fill(username);
  await page.getByTestId("password-auth-password").fill(password);
  await page.getByTestId("password-auth-submit").click({ force: true });
  await expect(page.getByRole("button", { name: "Sign out", exact: true })).toBeVisible({ timeout: 30_000 });
  await protectAndStartEncryptedSync(page);
}

export async function protectAndStartEncryptedSync(page) {
  const protect = page.getByTestId("sync-enable-passcode");
  if (await protect.isVisible()) {
    await protect.click();
    await page.locator("#cf-al-pass").fill(E2E_PASSCODE);
    await page.locator("#cf-al-confirm").fill(E2E_PASSCODE);
    await page.locator("#cf-al-ok").click();
  }
  const startSync = page.getByTestId("cloud-start-encrypted-sync");
  if (!(await startSync.isVisible())) {
    await waitForSync(page);
    await expect(page.getByTestId("sync-status-card")).toContainText(/encrypted/i);
    return;
  }
  await expect(page.getByTestId("cloud-setup-first")).toBeVisible({ timeout: 15_000 });
  await startSync.click();
  await waitForSync(page);
}

export async function unlockIfNeeded(page) {
  const gate = page.locator("#cf-applock-gate");
  if (await gate.isVisible().catch(() => false)) {
    await page.locator("#cf-applock-input").fill(E2E_PASSCODE);
    await gate.getByRole("button", { name: "Unlock" }).click();
    await expect(gate).toBeHidden({ timeout: 10_000 });
  }
}

export async function signOut(page) {
  const button = page.getByRole("button", { name: "Sign out", exact: true });
  await button.click();
  await expect(page.locator("#cf-dialog-confirm")).toBeVisible();
  await page.locator("#cf-dialog-confirm").click();
  await expect(button).toHaveCount(0, { timeout: 20_000 });
}

export async function waitForSync(page) {
  await expect(page.getByTestId("sync-pulse")).toHaveAttribute("data-sync-state", "synced", { timeout: 45_000 });
}

export async function addUniqueTransaction(page, name) {
  await nav(page, "/transactions");
  const panel = page.getByTestId("txn-add-amount");
  await expect.poll(async () => {
    if (await panel.count()) return true;
    await page.getByTestId("add-transaction-btn").click().catch(() => {});
    return (await panel.count()) > 0;
  }, { timeout: 20_000 }).toBe(true);
  await panel.fill("17.43");
  await page.getByTestId("txn-add-payee").fill(name);
  await page.getByTestId("txn-add-desc").fill(name);
  const save = page.getByRole("button", { name: /^save$/i }).last();
  await expect.poll(async () => {
    if (await page.getByText(name, { exact: true }).count()) return true;
    await save.click({ timeout: 2000 }).catch(() => {});
    return (await page.getByText(name, { exact: true }).count()) > 0;
  }, { timeout: 25_000 }).toBe(true);
}

export async function datasetHas(page, text) {
  return page.evaluate((needle) => {
    const raw = window.cashfluxStoreGet?.("cashflux:dataset") || "";
    return raw.includes(needle);
  }, text);
}

export async function accountSnapshots(request, origin, accessToken) {
  const response = await request.get(`${origin}/v1/account/export`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  expect(response.status(), "account export should read this signed-in user's server copy").toBe(200);
  const body = await response.json();
  const snapshots = body.snapshots || body.Snapshots || [];
  return snapshots.map((snapshot) => {
    const encoded = snapshot.Dataset ?? snapshot.dataset ?? "";
    const bytes = Buffer.from(encoded, "base64");
    let envelope;
    try { envelope = gunzipSync(bytes).toString("utf8"); } catch { envelope = bytes.toString("utf8"); }
    return { workspaceID: snapshot.WorkspaceID ?? snapshot.workspaceId ?? "", envelope };
  });
}

export async function accessToken(page) {
  return page.evaluate(() => window.cashfluxStoreGet?.("cashflux:auth:access-token") || "");
}

export async function reloadReady(page) {
  await page.reload();
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await unlockIfNeeded(page);
}
