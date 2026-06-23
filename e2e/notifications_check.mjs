// C75 — Notification Center lists the feed, marks read on open, and clears.
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
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.addInitScript(() => {
    localStorage.setItem("cashflux:notify:feed", JSON.stringify([
      { id: "n1", title: "Rent is due soon", body: "Due in 3 days — $1,200", at: 1750000000, read: false },
    ]));
  });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator('a[title="Notifications"]').first().click();
  await page.waitForTimeout(500);
  if (!(await page.evaluate(() => document.body.innerText.includes("Rent is due soon")))) fail("Notification Center did not list the feed item");
  // Opening marks read → persisted item.read true.
  if (!(await page.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:notify:feed") || "[]")[0] || {}).read === true))) fail("opening the center did not mark items read");
  // Clear all → empty.
  await page.getByRole("button", { name: "Clear all" }).first().click();
  await page.waitForTimeout(300);
  if (!(await page.evaluate(() => document.body.innerText.includes("No notifications yet")))) fail("Clear all did not empty the center");

  // Settings has the Browser notifications toggle.
  await page.locator("button.hh").first().click();
  await page.waitForTimeout(500);
  if (!(await page.evaluate(() => document.body.innerText.includes("Browser notifications")))) fail("Settings is missing the Browser notifications toggle");
  await ctx.close();
  if (!process.exitCode) console.log("PASS: Notification Center lists feed, marks read, clears; Settings has the browser toggle.");
} finally {
  await browser.close();
}
