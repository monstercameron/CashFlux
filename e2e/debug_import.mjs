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

// 1. Add L44 Omar Checking
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

// 2. Navigate to documents and try import with correct account name
await goto("/documents");

// Use the known CSV format
const CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking
2026-06-16,L44 COFFEE SHOP,-12.50,L44 Omar Checking`;

// Fill the second textarea (CSV import one)
const textareas = await page.$$('textarea');
await textareas[1].fill(CSV);
await page.waitForTimeout(300);

// Click Import
const importBtn = await page.$('button:has-text("Import")');
await importBtn.click();
await page.waitForTimeout(2000);

const docBody = await page.evaluate(() => document.body.innerText);
// Print full page body to see the import message
const mainContent = docBody.slice(docBody.indexOf("Documents"), docBody.indexOf("Documents") + 600);
console.log("Documents page content after import:", mainContent);

// Check if the message section shows the import result
// Look for any message that changed
const msgEl = await page.$('.msg, [class*="msg"], [class*="notice"], p:has-text("Imported"), p:has-text("transaction"), p:has-text("error")');
if (msgEl) {
  const msgText = await msgEl.textContent();
  console.log("Message element:", msgText);
}

// 3. Check transactions
await goto("/transactions");
const txnBody = await page.evaluate(() => document.body.innerText);
const hasL44 = txnBody.includes("L44");
console.log("L44 in transactions:", hasL44);
if (hasL44) {
  const idx = txnBody.indexOf("L44");
  console.log("L44 context:", txnBody.slice(idx, idx+300));
} else {
  // Show period and some transaction rows
  const txnSample = txnBody.slice(txnBody.indexOf("Transactions"), txnBody.indexOf("Transactions") + 800);
  console.log("Transactions sample:", txnSample);
}

await browser.close();
