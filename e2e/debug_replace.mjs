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
p.on("console", m => console.log("B>", m.text()));

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

const count1 = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Before inject:", count1);

// REPLACE transactions with just 3
const injectResult = await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id;
  ds.transactions = [
    { id: "L53-oak1",   accountId: acctId, date: "2024-07-14T00:00:00Z", desc: "L53-Oak-1",   amount: { Amount: -50000, Currency: "USD" }, custom: { l53_property: "Oak Street" } },
    { id: "L53-maple1", accountId: acctId, date: "2024-07-14T00:00:00Z", desc: "L53-Maple-1", amount: { Amount: -20000, Currency: "USD" }, custom: { l53_property: "Maple Ave"  } },
    { id: "L53-oak2",   accountId: acctId, date: "2024-07-14T00:00:00Z", desc: "L53-Oak-2",   amount: { Amount: -30000, Currency: "USD" }, custom: { l53_property: "Oak Street" } },
  ];
  const newRaw = JSON.stringify(ds);
  localStorage.setItem("cashflux:dataset", newRaw);

  // Verify
  const verify = JSON.parse(newRaw);
  return { txns: verify.transactions.length, ids: verify.transactions.map(t => t.id), rawLen: newRaw.length };
});
console.log("After inject (no reload):", JSON.stringify(injectResult));

// Reload immediately
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

const afterReload = await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const found = (ds.transactions || []).find(t => t.id === "L53-oak1");
  return { txns: (ds.transactions || []).length, ids: (ds.transactions || []).map(t => t.id), found: !!found };
});
console.log("After reload:", JSON.stringify(afterReload));

await b.close();
