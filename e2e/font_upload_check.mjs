// Verifies the custom-font upload path (B20): open Settings, click "Upload font",
// feed a (dummy) WOFF2 via the file chooser, and assert the plumbing — a managed
// <style id="cashflux-fonts"> gets an @font-face for the family, the family is
// persisted to localStorage, it appears selected in the Interface font picker,
// and the active theme's fontUi is set to it. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const FAMILY = "TestFont";

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

  // Feed a dummy WOFF2 to the file chooser the upload button opens.
  page.on("filechooser", async (fc) => {
    await fc.setFiles({ name: "TestFont.woff2", mimeType: "font/woff2", buffer: Buffer.alloc(256, 7) });
  });
  await page.getByRole("button", { name: "Upload font" }).click();
  await page.waitForTimeout(500);

  // 1. @font-face injected into the managed style tag.
  const css = await page.evaluate(() => document.getElementById("cashflux-fonts")?.textContent || "");
  if (!/@font-face/.test(css) || !css.includes(FAMILY)) {
    fail(`expected an @font-face for ${FAMILY} in #cashflux-fonts, got: ${css.slice(0, 200)}`);
  }

  // 2. Persisted to localStorage.
  const stored = await page.evaluate(() => localStorage.getItem("cashflux:fonts") || "");
  if (!stored.includes(FAMILY)) fail(`cashflux:fonts should persist ${FAMILY}, got: ${stored}`);

  // 3. Appears (selected) in the Interface font picker.
  const selected = await page.evaluate(() => {
    const sel = document.querySelector('select[aria-label="Interface font"]');
    return sel ? sel.value : null;
  });
  if (selected !== FAMILY) fail(`Interface font select value = ${selected}, want ${FAMILY}`);

  // 4. Active theme uses it.
  const theme = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:theme") || "{}"));
  if (theme.fontUi !== FAMILY) fail(`theme.fontUi = ${theme.fontUi}, want ${FAMILY}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: custom font "${FAMILY}" uploads, injects @font-face, persists, and is selected.`);
} finally {
  await browser.close();
}
