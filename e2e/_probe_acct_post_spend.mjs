// Probe: what does the account section look like after a spend transaction?
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });

// Step 1: Add account
await page.goto("http://127.0.0.1:8099/accounts", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3000);
await (await page.$('input[placeholder="Name"]')).fill("L59 SpendProbe");
await (await page.$('input[placeholder="Opening balance"]')).fill("3000");
const sub = await page.$('button[type="submit"]');
await sub.click();
await page.waitForTimeout(2000);

// Step 2: Log a $500 spend in /transactions against that account
await page.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3000);
const descIn = await page.$('input[placeholder="Description"]');
const amtIn  = await page.$('input[type="number"][aria-required="true"]') ?? await page.$('input[placeholder="Amount"]');
if (descIn && amtIn) {
  await descIn.fill("L59 SpendProbe Purchase");
  await amtIn.fill("500");
  // Select account
  const acctSel = await page.$('select[aria-label*="account" i]');
  if (acctSel) {
    const opts = await acctSel.evaluate(el => Array.from(el.options).map(o => ({v: o.value, t: o.text})));
    const match = opts.find(o => o.t.includes("L59 SpendProbe"));
    if (match) await acctSel.selectOption({ value: match.v });
  }
  const sub2 = await page.$('button[type="submit"]');
  await sub2.click();
  await page.waitForTimeout(2000);
}

// Step 3: Go back to accounts and read the balance
await page.goto("http://127.0.0.1:8099/accounts", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3000);
const txt = await page.evaluate(() => document.body.innerText);
const idx = txt.indexOf("L59 SpendProbe");
console.log("Account context after spend:", txt.substring(idx, idx + 200));

await browser.close();
