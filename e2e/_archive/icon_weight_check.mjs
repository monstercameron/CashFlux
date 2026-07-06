// Verifies themeable icon weight (B13): open Settings, read a rail icon's computed
// stroke-width, pick the "Bold" icon weight, and assert --icon-stroke and the
// icon's rendered stroke-width both change (and persist). Screenshots the rail.
// Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const RAIL_ICON = 'nav[aria-label="Main navigation"] a svg';

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
  await page.waitForSelector(RAIL_ICON, { timeout: 60000 });

  const before = await page.evaluate((sel) => getComputedStyle(document.querySelector(sel)).strokeWidth, RAIL_ICON);

  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });
  await page.getByRole("radio", { name: "Bold", exact: true }).click();
  await page.waitForTimeout(300);

  const after = await page.evaluate((sel) => ({
    stroke: getComputedStyle(document.querySelector(sel)).strokeWidth,
    cssVar: getComputedStyle(document.documentElement).getPropertyValue("--icon-stroke").trim(),
    themeStroke: (JSON.parse(localStorage.getItem("cashflux:theme") || "{}")).iconStroke,
  }), RAIL_ICON);

  console.log("rail icon stroke-width before:", before, "after:", after.stroke);
  if (after.cssVar !== "2.2") fail(`--icon-stroke = ${after.cssVar}, want 2.2`);
  if (!/2\.2/.test(after.stroke)) fail(`rail icon stroke-width should be 2.2, got ${after.stroke}`);
  if (before === after.stroke) fail("icon stroke-width did not change when weight switched to Bold");
  if (after.themeStroke !== 2.2) fail(`theme.iconStroke = ${after.themeStroke}, want 2.2`);

  await page.getByRole("button", { name: "Cancel", exact: true }).click().catch(() => {});
  await page.waitForTimeout(300);
  await page.locator('nav[aria-label="Main navigation"]').screenshot({ path: path.join(__dirname, "icon-weight-bold.png") });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: icon weight is themeable (Bold thickens icons, persists). Wrote icon-weight-bold.png");
} finally {
  await browser.close();
}
