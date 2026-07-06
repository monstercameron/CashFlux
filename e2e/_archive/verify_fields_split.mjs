// Verifies the /customize → /customize (Formulas) + /fields (Custom fields) split
// (themed-remap item 7). Asserts: both rail entries exist; /customize renders the
// formula calculator and NOT the custom-fields manager; /fields renders the
// custom-fields manager and NOT the formula calculator. Pass/fail (exit code).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const fails = [];
const ok = (cond, msg) => { if (!cond) fails.push(msg); else console.log("  ✓", msg); };

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(700);

  // 1) Both rail entries present.
  const hasFormulas = await page.locator('nav a[title="Formulas"]').count();
  const hasFields = await page.locator('nav a[title="Custom fields"]').count();
  ok(hasFormulas > 0, 'rail has "Formulas" entry (/customize)');
  ok(hasFields > 0, 'rail has "Custom fields" entry (/fields)');

  // 2) /customize (Formulas) = formula calculator only.
  await page.locator('nav a[title="Formulas"]').first().click();
  await page.waitForTimeout(700);
  let body = await page.locator("main").innerText();
  ok(/Formula calculator/.test(body), "/customize shows the Formula calculator");
  // The variable palette replaced the "Available variables" heading (4366ae87);
  // its click-to-insert hint is the stable marker now.
  ok(/Click a variable to insert/.test(body), "/customize shows the variable palette");
  ok(!/Add a custom field/.test(body), "/customize does NOT show the custom-fields manager");
  const cUrl = page.url();
  ok(/\/customize$/.test(cUrl), `/customize URL is correct (${cUrl})`);

  // 3) /fields (Custom fields) = custom-fields manager only.
  await page.locator('nav a[title="Custom fields"]').first().click();
  await page.waitForTimeout(700);
  body = await page.locator("main").innerText();
  ok(/Add a custom field/.test(body), "/fields shows the custom-fields manager");
  ok(!/Formula calculator/.test(body), "/fields does NOT show the Formula calculator");
  const fUrl = page.url();
  ok(/\/fields$/.test(fUrl), `/fields URL is correct (${fUrl})`);

  // 4) Deep-link refresh straight to /fields resolves (SPA fallback).
  await page.goto(BASE + "/fields", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(700);
  body = await page.locator("main").innerText();
  ok(/Add a custom field/.test(body), "deep-link /fields renders the custom-fields manager");

  ok(errors.length === 0, `no page errors (${errors.length ? errors.join(" | ") : "none"})`);
} finally {
  await browser.close();
}

if (fails.length) {
  console.error("\nFAIL:\n - " + fails.join("\n - "));
  process.exit(1);
}
console.log("\nPASS: /customize + /fields split verified");
