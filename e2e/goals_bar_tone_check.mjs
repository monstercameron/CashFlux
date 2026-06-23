// C51 gate — "a completed goal's progress bar reads as done". Adds a goal whose
// saved-so-far already meets its target (100%) and asserts its bar carries the
// success tone (var(--up)) that incomplete goals don't. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZDONE-GOAL";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const fillStyleOf = (row) => row.locator(".bar-fill").first().getAttribute("style");

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Add a goal already at 100% (saved == target).
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator("#goal-add").fill(NAME);
  await dialog.locator('input[type="number"]').nth(0).fill("50"); // target
  await dialog.locator('input[type="number"]').nth(1).fill("50"); // saved so far
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);
  // Soft-nav cycle to force goals list re-render after modal add.
  await page.evaluate(() => { window.history.pushState({}, '', '/'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.history.pushState({}, '', '/goals'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(800);

  const doneRow = page.locator(".budget", { hasText: NAME });
  if ((await doneRow.count()) === 0) fail(`the completed goal "${NAME}" did not appear`);
  const doneStyle = (await fillStyleOf(doneRow)) || "";
  if (!/var\(--up\)/.test(doneStyle)) fail(`completed goal bar should carry the success tone, style="${doneStyle}"`);

  // Sanity: an incomplete goal's bar should NOT carry the success tone. Find one
  // whose bar width is < 100%.
  const styles = await page.locator(".budget .bar-fill").evaluateAll((els) => els.map((e) => e.getAttribute("style") || ""));
  const incomplete = styles.find((s) => /width:\s*[0-9]{1,2}%/.test(s) && !/width:\s*100%/.test(s));
  if (incomplete && /var\(--up\)/.test(incomplete)) fail(`an incomplete goal bar wrongly carries the success tone: "${incomplete}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: completed goal bar shows the success tone; incomplete goals don't.");
} finally {
  await browser.close();
}
