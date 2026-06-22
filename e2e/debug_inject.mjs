import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000);

// Check current ds
const dsBefore = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Before: txns=", (dsBefore.transactions || []).length, "accounts=", (dsBefore.accounts || []).length);

const RUN = "L53_" + Date.now();

// Inject directly
const result = await p.evaluate((runTag) => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const accounts = ds.accounts || [];
  const defaultAcctId = accounts.length > 0 ? accounts[0].id : "acct-checking";
  const now = new Date().toISOString().slice(0, 10);

  const newTxn = {
    id: runTag + "-oak1",
    accountId: defaultAcctId,
    date: now,
    desc: runTag + "-Oak-1",
    amount: { Amount: -50000, Currency: "USD" },
    custom: { l53_property: "Oak Street", l53_tax_ded: true }
  };
  ds.transactions = ds.transactions || [];
  ds.transactions.push(newTxn);
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));

  // Verify it was saved
  const ds2 = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const found = (ds2.transactions || []).find(t => t.id === newTxn.id);
  return { totalBefore: (ds.transactions || []).length - 1, totalAfter: (ds2.transactions || []).length, found: !!found, foundCustom: found?.custom };
}, RUN);
console.log("Inject result:", JSON.stringify(result));

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

// Reload and verify
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(3000);

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const dsAfter = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = dsAfter.transactions || [];
const found = allTxns.find(t => t.id && t.id.includes("oak1") && t.id.includes("L53_"));
console.log("After reload: total=", allTxns.length, "found injected=", !!found, "custom=", found?.custom);

await b.close();
