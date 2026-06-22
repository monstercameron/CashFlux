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
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000);

// flush first to get current state saved
await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const RUN = "INJECT2_" + Date.now();

// Read current ds (should be properly saved now)
const dsBeforeInject = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Before inject: txns=", (dsBeforeInject.transactions || []).length);

// Inject directly into localStorage
await p.evaluate((runTag) => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const accounts = ds.accounts || [];
  const defaultAcctId = accounts.length > 0 ? accounts[0].id : "acct-checking";
  const now = new Date().toISOString().slice(0, 10);

  ds.transactions = ds.transactions || [];
  ds.transactions.push({
    id: runTag + "-t1",
    accountId: defaultAcctId,
    date: now,
    desc: runTag + "-Oak-1",
    amount: { Amount: -50000, Currency: "USD" },
    custom: { l53_property: "Oak Street", l53_tax_ded: true }
  });
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  console.log("Injected into localStorage, new count:", ds.transactions.length);
}, RUN);

// Read back immediately (no reload)
const dsImmediate = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Immediately after inject (no reload): txns=", (dsImmediate.transactions || []).length);
const immediateFound = (dsImmediate.transactions || []).find(t => t.id && t.id.includes("INJECT2_"));
console.log("Found injected:", !!immediateFound, "custom:", immediateFound?.custom);

// Reload page WITHOUT dispatching visibilitychange first (to avoid wasm overwriting)
await p.reload({ waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000); // wait for wasm to boot and load data

// Read localStorage RIGHT AWAY (before any visibilitychange)
const dsAfterReload = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("After reload (no visibilitychange): txns=", (dsAfterReload.transactions || []).length);
const afterReloadFound = (dsAfterReload.transactions || []).find(t => t.id && t.id.includes("INJECT2_"));
console.log("Found after reload:", !!afterReloadFound, "custom:", afterReloadFound?.custom);

// Now dispatch visibilitychange and check again
await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);
const dsAfterVC = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("After visibilitychange: txns=", (dsAfterVC.transactions || []).length);
const afterVCFound = (dsAfterVC.transactions || []).find(t => t.id && t.id.includes("INJECT2_"));
console.log("Found after VC:", !!afterVCFound);

await b.close();
