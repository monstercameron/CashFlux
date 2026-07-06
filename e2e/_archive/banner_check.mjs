// Verifies the dashboard banner (B20): open Settings, pick a gradient preset, and
// assert the band activates (root data-banner="on", .app-banner visible, --banner-bg
// set, persisted), then close the panel and screenshot the dashboard with the band.
// Also checks "Remove banner" turns it off. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { ready } from "./_ready.mjs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 1000 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await ready(page);
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });

  // Pick the "Aurora" gradient preset.
  await page.getByRole("button", { name: "Aurora", exact: true }).click();
  await page.waitForTimeout(300);

  const on = await page.evaluate(() => ({
    attr: document.documentElement.getAttribute("data-banner"),
    bg: getComputedStyle(document.documentElement).getPropertyValue("--banner-bg").trim(),
    display: (() => { const b = document.querySelector(".app-banner"); return b ? getComputedStyle(b).display : "none"; })(),
    stored: localStorage.getItem("cashflux:banner") || "",
  }));
  if (on.attr !== "on") fail(`data-banner = ${on.attr}, want "on"`);
  if (!/gradient/.test(on.bg)) fail(`--banner-bg should be a gradient, got: ${on.bg}`);
  if (on.display === "none") fail(".app-banner should be visible when a banner is set");
  if (!on.stored.includes("Aurora")) fail(`cashflux:banner should persist Aurora, got: ${on.stored}`);

  // Close the settings panel and screenshot the dashboard with the band.
  await page.getByRole("button", { name: "Cancel", exact: true }).click().catch(() => {});
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(__dirname, "banner-dashboard.png") });

  // Re-open and remove the banner.
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });
  await page.getByRole("button", { name: "Remove banner", exact: true }).click();
  await page.waitForTimeout(300);
  const off = await page.evaluate(() => document.documentElement.getAttribute("data-banner"));
  if (off !== "off") fail(`after remove, data-banner = ${off}, want "off"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: banner preset activates the band, persists, and removes cleanly. Wrote banner-dashboard.png");
} finally {
  await browser.close();
}
