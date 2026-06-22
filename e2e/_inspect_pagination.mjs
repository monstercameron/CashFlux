import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 800 });
await page.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
await page.waitForTimeout(2000);

const info = await page.evaluate(() => {
  const trs = document.querySelectorAll("tbody tr");
  // look for pagination elements
  const pagerText = document.body.innerText.match(/showing \d+ of \d+|page \d+|\d+–\d+ of \d+/i);
  const totalCount = document.body.innerText.match(/\d+ transactions?/i);
  // get first and last visible row descriptions
  const firstRow = trs[0]?.textContent?.trim()?.substring(0, 60);
  const lastRow = trs[trs.length - 1]?.textContent?.trim()?.substring(0, 60);
  return {
    rowCount: trs.length,
    pagerText: pagerText?.[0],
    totalCount: totalCount?.[0],
    firstRow,
    lastRow,
    // look for "Morning coffee" row
    coffeeRow: Array.from(trs).findIndex(tr => tr.textContent.includes("Morning coffee")),
  };
});

console.log(JSON.stringify(info, null, 2));
await browser.close();
