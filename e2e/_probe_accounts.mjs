import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.goto("http://127.0.0.1:8099/accounts", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(4000);
const inputs = await page.evaluate(() => {
  return Array.from(document.querySelectorAll("input")).map(el => ({
    type: el.type,
    placeholder: el.placeholder,
    ariaLabel: el.getAttribute("aria-label"),
    id: el.id,
    name: el.name
  }));
});
console.log("INPUTS:", JSON.stringify(inputs, null, 2));
const btns = await page.evaluate(() =>
  Array.from(document.querySelectorAll("button")).map(b => b.textContent?.trim()).filter(t => t)
);
console.log("BUTTONS:", btns.slice(0, 20));
await browser.close();
