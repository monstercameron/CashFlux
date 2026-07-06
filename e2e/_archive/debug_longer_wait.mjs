import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
p.on("pageerror", e => console.log("JSERR:", e.message));

await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000);

const before = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Count before:", before);

const tag = "LONGWAIT_" + Date.now();

// Use pressSequentially for description
await p.locator("#txn-add").pressSequentially(tag, { delay: 15 });
await p.waitForTimeout(500);

// Use pressSequentially for amount (treats each digit as a key press)
await p.locator('.form-grid input[type="number"]').pressSequentially("99", { delay: 15 });
await p.waitForTimeout(500);

const amtVal = await p.locator('.form-grid input[type="number"]').inputValue();
console.log("Amount DOM value:", amtVal);

// Submit
await p.locator('.form-grid button:has-text("Add")').click();
await p.waitForTimeout(3000); // wait 3s

// Dispatch visibilitychange 3 times
for (let i = 0; i < 3; i++) {
  await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await p.waitForTimeout(1000);
}

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("LONGWAIT_"));
const last = allTxns[allTxns.length - 1];
console.log("Total:", allTxns.length, "Found LONGWAIT_:", found.length);
console.log("Last 3:", JSON.stringify(allTxns.slice(-3).map(t => ({ desc: t.description?.substring(0,20), amt: t.amount?.Amount }))));

await b.close();
