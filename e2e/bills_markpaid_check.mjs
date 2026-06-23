// C57 — Bills "Mark paid" logs a payment transaction.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
const txnCount = (page) => page.evaluate(() => {
  try { return (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length; } catch (e) { return -1; }
});
const flush = async (page) => { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(350); };
try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);
  await flush(page);
  const before = await txnCount(page);
  await page.locator('a[title="Bills"]').first().click();
  await page.waitForTimeout(700);
  const paid = page.locator('.rows .row button', { hasText: "Mark paid" }).first();
  if ((await paid.count()) === 0) { console.log("SKIP: no upcoming bills in sample"); process.exit(0); }
  await paid.click();
  await page.waitForTimeout(400);
  if (!(await page.evaluate(() => document.body.innerText.includes("Logged a payment")))) fail("no 'Logged a payment' toast after Mark paid");
  await flush(page);
  const after = await txnCount(page);
  if (!(after > before)) fail(`Mark paid did not add a transaction: ${before} -> ${after}`);
  if (!process.exitCode) console.log("PASS: Bills 'Mark paid' logs a payment transaction.");
} finally {
  await browser.close();
}
