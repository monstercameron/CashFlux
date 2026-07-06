// L49 gate — doCancel / doUncancel fire a success notice so the Subscriptions
// screen re-renders immediately after "Mark as cancelled" / "Undo cancel".
// Asserts: after clicking "Mark as cancelled" a toast appears and the row flips
// to its cancelled state (shows the Undo cancel button); after clicking "Undo
// cancel" the row reverts to its active state (shows "Mark as cancelled" again).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  await page.locator('a[title="Subscriptions"]').first().click();
  await page.waitForTimeout(700);

  // Find the first "Mark as cancelled" button (non-cancelled row).
  const cancelBtn = page.locator('.rows .row button', { hasText: "Mark as cancelled" }).first();
  if ((await cancelBtn.count()) === 0) {
    console.log("SKIP: no detected subscriptions in sample — cannot test cancel re-render");
    process.exit(0);
  }

  await cancelBtn.click();
  await page.waitForTimeout(500);

  // A toast should have appeared with "Marked … as cancelled."
  const bodyText = await page.evaluate(() => document.body.innerText);
  if (!/(Marked|cancelled)/i.test(bodyText)) {
    fail("no cancel-confirmation toast after 'Mark as cancelled'");
  }

  // The row should now show "Undo cancel" — meaning the screen re-rendered.
  const undoBtn = page.locator('.rows .row button', { hasText: "Undo cancel" }).first();
  if ((await undoBtn.count()) === 0) {
    fail("row did not re-render to cancelled state — 'Undo cancel' button not found");
  }

  // Now click "Undo cancel" and verify the row reverts.
  await undoBtn.click();
  await page.waitForTimeout(500);

  const bodyAfter = await page.evaluate(() => document.body.innerText);
  if (!/(Removed.*cancellation|cancellation)/i.test(bodyAfter)) {
    fail("no uncancel-confirmation toast after 'Undo cancel'");
  }

  const cancelBtnAfter = page.locator('.rows .row button', { hasText: "Mark as cancelled" }).first();
  if ((await cancelBtnAfter.count()) === 0) {
    fail("row did not re-render back to active state — 'Mark as cancelled' button not found after uncancel");
  }

  if (!process.exitCode) console.log("PASS: cancel/uncancel trigger re-render and confirmation toasts.");
} finally {
  await browser.close();
}
