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

const consoleMsgs = [];
page.on("console", msg => consoleMsgs.push({ type: msg.type(), text: msg.text() }));

// Add account
await goto("/accounts");
await (await page.$('input[type="text"]')).fill("L44 Omar Checking");
await (await page.$('select')).selectOption({ label: "Checking" });
await (await page.$('input[type="number"]')).fill("1000");
await (await page.$('button:has-text("Add account")')).click();
await page.waitForTimeout(1500);

// Verify account is in app state
const acctBody = await page.evaluate(() => document.body.innerText);
console.log("L44 in accounts:", acctBody.includes("L44 Omar Checking"));

// Now navigate to documents WITHOUT a full page reload (use router navigation)
// The gwc/router handles this as a client-side navigation
await page.goto(BASE + "/documents", { waitUntil: "domcontentloaded" });
await waitNav(page);
await page.waitForTimeout(2000);

// Check console for what happened
const preImportLogs = consoleMsgs.filter(m => m.type === "log");
console.log("Pre-import console:", JSON.stringify(preImportLogs.slice(-5)));

const textareas = await page.$$('textarea');
const CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking`;

await textareas[1].fill(CSV);
await page.waitForTimeout(500);
const importBtn = await page.$('button:has-text("Import")');
await importBtn.click();
await page.waitForTimeout(3000);

// Check console for import message
const postImportLogs = consoleMsgs.filter(m => m.text.includes("import") || m.text.includes("transaction") || m.text.includes("account") || m.text.includes("csv"));
console.log("Post-import logs:", JSON.stringify(postImportLogs));

// Screenshot the documents page showing the result
await page.screenshot({ path: "e2e/debug-after-import.png" });

// Full body text to find the message
const fullBody = await page.evaluate(() => document.body.innerText);
// Find the section after "Import transactions"  
const importSectionIdx = fullBody.indexOf("Import transactions");
if (importSectionIdx >= 0) {
  console.log("Import transactions section:", fullBody.slice(importSectionIdx, importSectionIdx + 400));
}

await browser.close();
