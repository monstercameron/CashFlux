import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });
await page.goto(BASE, { waitUntil: "domcontentloaded" });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 }).catch(() => {});
await page.waitForTimeout(3000);

// navigate to planning
await page.evaluate(() => {
  const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
  const link = links.find(l => l.getAttribute("title") === "Planning");
  if (link) link.click();
});
await page.waitForTimeout(2500);

const body = await page.evaluate(() => document.body.innerText.slice(0, 4000));
console.log("=== PLANNING BODY ===\n", body);

const sectionClasses = await page.evaluate(() =>
  Array.from(document.querySelectorAll("section")).map(s => ({
    cls: s.className, text: s.textContent.slice(0, 80).replace(/\s+/g, " ")
  }))
);
console.log("=== SECTIONS ===\n", JSON.stringify(sectionClasses, null, 2));

const allInputs = await page.evaluate(() =>
  Array.from(document.querySelectorAll("input")).map(i => ({
    type: i.type, placeholder: i.placeholder, ariaLabel: i.getAttribute("aria-label"), value: i.value
  }))
);
console.log("=== INPUTS ===\n", JSON.stringify(allInputs, null, 2));

// Navigate to accounts to see form
await page.evaluate(() => {
  const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
  const link = links.find(l => l.getAttribute("title") === "Accounts");
  if (link) link.click();
});
await page.waitForTimeout(2000);
const acctsBody = await page.evaluate(() => document.body.innerText.slice(0, 2000));
console.log("=== ACCOUNTS BODY ===\n", acctsBody);

const acctInputs = await page.evaluate(() =>
  Array.from(document.querySelectorAll("input")).map(i => ({
    type: i.type, placeholder: i.placeholder, ariaLabel: i.getAttribute("aria-label")
  }))
);
console.log("=== ACCOUNT INPUTS ===\n", JSON.stringify(acctInputs, null, 2));

await browser.close();
