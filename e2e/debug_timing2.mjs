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
p.on("console", m => { if (m.text().includes('[LS_WRITE]') || m.text().includes('[INJ')) console.log("BROWSER:", m.text()); });

// Watch localStorage writes
await p.addInitScript(() => {
  const origSet = localStorage.setItem.bind(localStorage);
  localStorage.setItem = (k, v) => {
    if (k === 'cashflux:dataset') {
      const parsed = JSON.parse(v || '{}');
      console.log('[LS_WRITE] txns=' + (parsed.transactions || []).length);
    }
    return origSet(k, v);
  };
});

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000); // initial autosave

const countBefore = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Count after 5s:", countBefore);

const RUN = "DBTIMING_" + Date.now();
await p.evaluate((run) => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id || "acct-checking";
  ds.transactions.push({
    id: run + "-t1",
    accountId: acctId,
    date: new Date().toISOString(),
    desc: run + "-Desc",
    amount: { Amount: -50000, Currency: "USD" }
  });
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  console.log('[INJ] count after inject=' + ds.transactions.length);
}, RUN);

// reload and capture writes
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(6000);

const countAfter = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Count after reload+6s:", countAfter);

await b.close();
