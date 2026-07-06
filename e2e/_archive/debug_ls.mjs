import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
p.on("console", m => console.log("B>", m.text()));

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

// List ALL localStorage keys
const allKeys = await p.evaluate(() => {
  const keys = [];
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    const v = localStorage.getItem(k);
    keys.push({ key: k, len: v?.length || 0 });
  }
  return keys.sort((a,b) => b.len - a.len);
});
console.log("All localStorage keys:");
allKeys.forEach(k => console.log(`  ${k.key}: ${k.len} chars`));

// Now inject 3 transactions
await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id;
  ds.transactions = [
    { id: "L53-oak1", accountId: acctId, date: "2024-07-14T00:00:00Z", desc: "L53-Oak-1", amount: { Amount: -50000, Currency: "USD" } }
  ];
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  console.log("Injected 1 tx, new count:", ds.transactions.length);
});

// Reload
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

// List ALL localStorage keys again
const allKeysAfter = await p.evaluate(() => {
  const keys = [];
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    const v = localStorage.getItem(k);
    keys.push({ key: k, len: v?.length || 0, txns: k === 'cashflux:dataset' ? (JSON.parse(v || '{}').transactions || []).length : null });
  }
  return keys.sort((a,b) => b.len - a.len);
});
console.log("\nAll localStorage keys AFTER reload:");
allKeysAfter.forEach(k => console.log(`  ${k.key}: ${k.len} chars${k.txns !== null ? ` (${k.txns} txns)` : ''}`));

await b.close();
