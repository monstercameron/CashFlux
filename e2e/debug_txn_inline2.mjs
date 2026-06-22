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

// Navigate directly to transactions and wait for wasm to load
await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000); // wait for wasm + sample data to load

// Count txns before
const dsBefore = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Txns before (after 4s wait):", (dsBefore.transactions || []).length);

const tag = "INLINE2_" + Date.now();
const descInput = await p.$("#txn-add");
if (descInput) await descInput.fill(tag);

const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) await amtInput.fill("88");

// Submit
const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

// Flush + check
await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const dsAfter = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = dsAfter.transactions || [];
console.log("Total txns after submit:", allTxns.length);

// Show the last 3 transactions (most recent should be ours)
const last3 = allTxns.slice(-3);
console.log("Last 3 txns:", JSON.stringify(last3.map(t => ({ desc: t.description, amount: t.amount, custom: t.custom })), null, 2));

const found = allTxns.filter(t => t.description && t.description.includes("INLINE2"));
console.log("Found INLINE2 txns:", found.length);

await b.close();
