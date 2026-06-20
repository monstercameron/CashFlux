// B16 E2E story — "allocate: exclude + restore a suggestion". The Allocate screen
// ranks where to put new capital. This asserts the core interaction: excluding a
// ranked candidate removes it from the active suggestions (and offers a restore),
// and restoring brings it back. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const EXCLUDE = 'button[title="Leave this out of the suggestions"]';
const RESTORE = 'button[title="Bring this back into the suggestions"]';

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

async function waitForCount(locatorSel, page, want, timeoutMs = 6000) {
  for (let waited = 0; waited < timeoutMs; waited += 300) {
    if ((await page.locator(locatorSel).count()) === want) return true;
    await page.waitForTimeout(300);
  }
  return (await page.locator(locatorSel).count()) === want;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/allocate", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(EXCLUDE, { timeout: 60000 });

  const activeBefore = await page.locator(EXCLUDE).count();
  const excludedBefore = await page.locator(RESTORE).count();
  if (activeBefore < 2) fail(`expected at least 2 ranked suggestions, got ${activeBefore}`);

  // Exclude the top suggestion.
  await page.locator(EXCLUDE).first().click();
  if (!(await waitForCount(EXCLUDE, page, activeBefore - 1))) fail(`active suggestions should drop to ${activeBefore - 1}`);
  if ((await page.locator(RESTORE).count()) !== excludedBefore + 1) fail("excluding should add a restorable item");

  // Restore it.
  await page.locator(RESTORE).first().click();
  if (!(await waitForCount(EXCLUDE, page, activeBefore))) fail(`restoring should bring active suggestions back to ${activeBefore}`);
  if ((await page.locator(RESTORE).count()) !== excludedBefore) fail("restoring should clear the restorable item");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: allocate exclude/restore works (${activeBefore} suggestions -> ${activeBefore - 1} -> ${activeBefore}).`);
} finally {
  await browser.close();
}
