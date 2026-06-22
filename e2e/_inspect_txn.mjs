import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
await page.waitForTimeout(2000);

const info = await page.evaluate(() => {
  const txnAdd = document.querySelector("#txn-add");
  const form = txnAdd ? txnAdd.closest("form") : document.querySelector("form");
  const inputs = Array.from(document.querySelectorAll("input")).map(i =>
    `  <input type="${i.type}" id="${i.id}" name="${i.name}" aria-label="${i.getAttribute("aria-label")}" placeholder="${i.placeholder}" aria-required="${i.getAttribute("aria-required")}">`
  ).join("\n");
  const buttons = Array.from(document.querySelectorAll("button[type=submit], button")).slice(0,5).map(b =>
    `  <button type="${b.type}" id="${b.id}" class="${b.className.substring(0,60)}">${b.textContent.trim().substring(0,30)}</button>`
  ).join("\n");
  const h1s = Array.from(document.querySelectorAll("h1")).map(h => h.textContent.trim()).join(" | ");
  const rows = document.querySelectorAll("tr, [data-txn], [class*=row]").length;
  return { h1s, inputs, buttons, rows, hasTxnAdd: !!txnAdd, formHtml: form ? form.outerHTML.substring(0, 1500) : "no form" };
});

console.log("H1:", info.h1s);
console.log("Has #txn-add:", info.hasTxnAdd);
console.log("Row-like elements:", info.rows);
console.log("\nINPUTS:\n" + info.inputs);
console.log("\nBUTTONS:\n" + info.buttons);
console.log("\nFORM HTML:\n" + info.formHtml);

await browser.close();
