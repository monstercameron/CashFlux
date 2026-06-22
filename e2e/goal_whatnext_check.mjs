// L20 gate — "a completed goal offers a calm 'what next' redirect." Creates a
// goal that's already funded (saved >= target), asserts the row shows the
// what-next prompt with a Reallocate action, and that clicking it navigates to
// Allocate so the freed-up monthly can be put to work elsewhere.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZWHATNEXT-" + Date.now();
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#goal-add", { timeout: 60000 });

  // Add a goal already funded: target 100, saved 100 → complete.
  await page.fill("#goal-add", NAME);
  await page.locator('input[type="number"]').nth(0).fill("100"); // target
  await page.locator('input[type="number"]').nth(1).fill("100"); // saved so far
  await page.locator('form button[type="submit"]').first().click();
  await page.waitForTimeout(500);

  // The new (complete) goal's row shows the "what next" prompt + Reallocate action.
  const row = page.locator('[data-testid^="goal-row-"]', { hasText: NAME }).first();
  await row.waitFor({ state: "attached", timeout: 10000 });
  const whatNext = row.locator('[data-testid^="goal-whatnext-"]');
  if ((await whatNext.count()) === 0) fail("completed goal does not show the 'what next' prompt");
  const redirect = row.locator('[data-testid^="goal-redirect-"]');
  if ((await redirect.count()) === 0) { fail("no Reallocate action on the completed goal"); process.exit(1); }

  // Clicking Reallocate navigates to Allocate.
  await redirect.first().click();
  await page.waitForTimeout(500);
  if (!/\/allocate$/.test(new URL(page.url()).pathname)) {
    fail(`Reallocate did not navigate to /allocate (url=${page.url()})`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: a completed goal shows a 'what next' prompt that jumps to Allocate.");
} finally {
  await browser.close();
}
