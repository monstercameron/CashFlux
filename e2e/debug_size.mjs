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
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

const result = await p.evaluate(() => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const acctId = (ds.accounts || [])[0]?.id;

  // Inject one transaction
  ds.transactions.push({
    id: "INJECT_SIZE_TEST",
    accountId: acctId,
    date: "2024-07-14T00:00:00Z",
    desc: "SIZE_TEST-Desc",
    amount: { Amount: -50000, Currency: "USD" }
  });

  const newRaw = JSON.stringify(ds);
  const before = raw.length;
  const after = newRaw.length;
  localStorage.setItem("cashflux:dataset", newRaw);

  // Verify it round-trips
  const re = localStorage.getItem("cashflux:dataset");
  const ds2 = JSON.parse(re);
  const found = (ds2.transactions || []).find(t => t.id === "INJECT_SIZE_TEST");

  return { beforeLen: before, afterLen: after, storedLen: re?.length, txns: ds2.transactions.length, found: !!found };
});

console.log("Size test:", JSON.stringify(result));

// Now reload and check
await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

const afterReload = await p.evaluate(() => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const found = (ds.transactions || []).find(t => t.id === "INJECT_SIZE_TEST");
  return { txns: (ds.transactions || []).length, rawLen: raw.length, found: !!found };
});
console.log("After reload:", JSON.stringify(afterReload));

await b.close();
