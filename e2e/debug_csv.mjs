import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });
const waitNav = (page) => page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
const goto = async (hash) => { await page.goto(BASE + hash, { waitUntil: "domcontentloaded" }); await waitNav(page); await page.waitForTimeout(1500); };

// Test CSV import with correct format
await goto("/documents");

const IMPORT_CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,
2026-06-16,L44 COFFEE SHOP,-12.50,
2026-06-18,L44 RENT PARTIAL,-200.00,
2026-06-20,L44 PAYCHECK DEPOSIT,1500.00,
2026-06-21,L44 UTILITIES PAYMENT,-147.50,`;

// Find and fill the CSV textarea (the one with date,payee placeholder)
const textareas = await page.$$('textarea');
console.log("Textarea count:", textareas.length);
for (let i = 0; i < textareas.length; i++) {
  const ph = await textareas[i].getAttribute("placeholder");
  console.log(`Textarea ${i} placeholder:`, ph?.slice(0, 80));
}

// Fill the second textarea (CSV import)
const csvTA = textareas[1];
await csvTA.fill(IMPORT_CSV);
console.log("Filled CSV textarea");

// Click Import
const importBtn = await page.$('button:has-text("Import")');
await importBtn.click();
await page.waitForTimeout(2000);

const bodyAfter = await page.evaluate(() => document.body.innerText);
// Find the message area near the Import button
const msgSection = bodyAfter.match(/Import[\s\S]{0,400}/i);
console.log("After import body fragment:", msgSection?.[0]?.slice(0, 400));

// Check /transactions
await goto("/transactions");
const txnBody = await page.evaluate(() => document.body.innerText);
const hasL44 = txnBody.includes("L44");
console.log("L44 in transactions:", hasL44);
if (hasL44) {
  const idx = txnBody.indexOf("L44");
  console.log("L44 context:", txnBody.slice(Math.max(0, idx-50), idx+200));
}

await browser.close();
