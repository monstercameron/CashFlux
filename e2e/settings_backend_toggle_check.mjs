// C81 follow-up gate — "the backend can be clearly disabled in the Settings
// modal". Opens Settings, asserts the backend connection toggle defaults on with
// the URL field visible, then turns it off and asserts the connection fields hide
// and BackendDisabled persists to prefs. Exits non-zero on any failure.
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

const prefs = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:prefs") || "{}"));

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('button[title*="Settings"]', { timeout: 60000 });
  await page.locator('button[title*="Settings"]').first().click();

  // The backend toggle row.
  const row = page.locator(".toggle-row", { hasText: "Connect to a backend" });
  await row.waitFor({ timeout: 20000 });
  const sw = row.locator('[role="switch"]');

  if ((await sw.getAttribute("aria-checked")) !== "true") fail("backend toggle should default ON");
  if ((await page.locator('input[aria-label="Backend URL"]').count()) === 0) fail("Backend URL field should be visible when on");

  // Turn it off.
  await sw.click();
  await page.waitForTimeout(400);

  if ((await sw.getAttribute("aria-checked")) !== "false") fail("toggle should be off after click");
  if ((await page.locator('input[aria-label="Backend URL"]').count()) !== 0) fail("Backend URL field should hide when backend is off");

  const p = await prefs(page);
  if (p.backendDisabled !== true) fail(`expected prefs.backendDisabled=true after turning off, got ${JSON.stringify(p.backendDisabled)}`);

  // Turn it back on — field returns, flag clears.
  await sw.click();
  await page.waitForTimeout(400);
  if ((await page.locator('input[aria-label="Backend URL"]').count()) === 0) fail("Backend URL field should return when re-enabled");
  const p2 = await prefs(page);
  if (p2.backendDisabled) fail("prefs.backendDisabled should be cleared when re-enabled");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: backend connection can be clearly toggled off/on in Settings (fields hide; pref persists).");
} finally {
  await browser.close();
}
