// C264 gate — user-settable alert thresholds.
// Opens Settings → Notifications → Manage alerts, sets the large-transaction
// threshold to $1000, reloads the page, and confirms the value persists.
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SCREENSHOT_DIR = path.join(__dirname, "screenshots");

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Boot the app.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('button[title*="Settings"]', { timeout: 60000 });

  // Open the global Settings panel.
  await page.locator('button[title*="Settings"]').first().click();

  // Wait for the Manage alerts section to appear.
  const alertsDiv = page.locator('[data-testid="settings-manage-alerts"]');
  await alertsDiv.waitFor({ timeout: 20000 });

  // Find the large-transaction threshold input by its aria-label.
  const threshInput = page.locator('input[aria-label*="large transaction" i][aria-label*="threshold" i]').first();
  const count = await threshInput.count();
  if (count === 0) {
    fail("large-transaction threshold input not found in Manage alerts section");
  } else {
    // Set the threshold to $1000.
    // Triple-click to select all, then type the new value.  Using
    // pressSequentially + Tab (blur) reliably triggers the wasm-registered
    // "change" event listener; a bare fill() + dispatchEvent() may race the
    // wasm goroutine and arrive before the handler is live.
    await threshInput.click({ clickCount: 3 });
    await threshInput.pressSequentially("1000", { delay: 30 });
    await threshInput.press("Tab");   // blur → fires native change event
    // Give the wasm change-handler time to write the new value into the
    // in-memory SQLite settingskv table, then wait for the 4 s autosave
    // ticker to flush the updated dataset to IDB before reloading.
    await page.waitForTimeout(5000);

    // Close settings and reload.
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('button[title*="Settings"]', { timeout: 30000 });

    // Reopen Settings.
    await page.locator('button[title*="Settings"]').first().click();
    await alertsDiv.waitFor({ timeout: 20000 });

    // Check that the threshold persisted.
    const threshInput2 = page.locator('input[aria-label*="large transaction" i][aria-label*="threshold" i]').first();
    const val = await threshInput2.inputValue();
    if (val !== "1000") {
      fail(`large-transaction threshold should persist as 1000 after reload, got "${val}"`);
    } else {
      console.log("PASS: large-transaction threshold set to $1000 and persisted across reload.");
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  // Take a screenshot.
  if (!fs.existsSync(SCREENSHOT_DIR)) {
    fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  }
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, "c264_thresholds.png") });
  console.log("Screenshot saved to e2e/screenshots/c264_thresholds.png");
} finally {
  await browser.close();
}
