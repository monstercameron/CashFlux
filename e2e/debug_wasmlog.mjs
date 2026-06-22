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

// Capture ALL console messages from the browser
const consoleMsgs = [];
p.on("console", m => {
  const txt = m.text();
  consoleMsgs.push(txt);
  // Print messages that look like errors or import/hydrate related
  if (txt.toLowerCase().includes('import') || txt.toLowerCase().includes('hydrat') ||
      txt.toLowerCase().includes('error') || txt.toLowerCase().includes('err') ||
      txt.toLowerCase().includes('seed') || txt.toLowerCase().includes('dataset') ||
      txt.toLowerCase().includes('transaction')) {
    console.log("BROWSER:", txt);
  }
});

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

const RUN = "WASMLOG_" + Date.now();

// Inject
const count = await p.evaluate((run) => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const acctId = (ds.accounts || [])[0]?.id || "acct-checking";
  ds.transactions.push({
    id: run + "-t1",
    accountId: acctId,
    date: "2024-07-14T00:00:00Z", // Exact same format as seed data
    desc: run + "-Desc",
    amount: { Amount: -50000, Currency: "USD" },
    custom: { test_prop: "Oak" }
  });
  localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
  return ds.transactions.length;
}, RUN);
console.log("Injected, count=", count);

// Reload and capture all browser logs
consoleMsgs.length = 0;
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

const countAfter = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("After reload:", countAfter);

// Print all collected browser messages
console.log("\n--- All browser console messages on reload ---");
consoleMsgs.forEach(m => console.log("B>", m));

await b.close();
