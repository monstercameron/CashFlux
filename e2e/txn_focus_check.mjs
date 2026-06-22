// L32 gate — fast logging: Description auto-focuses on load, Enter submits, and
// focus returns to Description after a submit; amount input is inputmode=decimal.
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
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });
  await page.waitForTimeout(700);

  // Auto-focus on Description.
  if ((await page.evaluate(() => document.activeElement && document.activeElement.id)) !== "txn-add")
    fail("Description is not auto-focused on load");

  // inputmode=decimal on amount.
  const im = await page.locator('input[type="number"][inputmode="decimal"]').first().count();
  if (im === 0) fail("amount input is not inputmode=decimal");

  // Enter submits, then focus returns to Description.
  const desc = "ZZL32-" + Date.now();
  await page.fill("#txn-add", desc);
  await page.locator('input[type="number"]').first().fill("4.25");
  await page.locator("#txn-add").focus();
  await page.keyboard.press("Enter");
  await page.waitForTimeout(700);
  let saved = false;
  for (let i = 0; i < 15 && !saved; i++) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    saved = await page.evaluate((d) => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).some((t) => (t.desc || "").includes(d)), desc);
    if (!saved) await page.waitForTimeout(300);
  }
  if (!saved) fail("Enter did not submit the add form");
  if ((await page.evaluate(() => document.activeElement && document.activeElement.id)) !== "txn-add")
    fail("focus did not return to Description after submit");

  if (!process.exitCode) console.log("PASS: auto-focus + Enter-submit + focus-return + inputmode=decimal all work.");
} finally {
  await browser.close();
}
