// One-off visual check for the B20 theme editor: boot the app, open Settings
// (the household button at the foot of the rail), wait for the theme editor to
// render, and screenshot the appearance panel. Not a pass/fail test — it writes
// e2e/theme-editor.png for human/agent review.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 1400 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

  // Open Settings via the household button at the foot of the sidebar.
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });

  // Scroll the editor into view and shoot it (plus the full panel).
  await page.locator(".theme-editor").scrollIntoViewIfNeeded();
  await page.locator(".theme-editor").screenshot({ path: path.join(__dirname, "theme-editor.png") });
  await page.screenshot({ path: path.join(__dirname, "theme-settings-full.png") });

  // Click the first preset to confirm live-apply works, then reshoot.
  await page.locator(".theme-editor button").first().click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(__dirname, "theme-after-preset.png") });

  console.log("page errors:", errors.length ? errors.join(" | ") : "none");
  console.log("wrote theme-editor.png, theme-settings-full.png, theme-after-preset.png");
} finally {
  await browser.close();
}
