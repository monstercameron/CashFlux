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

// Add the account fresh and inspect
await goto("/accounts");

// Fill form
const nameInput = await page.$('input[type="text"]');
await nameInput.fill("L44 Omar Checking");
const typeSelect = await page.$('select');
await typeSelect.selectOption({ label: "Checking" });
const amtInput = await page.$('input[type="number"]');
await amtInput.fill("1000");
const addBtn = await page.$('button:has-text("Add account")');
await addBtn.click();
await page.waitForTimeout(1200);

// Now read the body text and specifically find L44 Omar Checking's line
const body = await page.evaluate(() => document.body.innerText);
const idx = body.indexOf("L44 Omar Checking");
console.log("L44 Omar Checking context:", body.slice(Math.max(0, idx), idx + 300));

// Find account rows - each row has the account name + balance
const rows = await page.evaluate(() => {
  const descs = Array.from(document.querySelectorAll('.row-desc, .row-main'));
  return descs.map(el => ({
    text: el.innerText?.trim(),
    parentHTML: el.parentElement?.innerText?.trim().slice(0, 200)
  }));
});
console.log("Account rows:", JSON.stringify(rows.filter(r => r.text.includes("L44") || r.text.includes("Omar"))));

await browser.close();
