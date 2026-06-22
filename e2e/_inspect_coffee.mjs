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

// Fill and submit
await page.locator("#txn-add").fill("Morning coffee");
await page.locator('input[type="number"][aria-required="true"]').fill("5.00");
await page.locator('form button[type="submit"]').click();
await page.waitForTimeout(1500);

const info = await page.evaluate(() => {
  // find all elements containing "Morning coffee"
  const all = Array.from(document.querySelectorAll("*")).filter(el =>
    el.children.length === 0 && el.textContent.includes("Morning coffee")
  );
  const locations = all.map(el => {
    const path = [];
    let cur = el;
    while (cur && cur !== document.body) {
      const tag = cur.tagName?.toLowerCase();
      const id = cur.id ? `#${cur.id}` : "";
      const cls = cur.className ? `.${String(cur.className).split(" ")[0]}` : "";
      path.unshift(`${tag}${id}${cls}`);
      cur = cur.parentElement;
    }
    return path.join(" > ");
  });

  // also check if it's in the #txn-add input itself (still in form)
  const inputVal = document.querySelector("#txn-add")?.value;

  // check first row text
  const firstTr = document.querySelector("tbody tr")?.textContent?.trim()?.substring(0, 100);

  return { locations, inputVal, firstTrText: firstTr };
});

console.log("Locations of 'Morning coffee':", JSON.stringify(info.locations, null, 2));
console.log("Input value after submit:", info.inputVal);
console.log("First tbody tr:", info.firstTrText);

await browser.close();
