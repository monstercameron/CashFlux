// C263 gate — per-alert-type "Manage alerts" rows in Settings → Notifications.
// Opens Settings, navigates to the Notifications section, asserts the
// "Manage alerts" group with per-alert toggles renders, toggles one alert
// type OFF, reloads, confirms it persists off, then takes a screenshot.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const openSettingsNotifications = async (page) => {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('button[title*="Settings"]', { timeout: 60000 });
  await page.locator('button[title*="Settings"]').first().click();
  // Wait for panel; use the section nav if available, otherwise just wait for the section.
  await page.waitForSelector('[data-testid="settings-notifications"]', { timeout: 20000 });
  // Scroll the Notifications section into view via the jump-to nav if present.
  const notifyTab = page.locator(".set-section-nav button", { hasText: "Notifications" });
  if ((await notifyTab.count()) > 0) {
    await notifyTab.first().click();
    await page.waitForTimeout(500);
  }
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await openSettingsNotifications(page);

  // Assert the Notifications section exists.
  const section = page.locator('[data-testid="settings-notifications"]');
  if ((await section.count()) === 0) {
    fail("settings-notifications section not found");
  }

  // Assert the Manage alerts sub-group exists.
  const manageGroup = page.locator('[data-testid="settings-manage-alerts"]');
  if ((await manageGroup.count()) === 0) {
    fail("settings-manage-alerts group not found in the Notifications section");
  }

  // Assert at least one alert-type toggle row is present.
  const alertRows = manageGroup.locator(".toggle-row");
  const rowCount = await alertRows.count();
  if (rowCount === 0) {
    fail("No alert-type toggle rows found in the Manage alerts group");
  }
  console.log(`Found ${rowCount} alert-type rows`);

  // Pick the first row, read its switch state, and toggle it OFF (if on) or ON then back OFF.
  const firstSwitch = alertRows.first().locator('[role="switch"]');
  const initialState = await firstSwitch.getAttribute("aria-checked");
  console.log(`First alert row initial state: ${initialState}`);

  // Toggle the first alert row to OFF.
  if (initialState !== "false") {
    await firstSwitch.click();
    await page.waitForTimeout(400);
    const afterToggle = await firstSwitch.getAttribute("aria-checked");
    if (afterToggle !== "false") {
      fail(`Toggle did not turn off — was '${initialState}', now '${afterToggle}'`);
    }
  }

  // Wait for the autosave tick (fires every 4s) so the setting is flushed to
  // IndexedDB before we reload. The pagehide handler also fires on reload, but
  // waiting here makes the test deterministic regardless of timing.
  await page.waitForTimeout(5000);

  // Close settings, reload, reopen settings to check persistence.
  const closeBtn = page.locator('.flip-panel button[aria-label*="Close"], .flip-panel button', { hasText: "Close" });
  if ((await closeBtn.count()) > 0) {
    await closeBtn.first().click();
    await page.waitForTimeout(300);
  }

  await page.reload({ waitUntil: "domcontentloaded" });
  await openSettingsNotifications(page);

  // After reload, the first row should still be off.
  const manageGroupAfter = page.locator('[data-testid="settings-manage-alerts"]');
  const firstSwitchAfter = manageGroupAfter.locator(".toggle-row").first().locator('[role="switch"]');
  const persistedState = await firstSwitchAfter.getAttribute("aria-checked");
  if (persistedState !== "false") {
    fail(`Persisted state should be 'false' after reload, got '${persistedState}'`);
  } else {
    console.log("Persistence confirmed: first alert row is still off after reload");
  }

  // Restore the toggle so we don't dirty the user's prefs.
  await firstSwitchAfter.click();
  await page.waitForTimeout(300);

  // Screenshot.
  const screenshotPath = path.join(__dirname, "screenshots", "c263_manage_alerts.png");
  await page.screenshot({ path: screenshotPath, fullPage: false });
  console.log("Screenshot saved to: " + screenshotPath);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: Manage alerts rows render with per-alert toggles; toggle persists off across reload."
    );
} finally {
  await browser.close();
}
