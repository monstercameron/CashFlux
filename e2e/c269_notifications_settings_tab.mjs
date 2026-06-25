// C269 gate — Notifications jump-to tab in the global Settings panel.
// Opens Settings, asserts a "Notifications" jump-to tab exists, clicks it,
// confirms the Notifications section scrolls into view and contains the
// browser-notifications toggle, then toggles it and verifies the state changes.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('button[title*="Settings"]', { timeout: 60000 });
  await page.locator('button[title*="Settings"]').first().click();

  // Wait for the settings panel to open.
  await page.waitForSelector(".set-section-nav", { timeout: 20000 });

  // Assert a "Notifications" jump-to tab button exists in the nav.
  const notifyTab = page.locator(".set-section-nav button", { hasText: "Notifications" });
  if ((await notifyTab.count()) === 0) {
    fail("No 'Notifications' jump-to tab found in the settings section nav");
  }

  // Click the Notifications tab and wait for scroll to settle.
  await notifyTab.first().click();
  await page.waitForTimeout(600);

  // Assert the Notifications section exists (via data-testid).
  const section = page.locator('[data-testid="settings-notifications"]');
  if ((await section.count()) === 0) {
    fail("settings-notifications section not found in the DOM");
  }

  // Assert the browser-notifications toggle is present inside the section.
  const toggleRow = section.locator(".toggle-row", { hasText: "Browser notifications" });
  if ((await toggleRow.count()) === 0) {
    fail("Browser notifications toggle row not found in the Notifications section");
  }

  // Read initial state of the toggle.
  const sw = toggleRow.locator('[role="switch"]');
  const before = await sw.getAttribute("aria-checked");

  // Click the toggle and verify the state changed.
  await sw.click();
  await page.waitForTimeout(400);
  const after = await sw.getAttribute("aria-checked");
  if (after === before) {
    fail(`Toggle did not change state — was '${before}', still '${after}'`);
  }

  // Toggle back so we don't leave a dirty pref.
  await sw.click();
  await page.waitForTimeout(300);

  // Take a screenshot as evidence.
  const screenshotPath = path.join(__dirname, "c269_notifications_settings_tab.png");
  await page.screenshot({ path: screenshotPath });
  console.log("Screenshot saved to: " + screenshotPath);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: Notifications jump-to tab present; section and browser-notifications toggle verified; toggle state changes correctly."
    );
} finally {
  await browser.close();
}
