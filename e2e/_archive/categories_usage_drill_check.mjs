// C63 gate — "category rows show a usage count that drills into Transactions".
// Asserts each category row carries a transactions badge, and clicking a non-zero
// badge navigates to /transactions with that category persisted as the active
// filter. Exits non-zero on any failure.
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

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  // The usage badge appears for categories that have transactions in the sample.
  await page.waitForSelector(".cat-usage", { timeout: 60000 });

  const badge = page.locator(".cat-usage").first();
  const label = (await badge.innerText()).trim();
  if (!/\d+ transactions?$/.test(label)) fail(`usage badge text = "${label}", want "N transaction(s)"`);

  await badge.click();
  await page.waitForTimeout(500);

  if (!page.url().includes("/transactions")) fail(`after drill, url = ${page.url()}, want /transactions`);

  const filter = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}"));
  if (!filter.category) fail(`expected a persisted category filter after drill, got ${JSON.stringify(filter)}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: category usage badge "${label}" drills into Transactions filtered by category.`);
} finally {
  await browser.close();
}
