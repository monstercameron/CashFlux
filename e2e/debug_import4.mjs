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
page.on("console", msg => consoleMsgs.push(msg.text()));

// Add account
await goto("/accounts");
await (await page.$('input[type="text"]')).fill("L44 Omar Checking");
await (await page.$('select')).selectOption({ label: "Checking" });
await (await page.$('input[type="number"]')).fill("1000");
await (await page.$('button:has-text("Add account")')).click();
await page.waitForTimeout(1500);

await page.goto(BASE + "/documents", { waitUntil: "domcontentloaded" });
await waitNav(page);
await page.waitForTimeout(2000);

const textareas = await page.$$('textarea');
const CSV = `date,payee,amount,account
2026-06-15,L44 SUPERMARKET GROCERIES,-95.00,L44 Omar Checking`;
await textareas[1].fill(CSV);
await page.waitForTimeout(500);

// Scroll to the Import button and click it
const importBtn = await page.$('button:has-text("Import")');
await importBtn.scrollIntoViewIfNeeded();
await page.waitForTimeout(300);

// Take screenshot BEFORE clicking to verify button is visible  
await page.screenshot({ path: "e2e/debug-before-import-click.png" });

await importBtn.click();
await page.waitForTimeout(3000);

// Screenshot AFTER clicking
await page.screenshot({ path: "e2e/debug-after-import-click.png" });

// Filter console for relevant logs
const relevantLogs = consoleMsgs.filter(m => m.includes("import") || m.includes("transaction") || m.includes("csv") || m.includes("account") || m.includes("saved"));
console.log("Relevant console:", JSON.stringify(relevantLogs));

// Read the full current page body
const fullBody = await page.evaluate(() => document.body.innerText);
const csvSection = fullBody.slice(fullBody.indexOf("Import transactions"), fullBody.indexOf("Import transactions") + 500);
console.log("CSV section:", csvSection);

await browser.close();
