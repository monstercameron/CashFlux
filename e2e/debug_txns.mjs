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

// Add account + import CSV
await goto("/accounts");
const nameInput = await page.$('input[type="text"]');
await nameInput.fill("L44 Omar Checking");
const typeSelect = await page.$('select');
await typeSelect.selectOption({ label: "Checking" });
const amtInput = await page.$('input[type="number"]');
await amtInput.fill("1000");
const addBtn = await page.$('button:has-text("Add account")');
await addBtn.click();
await page.waitForTimeout(1200);

await goto("/documents");
const textareas = await page.$$('textarea');
const CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking
2026-06-16,L44 COFFEE SHOP,-12.50,L44 Omar Checking
2026-06-18,L44 RENT PARTIAL,-200.00,L44 Omar Checking
2026-06-20,L44 PAYCHECK DEPOSIT,1500.00,L44 Omar Checking
2026-06-21,L44 UTILITIES PAYMENT,-147.50,L44 Omar Checking`;
await textareas[1].fill(CSV);
const importBtn = await page.$('button:has-text("Import")');
await importBtn.click();
await page.waitForTimeout(2000);

// Check transactions - look at actual row elements
await goto("/transactions");
const txnRowEls = await page.$$('[class*="row"]:not([class*="row-main"]):not([class*="row-desc"]):not([class*="row-meta"])');
console.log("Transaction row elements found:", txnRowEls.length);

// Read all visible row text
const allRowTexts = await page.evaluate(() => {
  // Look for the transaction list items
  const items = document.querySelectorAll('li, tr, [class*="txn"], [class*="transaction"]');
  return Array.from(items).map(el => el.innerText?.trim()?.slice(0, 100)).filter(t => t && t.length > 5).slice(0, 30);
});
console.log("Row texts:", JSON.stringify(allRowTexts));

// Also check the full transactions body for L44 specifically
const fullBody = await page.evaluate(() => document.body.innerText);
const grocIdx = fullBody.indexOf("SUPERMARKET");
const coffeeIdx = fullBody.indexOf("COFFEE");
console.log("SUPERMARKET index:", grocIdx, "COFFEE index:", coffeeIdx);
if (grocIdx >= 0) console.log("SUPERMARKET context:", fullBody.slice(grocIdx-100, grocIdx+200));

// Check the period shown
const periodMatch = fullBody.match(/(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+20\d\d/i);
console.log("Period:", periodMatch?.[0]);

await browser.close();
