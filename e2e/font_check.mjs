// Verifies the app actually consumes the theme font tokens (B20): open Settings,
// read the body's computed font-family, switch the Interface font, and assert the
// computed family changed to follow the selection. Proves the --font-ui token is
// live, not inert. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SERIF = "ui-serif, Georgia, serif";

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
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });

  const before = await page.evaluate(() => getComputedStyle(document.body).fontFamily);

  // Switch the Interface font to the serif option and let it apply.
  await page.locator('select[aria-label="Interface font"]').selectOption(SERIF);
  await page.waitForTimeout(300);

  const after = await page.evaluate(() => getComputedStyle(document.body).fontFamily);

  console.log("body font-family before:", before);
  console.log("body font-family after :", after);

  if (!/serif/i.test(after) || !/georgia/i.test(after)) {
    fail(`after switching to System serif, body font-family should include the serif stack, got: ${after}`);
  }
  if (before === after) {
    fail("body font-family did not change when the Interface font was switched");
  }
  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) console.log("PASS: app consumes the --font-ui token (font selection is live).");
} finally {
  await browser.close();
}
