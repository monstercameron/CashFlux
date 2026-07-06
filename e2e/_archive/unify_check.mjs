// Verifies the density/scale unify (B20): the theme editor is now the single
// source of truth. Asserts the editor's Compact density actually sets
// data-density (was inert before), the text-size drives --ui-scale and syncs the
// legacy prefs, and the duplicate legacy scale <select> is gone. Non-zero on fail.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

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
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.locator("button.hh").first().click();
  await page.waitForSelector(".theme-editor", { timeout: 8000 });

  // No legacy scale <select> remains (the editor uses a number input).
  const scaleSelects = await page.evaluate(() =>
    [...document.querySelectorAll("select[aria-label]")].filter((s) => /scale/i.test(s.getAttribute("aria-label"))).length
  );
  if (scaleSelects !== 0) fail(`expected no legacy scale <select>, found ${scaleSelects}`);

  // Editor Compact density now sets data-density (previously inert).
  await page.getByRole("radio", { name: "Compact", exact: true }).click();
  await page.waitForTimeout(250);
  const density = await page.evaluate(() => document.documentElement.getAttribute("data-density"));
  if (density !== "compact") fail(`data-density = ${density}, want "compact"`);

  // Editor text-size drives --ui-scale and syncs into the legacy prefs slot.
  const sizeInput = page.locator('input[aria-label="Text size percent"]');
  await sizeInput.fill("130");
  await page.keyboard.press("Tab");
  await page.waitForTimeout(300);
  const out = await page.evaluate(() => ({
    scaleVar: getComputedStyle(document.documentElement).getPropertyValue("--ui-scale").trim(),
    prefScale: (JSON.parse(localStorage.getItem("cashflux:prefs") || "{}")).scale,
    prefCompact: (JSON.parse(localStorage.getItem("cashflux:prefs") || "{}")).compact,
    themeScale: (JSON.parse(localStorage.getItem("cashflux:theme") || "{}")).scale,
  }));
  if (out.scaleVar !== "1.3") fail(`--ui-scale = ${out.scaleVar}, want 1.3`);
  if (out.prefScale !== 130) fail(`prefs.scale = ${out.prefScale}, want 130 (synced)`);
  if (out.prefCompact !== true) fail(`prefs.compact = ${out.prefCompact}, want true (synced)`);
  if (out.themeScale !== 1.3) fail(`theme.scale = ${out.themeScale}, want 1.3`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: editor owns density+scale, drives the DOM, syncs prefs; legacy scale select gone.");
} finally {
  await browser.close();
}
