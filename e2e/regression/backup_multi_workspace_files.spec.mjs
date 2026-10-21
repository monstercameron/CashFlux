import { test, expect } from "@playwright/test";
import fs from "node:fs/promises";
import { nav } from "./fixtures.mjs";

const image = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");

async function ready(page) {
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 60_000 });
}

async function upload(page, filename) {
  await nav(page, "/artifacts");
  const [chooser] = await Promise.all([
    page.waitForEvent("filechooser"),
    page.getByRole("button", { name: /upload image/i }).click(),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: image });
  await expect(page.getByTestId("artifact-name").filter({ hasText: filename })).toBeVisible();
}

async function downloadArtifact(page, filename) {
  await nav(page, "/artifacts");
  const name = page.getByTestId("artifact-name").filter({ hasText: filename });
  await expect(name).toBeVisible();
  const row = page.locator(".row").filter({ has: name });
  await row.locator('[data-testid^="artifact-menu-btn-"]').click();
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    row.locator('[data-testid^="artifact-download-"]').click(),
  ]);
  expect(await fs.readFile(await download.path())).toEqual(image);
}

test("full backup restores files from two workspaces in another browser", async ({ browser }) => {
  test.setTimeout(180_000);
  const sourceContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce", acceptDownloads: true });
  const restoreContext = await browser.newContext({ serviceWorkers: "block", reducedMotion: "reduce", acceptDownloads: true });
  const source = await sourceContext.newPage();
  const restored = await restoreContext.newPage();
  try {
    await source.goto("/");
    await ready(source);
    await expect(source.getByTestId("sample-start-fresh")).toBeVisible({ timeout: 60_000 });
    await Promise.all([source.waitForEvent("load"), source.getByTestId("sample-start-fresh").click()]);
    await ready(source);
    const firstID = await source.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")).activeId);
    await upload(source, "backup-first.png");

    await source.keyboard.press("Control+k");
    await source.locator("#cf-cmd-input").fill("new workspace");
    await source.getByText(/new workspace/i).last().click();
    const prompt = source.getByRole("dialog").getByRole("textbox", { name: "Name the new workspace" });
    await prompt.fill("Backup second workspace");
    await Promise.all([source.waitForEvent("load"), prompt.press("Enter")]);
    await ready(source);
    const secondID = await source.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")).activeId);
    expect(secondID).not.toBe(firstID);
    await upload(source, "backup-second.png");

    await nav(source, "/settings");
    await source.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    const [backupDownload] = await Promise.all([
      source.waitForEvent("download"),
      source.getByRole("button", { name: "Back up everything" }).click(),
    ]);
    const backupBytes = await fs.readFile(await backupDownload.path());
    const backup = JSON.parse(backupBytes.toString("utf8"));
    expect(backup.datasets).toHaveLength(2);
    expect(JSON.parse(backup.datasets[0]).artifacts[0].name).toBe("backup-first.png");
    expect(JSON.parse(backup.datasets[1]).artifacts[0].name).toBe("backup-second.png");

    await restored.goto("/");
    await ready(restored);
    await expect(restored.getByTestId("sample-start-fresh")).toBeVisible({ timeout: 60_000 });
    await Promise.all([restored.waitForEvent("load"), restored.getByTestId("sample-start-fresh").click()]);
    await ready(restored);
    await nav(restored, "/settings");
    await restored.locator(".settings-page .set-tab-strip button", { hasText: "Data" }).first().click();
    await restored.keyboard.press("Control+k");
    await restored.locator("#cf-cmd-input").fill("restore");
    const [restoreChooser] = await Promise.all([
      restored.waitForEvent("filechooser"),
      restored.getByText(/restore from a backup file/i).click(),
    ]);
    await restoreChooser.setFiles({ name: "cashflux-backup.json", mimeType: "application/json", buffer: backupBytes });
    await expect(restored.locator("#cf-dialog-confirm")).toBeVisible();
    await Promise.all([restored.waitForEvent("load"), restored.locator("#cf-dialog-confirm").click()]);
    await ready(restored);

    const restoredRegistry = await restored.evaluate(() => JSON.parse(window.cashfluxStoreGet("cashflux:workspaces")));
    expect(restoredRegistry.workspaces).toHaveLength(2);
    expect(restoredRegistry.activeId).toBe(secondID);
    await downloadArtifact(restored, "backup-second.png");

    await restored.getByTestId("ws-switch-trigger").click();
    await restored.getByTestId("ws-menu").getByRole("button", { name: "Default" }).click();
    await ready(restored);
    await downloadArtifact(restored, "backup-first.png");
  } finally {
    await sourceContext.close();
    await restoreContext.close();
  }
});
