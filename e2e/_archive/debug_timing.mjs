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

// Watch localStorage writes
await p.addInitScript(() => {
  const origSet = localStorage.setItem.bind(localStorage);
  localStorage.setItem = (k, v) => {
    if (k === 'cashflux:dataset') {
      const parsed = JSON.parse(v || '{}');
      console.log('[LS_WRITE]', new Date().toISOString(), 'txns=', (parsed.transactions || []).length);
    }
    return origSet(k, v);
  };
});

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

const RUN = "DBTIMING_" + Date.now();

// Inject
await p.evaluate((run) => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id || "acct-checking";
  ds.transactions = ds.transactions || [];
  ds.transactions.push({
    id: run + "-t1",
    accountId: acctId,
    date: new Date().toISOString(),
    desc: run + "-Desc",
    amount: { Amount: -50000, Currency: "USD" },
    custom: { test_prop: "Oak Street" }
  });
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  console.log('[INJECTED] count=', ds.transactions.length);
}, RUN);

await p.waitForTimeout(200);

// Now reload
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(6000); // 6 seconds — more than autosave 4s

const dsAfter = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const found = (dsAfter.transactions || []).find(t => t.id && t.id.includes("DBTIMING_"));
console.log("After reload+6s: txns=", (dsAfter.transactions || []).length, "found=", !!found);

await b.close();
