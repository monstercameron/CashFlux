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

// Spy on localStorage BEFORE wasm boots to trace every read/write
await p.addInitScript(() => {
  const origGet = localStorage.getItem.bind(localStorage);
  const origSet = localStorage.setItem.bind(localStorage);
  localStorage.getItem = (k) => {
    const v = origGet(k);
    if (k === 'cashflux:dataset') {
      const txns = v ? (JSON.parse(v).transactions || []).length : 0;
      console.log(`[LS_GET] ${k} → ${txns} txns, ${v?.length || 0} chars`);
    }
    return v;
  };
  localStorage.setItem = (k, v) => {
    if (k === 'cashflux:dataset') {
      const txns = v ? (JSON.parse(v).transactions || []).length : 0;
      console.log(`[LS_SET] ${k} ← ${txns} txns, ${v?.length || 0} chars`);
      console.trace('[LS_SET_TRACE]');
    }
    return origSet(k, v);
  };
});

// Inject BEFORE going to the page - use a different approach:
// Set the localStorage directly in the page context before any wasm runs
await p.goto("about:blank");
await p.evaluate(() => {
  // We can't set cashflux:dataset here because it's a different origin
});

// Go to the real URL, let it load once to get seed data
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

console.log("\n--- Now injecting ---");
await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id;
  ds.transactions = [
    { id: "L53-t1", accountId: acctId, date: "2024-07-14T00:00:00Z", desc: "L53-Desc", amount: { Amount: -50000, Currency: "USD" } }
  ];
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
});

console.log("\n--- Reloading ---");
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(6000);

const final = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Final txn count:", final);

await b.close();
