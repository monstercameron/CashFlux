// C295 — "Import dataset" must show a confirmation modal before overwriting data.
//
// Test plan:
//   1. Export the current dataset to a fixture file.
//   2. Open Settings → Data, click "Import JSON".
//   3. Assert the confirm modal appears with a destructive label ("Replace all data").
//   4. Click Cancel — assert the modal closes and no data was lost (accounts still exist).
//   5. Trigger Import again, supply the same fixture, and this time click the confirm button.
//   6. Assert the toast confirms the import completed ("Imported your data").
//
// Note: driving the file-picker inline without a headless download is doable because
// Playwright's filechooser event can intercept file inputs. The wasm runtime tears down
// after a headless download, so we export via the command palette first (which triggers
// the native download), reload, then use the import-button file-picker for the gate tests.
//
// Coverage limit: we cannot assert the app internal state before/after without a full
// reload cycle, so the Cancel path is verified by the absence of a "Imported" toast and
// the modal closing cleanly. The Confirm path is verified by the toast.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import os from "os";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Helper: navigate to the palette command.
const runCommand = async (page, text, query) => {
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.fill("#cf-cmd-input", query);
  await page.waitForTimeout(150);
  return page.locator("[data-cmd-row]").filter({ hasText: text }).first();
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    if (/Go program has already exited/.test(s)) return; // known headless-download artifact
    errors.push(s);
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // ── Step 1: Export the current dataset to a temp fixture via the palette. ──
  const exportRow = await runCommand(page, /export json/i, "export json");
  const [dl] = await Promise.all([
    page.waitForEvent("download", { timeout: 10000 }),
    exportRow.click(),
  ]);
  const fixture = path.join(os.tmpdir(), "cashflux-c295-fixture.json");
  fs.copyFileSync(await dl.path(), fixture);
  if (!fs.existsSync(fixture)) {
    fail("could not capture exported dataset fixture");
  }

  // The download tears down the wasm runtime; reload before continuing.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // ── Step 2: Navigate to Settings → Data via the command palette. ──
  const settingsRow = await runCommand(page, /settings/i, "settings data");
  await settingsRow.click();
  await page.waitForTimeout(800);

  // ── Step 3: Click "Import JSON" and intercept the file chooser. ──
  page.once("filechooser", (fc) => fc.setFiles(fixture));
  // The button label uses the i18n key settings.importData.
  const importBtn = page.getByRole("button", { name: /import json/i }).first();
  await importBtn.click();

  // ── Step 4: Assert the confirm modal appears. ──
  // The in-app modal is rendered by DialogHost and contains the destructive-confirm button
  // labelled "Replace all data" (settings.importConfirmBtn).
  const confirmBtn = page.getByRole("button", { name: /replace all data/i });
  try {
    await confirmBtn.waitFor({ state: "visible", timeout: 8000 });
  } catch {
    fail("confirm modal did not appear after choosing an import file");
  }

  // ── Step 5: Click Cancel — the modal must close without an "Imported" toast. ──
  const cancelBtn = page.getByRole("button", { name: /cancel/i }).first();
  await cancelBtn.click();
  await page.waitForTimeout(400);

  // Modal should be gone.
  const modalStillVisible = await confirmBtn.isVisible().catch(() => false);
  if (modalStillVisible) {
    fail("modal is still visible after clicking Cancel");
  }

  // No "Imported" toast should have appeared.
  const importedToast = page.locator("text=Imported your data");
  const toastAfterCancel = await importedToast.isVisible().catch(() => false);
  if (toastAfterCancel) {
    fail("import proceeded despite clicking Cancel — data-loss gate broken");
  }

  // ── Step 6: Trigger Import again, this time confirm. ──
  page.once("filechooser", (fc) => fc.setFiles(fixture));
  await importBtn.click();

  // Wait for the modal to reappear.
  try {
    await confirmBtn.waitFor({ state: "visible", timeout: 8000 });
  } catch {
    fail("confirm modal did not appear on second import attempt");
  }

  // Click the destructive confirm button.
  await confirmBtn.click();

  // Assert the success toast.
  try {
    await page.locator("text=Imported your data").waitFor({ state: "visible", timeout: 8000 });
  } catch {
    fail("no success toast after confirming the import");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: C295 — import confirm gate verified: modal appears, Cancel aborts, Confirm proceeds.",
    );
} finally {
  await browser.close();
}
