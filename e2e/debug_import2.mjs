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

// Add account
await goto("/accounts");
const nameInput = await page.$('input[type="text"]');
await nameInput.fill("L44 Omar Checking");
await (await page.$('select')).selectOption({ label: "Checking" });
await (await page.$('input[type="number"]')).fill("1000");
await (await page.$('button:has-text("Add account")')).click();
await page.waitForTimeout(1500);

// Capture console messages to see the import result
const consoleMsgs = [];
page.on("console", msg => consoleMsgs.push({ type: msg.type(), text: msg.text() }));

// Also capture network requests to see what data is being sent  
await goto("/documents");

// Take a screenshot to see the state before import
await page.screenshot({ path: "e2e/debug-docs-before.png" });

const textareas = await page.$$('textarea');
console.log("Found textareas:", textareas.length);

// Use the CSV import textarea (index 1)
const CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking`;
await textareas[1].fill(CSV);
await page.waitForTimeout(300);

// Click Import
const importBtn = await page.$('button:has-text("Import")');
console.log("Import button found:", !!importBtn);
await importBtn.click();
await page.waitForTimeout(2500);

// Take screenshot after
await page.screenshot({ path: "e2e/debug-docs-after.png" });

// Get the msg state - look at all text elements near the import button
const msgText = await page.evaluate(() => {
  // Find all p, span, div elements that changed / contain "imported" or numbers
  const allText = document.body.innerText;
  // Look for any mention of "imported", "transaction", "skipped"  
  const importSection = allText.match(/Import[\s\S]{0,800}/i);
  return importSection?.[0] || allText.slice(0, 500);
});
console.log("After-import page fragment:", msgText.slice(0, 600));

// Console messages from the app
console.log("Console messages:", JSON.stringify(consoleMsgs.filter(m => 
  m.type === "error" || m.text.toLowerCase().includes("import") || m.text.toLowerCase().includes("transaction") ||
  m.text.toLowerCase().includes("account") || m.text.toLowerCase().includes("csv")
)));

await browser.close();
