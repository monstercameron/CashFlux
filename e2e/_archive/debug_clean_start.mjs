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

// Clear localStorage before loading
await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.evaluate(() => localStorage.clear());
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000);

const countBefore = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Count before (fresh):", countBefore);

const tag = "CLEAN_" + Date.now();
const descInput = await p.$("#txn-add");
if (descInput) {
  await descInput.fill(tag);
  const val = await descInput.evaluate(el => el.value);
  console.log("DOM value after fill:", val);
}

const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) await amtInput.fill("25");

const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("CLEAN_"));
const last = allTxns[allTxns.length - 1];
console.log("Total txns:", allTxns.length, "Found:", found.length);
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount?.Amount }));

await b.close();
