import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.goto("http://127.0.0.1:8099/goals", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(4000);
const inputs = await page.evaluate(() => {
  return Array.from(document.querySelectorAll("input")).map(el => ({
    type: el.type,
    placeholder: el.placeholder,
    ariaLabel: el.getAttribute("aria-label"),
    id: el.id,
  }));
});
console.log("INPUTS:", JSON.stringify(inputs, null, 2));
const sels = await page.evaluate(() =>
  Array.from(document.querySelectorAll("select")).map(s => ({
    ariaLabel: s.getAttribute("aria-label"),
    options: Array.from(s.options).map(o => o.text.trim()).slice(0, 5)
  }))
);
console.log("SELECTS:", JSON.stringify(sels, null, 2));
const bodyTxt = await page.evaluate(() => document.body.innerText);
// check for L59 goal or 100%
const has100 = /100\s*%/.test(bodyTxt);
const hasL59 = bodyTxt.includes("L59");
console.log("has100%:", has100, "hasL59:", hasL59);
console.log("Body snippet:", bodyTxt.substring(0, 500));
await browser.close();
