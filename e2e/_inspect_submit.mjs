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
  const form = document.querySelector("#txn-add")?.closest("form");
  const submit = form ? form.querySelector("button[type=submit]") || form.querySelector("button") : null;
  const allFormButtons = form ? Array.from(form.querySelectorAll("button")).map(b =>
    `type=${b.type} text="${b.textContent.trim()}" aria-label="${b.getAttribute("aria-label")}"`
  ) : [];
  // count ledger rows (tr or some row element excluding header)
  const trs = document.querySelectorAll("tbody tr");
  const ledgerCount = trs.length;
  return {
    submitHtml: submit ? submit.outerHTML.substring(0, 300) : "no submit",
    allFormButtons,
    ledgerCount,
    tbodyExists: !!document.querySelector("tbody"),
  };
});

console.log("Submit button:", info.submitHtml);
console.log("All form buttons:", JSON.stringify(info.allFormButtons, null, 2));
console.log("Ledger tbody:", info.tbodyExists, "rows:", info.ledgerCount);

await browser.close();
