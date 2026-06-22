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

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

const RUN = "DBGDATE_" + Date.now();

// Read before
const dsBefore = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Before: txns=", (dsBefore.transactions || []).length, "acct0=", (dsBefore.accounts || [])[0]?.id);

// Inject with RFC3339
const total = await p.evaluate((run) => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const accounts = ds.accounts || [];
  const acctId = accounts.length > 0 ? accounts[0].id : "acct-checking";
  const now = new Date().toISOString(); // RFC3339 full timestamp
  ds.transactions = ds.transactions || [];
  ds.transactions.push({
    id: run + "-t1",
    accountId: acctId,
    date: now,
    desc: run + "-Desc",
    amount: { Amount: -50000, Currency: "USD" },
    custom: { test_prop: "Oak Street" }
  });
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  return ds.transactions.length;
}, RUN);
console.log("After inject (no reload):", total);

// Reload without visibilitychange first
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

// Check localStorage immediately after reload (before wasm saves)
const dsAfter = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = dsAfter.transactions || [];
const found = allTxns.find(t => t.id && t.id.includes("DBGDATE_"));
console.log("After reload: total=", allTxns.length, "found=", !!found, "custom=", found?.custom);
console.log("First 2 dates:", allTxns.slice(0,2).map(t => t.date));

await b.close();
