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
await p.waitForTimeout(4000);

// Inject, then check what's in localStorage before reload
const injResult = await p.evaluate(() => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const acctId = (ds.accounts || [])[0]?.id;
  ds.transactions.push({
    id: "VERIFY_TEST_ABC",
    accountId: acctId,
    date: "2024-07-14T00:00:00Z",
    desc: "VERIFY-Desc",
    amount: { Amount: -50000, Currency: "USD" }
  });
  const newRaw = JSON.stringify(ds);
  localStorage.setItem("cashflux:dataset", newRaw);

  // Count how many JSON items the array has by parsing back
  const verify = JSON.parse(newRaw);
  const found = (verify.transactions || []).find(t => t.id === "VERIFY_TEST_ABC");
  return {
    txnCount: verify.transactions.length,
    found: !!found,
    foundId: found?.id,
    last5: verify.transactions.slice(-5).map(t => t.id)
  };
});
console.log("Before reload:", JSON.stringify(injResult));

// Reload
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

// After reload
const afterResult = await p.evaluate(() => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const found = (ds.transactions || []).find(t => t.id === "VERIFY_TEST_ABC");
  return {
    txnCount: ds.transactions.length,
    found: !!found,
    last5: ds.transactions.slice(-5).map(t => t.id)
  };
});
console.log("After reload:", JSON.stringify(afterResult));

await b.close();
